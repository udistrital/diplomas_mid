package services

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/utils_oas/v2/request"
)

type FirmaDiplomaService struct {
	S3 S3Service
}

func (s FirmaDiplomaService) Firmar(ctx context.Context, documentoID int64, input *FirmaDiplomaInput) (*FirmaDiplomaResult, error) {
	if documentoID <= 0 {
		return nil, fmt.Errorf("%w: documento id is required", ErrInvalidInput)
	}
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.DocumentoIdentidadFirmante <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}

	firmanteActivo, err := FirmanteService{}.ConsultarRolActivo(ctx, input.DocumentoIdentidadFirmante, time.Now())
	if err != nil {
		return nil, err
	}
	if err := validarRubricaFirmante(ctx, firmanteActivo); err != nil {
		return nil, err
	}
	if err := validarFlujoFirmaDocumento(ctx, documentoID, firmanteActivo); err != nil {
		return nil, err
	}
	if err := asegurarRegistroDiploma(ctx, documentoID); err != nil {
		return nil, err
	}
	preview := firmanteActivo.Orden < 4
	if preview {
		firmaDocumento, err := registrarFirmaDocumentoDiploma(ctx, documentoID, firmanteActivo)
		if err != nil {
			return nil, err
		}
		previewDocumento, err := DiplomaPreviewService{}.Generar(ctx, documentoID)
		if err != nil {
			return nil, err
		}
		return &FirmaDiplomaResult{
			Preview:          true,
			FirmaID:          "QR-DEMO",
			DocumentoID:      documentoID,
			Firmante:         *firmanteActivo,
			DocumentoDigital: previewDocumento.DocumentoDigital,
			FirmaDocumento:   firmaDocumento,
			File:             previewDocumento.File,
		}, nil
	}

	previewDocumento, err := generarDiplomaPreview(ctx, documentoID, firmanteActivo)
	if err != nil {
		return nil, err
	}
	metadatos := cloneMetadatos(input.Metadatos)
	metadatos["rol_firmante"] = firmanteActivo.Rol
	metadatos["orden_firmante"] = firmanteActivo.Orden
	metadatos["cargo_id_firmante"] = firmanteActivo.CargoID
	metadatos["documento_identidad_firmante"] = firmanteActivo.DocumentoIdentidad

	firmantes := []PersonaFirma{{
		Nombre:         firmanteActivo.Nombre,
		Cargo:          firmanteActivo.Cargo,
		TipoID:         "CC",
		Identificacion: strconv.FormatInt(firmanteActivo.DocumentoIdentidad, 10),
	}}
	representantes := input.Representantes
	if representantes == nil {
		representantes = []PersonaFirma{}
	}

	nombre := input.Nombre
	if nombre == "" {
		nombre = nombreArchivoDiploma(previewDocumento)
	}
	descripcion := input.Descripcion
	if descripcion == "" {
		descripcion = "Diploma digital final"
	}

	firmaPayload := []firmaElectronicaRequest{{
		RepositorioDocumental: "diplomas",
		DocumentoID:           documentoID,
		Nombre:                nombre,
		Descripcion:           descripcion,
		Metadatos:             metadatos,
		Firmantes:             firmantes,
		Representantes:        representantes,
		File:                  previewDocumento.File,
	}}

	var firmaResponse firmaElectronicaResponse
	status, err := request.PostWithContext(ctx, firmaElectronicaMidURL()+"/v2/firma_electronica", firmaPayload, &firmaResponse)
	if err != nil {
		return nil, fmt.Errorf("%w: firma_electronica_mid status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: firma_electronica_mid returned status %d", ErrExternalService, status)
	}
	if firmaResponse.File == "" {
		return nil, fmt.Errorf("%w: firma_electronica_mid response missing file", ErrExternalService)
	}
	if firmaResponse.UUIDDocumento == "" {
		return nil, fmt.Errorf("%w: firma_electronica_mid response missing uuid_documento", ErrExternalService)
	}
	if err := validarDocumentoFirmado(documentoID, firmaResponse); err != nil {
		return nil, err
	}

	s3Service := s.S3
	s3Ref, err := s3Service.PutDiploma(ctx, firmaResponse.UUIDDocumento, nombre, firmaResponse.File)
	if err != nil {
		return nil, err
	}

	var documentoResponse documentoDigitalResponse
	updatePayload := actualizarUUIDDocumentoRequest{UUIDDocumento: firmaResponse.UUIDDocumento}
	status, err = request.PutWithContext(
		ctx,
		fmt.Sprintf("%s/documento_digital/%d/uuid", diplomasCrudURL(), documentoID),
		updatePayload,
		&documentoResponse,
	)
	if err != nil {
		return nil, fmt.Errorf("diplomas_crud actualizar uuid status %d: %w", status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud actualizar uuid returned status %d", ErrExternalService, status)
	}

	firmaDocumento, err := registrarFirmaDocumentoDiploma(ctx, documentoID, firmanteActivo)
	if err != nil {
		return nil, err
	}

	return &FirmaDiplomaResult{
		Preview:               false,
		FirmaID:               firmaResponse.FirmaID,
		RepositorioDocumental: firmaResponse.RepositorioDocumental,
		DocumentoID:           documentoID,
		UUIDDocumento:         firmaResponse.UUIDDocumento,
		HashSHA256:            firmaResponse.HashSHA256,
		CodigoAutenticidad:    firmaResponse.CodigoAutenticidad,
		QRURLSegura:           firmaResponse.QRURLSegura,
		Firmante:              *firmanteActivo,
		S3:                    &s3Ref,
		DynamoDB:              firmaResponse.DynamoDB,
		DocumentoDigital:      documentoResponse,
		FirmaDocumento:        firmaDocumento,
	}, nil
}

func nombreArchivoDiploma(preview *DiplomaPreviewResult) string {
	if preview == nil {
		return "Diploma.pdf"
	}
	fecha := time.Now()
	if preview.DiplomaDigital != nil && !preview.DiplomaDigital.FechaGrado.IsZero() {
		fecha = preview.DiplomaDigital.FechaGrado
	}
	codigo := preview.DocumentoDigital.CodigoEstudiante
	if codigo <= 0 {
		codigo = preview.DocumentoID
	}
	return fmt.Sprintf("%s_Diploma_%d.pdf", fechaEnBogota(fecha).Format("20060102"), codigo)
}

func (s FirmaDiplomaService) Anular(ctx context.Context, documentoID int64, input *AnularFirmaDiplomaInput) (*AnularFirmaDiplomaResult, error) {
	if documentoID <= 0 {
		return nil, fmt.Errorf("%w: documento id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadFirmante <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	firmanteActivo, err := FirmanteService{}.ConsultarRolActivo(ctx, input.DocumentoIdentidadFirmante, time.Now())
	if err != nil {
		return nil, err
	}
	if firmanteActivo.Orden >= 4 {
		return nil, fmt.Errorf("%w: la firma final del rector no se puede anular desde este flujo", ErrInvalidState)
	}
	var documento documentoDigitalResponse
	status, err := request.GetWithContext(ctx, fmt.Sprintf("%s/documento_digital/%d", diplomasCrudURL(), documentoID), &documento)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital returned status %d", ErrExternalService, status)
	}
	if documento.UUIDDocumento != nil && *documento.UUIDDocumento != "" {
		return nil, fmt.Errorf("%w: el diploma ya fue finalizado", ErrInvalidState)
	}
	firmas, err := listarFirmasDocumento(ctx, documentoID)
	if err != nil {
		return nil, err
	}
	anuladas := 0
	for _, firma := range firmas {
		orden, ok := ordenRolFirmante(firma.RolFirmanteID)
		if !ok {
			continue
		}
		if orden > firmanteActivo.Orden {
			return nil, fmt.Errorf("%w: no se puede anular porque existe una firma posterior", ErrInvalidState)
		}
	}
	for _, firma := range firmas {
		orden, ok := ordenRolFirmante(firma.RolFirmanteID)
		if !ok || orden != firmanteActivo.Orden {
			continue
		}
		status, err := request.DeleteWithContext(ctx, fmt.Sprintf("%s/firma_documento/%d", diplomasCrudURL(), firma.ID), nil)
		if err != nil {
			return nil, fmt.Errorf("%w: diplomas_crud firma_documento delete status %d: %v", ErrExternalService, status, err)
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("%w: diplomas_crud firma_documento delete returned status %d", ErrExternalService, status)
		}
		anuladas++
	}
	return &AnularFirmaDiplomaResult{DocumentoID: documentoID, Orden: firmanteActivo.Orden, Anuladas: anuladas}, nil
}

func cloneMetadatos(input map[string]interface{}) map[string]interface{} {
	metadatos := make(map[string]interface{}, len(input)+4)
	for key, value := range input {
		metadatos[key] = value
	}
	return metadatos
}

func validarRubricaFirmante(ctx context.Context, firmante *FirmanteActivo) error {
	if firmante == nil {
		return fmt.Errorf("%w: firmante is required", ErrInvalidInput)
	}
	firmaFirmante, err := buscarFirmaFirmanteActiva(ctx, firmante.DocumentoIdentidad)
	if err != nil {
		return err
	}
	if firmaFirmante == nil || firmaFirmante.EnlaceFirma == "" {
		return fmt.Errorf("%w: el firmante no tiene rubrica activa registrada", ErrInvalidState)
	}
	return nil
}

func asegurarRegistroDiploma(ctx context.Context, documentoID int64) error {
	documento, err := consultarDocumentoDigital(ctx, documentoID)
	if err != nil {
		return err
	}

	fechaGrado, err := fechaCeremoniaDocumento(ctx, documentoID)
	if err != nil {
		return err
	}
	return asegurarRegistroDiplomaEnFecha(ctx, documento, fechaGrado)
}

// Idempotente: reutiliza el diploma y el registro ya reservado.
func asegurarRegistroDiplomaEnFecha(ctx context.Context, documento *documentoDigitalResponse, fechaGrado time.Time) error {
	documentoID := documento.ID
	diploma, err := buscarDiplomaDigitalExistente(ctx, documentoID)
	if err != nil {
		return err
	}
	if diploma == nil {
		payload, err := construirDiplomaFinalRequest(ctx, documento, fechaGrado)
		if err != nil {
			return err
		}
		diploma, err = crearDiplomaDigital(ctx, documentoID, *payload)
		if err != nil {
			return err
		}
	}
	if diploma.RegistroGrado != nil {
		return nil
	}

	diploma, err = asignarRegistroDiplomaDigital(ctx, diploma.ID)
	if err != nil {
		return err
	}
	if diploma.RegistroGrado == nil {
		return fmt.Errorf("%w: no se pudo asignar libro, folio y registro al diploma", ErrExternalService)
	}
	return nil
}

func construirDiplomaFinalRequest(ctx context.Context, documento *documentoDigitalResponse, fechaGrado time.Time) (*crearDiplomaDigitalRequest, error) {
	if documento == nil {
		return nil, fmt.Errorf("%w: documento_digital is required", ErrInvalidInput)
	}
	if documento.FacultadID == nil || *documento.FacultadID <= 0 {
		return nil, fmt.Errorf("%w: documento_digital sin facultad para generar registro de grado", ErrInvalidState)
	}
	vigenciaRegistro := fechaGrado.Year()
	if vigenciaRegistro <= 0 {
		return nil, fmt.Errorf("%w: fecha de ceremonia invalida para generar registro de grado", ErrInvalidState)
	}
	if strings.TrimSpace(documento.NombreEstudiante) == "" ||
		strings.TrimSpace(documento.TipoDocumentoEstudiante) == "" ||
		strings.TrimSpace(documento.NumeroDocumentoEstudiante) == "" ||
		strings.TrimSpace(documento.TituloOtorgado) == "" {
		return nil, fmt.Errorf("%w: documento_digital no tiene datos completos para generar diploma", ErrInvalidState)
	}
	if err := validarParametrizacionRegistro(ctx, *documento.FacultadID, vigenciaRegistro); err != nil {
		return nil, err
	}
	estadoGeneradoID, err := resolverParametroID(ctx, 0, "GENERADO")
	if err != nil {
		return nil, err
	}
	return &crearDiplomaDigitalRequest{
		FacultadID:                *documento.FacultadID,
		Vigencia:                  vigenciaRegistro,
		FechaGrado:                fechaGrado,
		NombreEstudiante:          strings.TrimSpace(documento.NombreEstudiante),
		TipoDocumentoEstudiante:   strings.TrimSpace(documento.TipoDocumentoEstudiante),
		NumeroDocumentoEstudiante: strings.TrimSpace(documento.NumeroDocumentoEstudiante),
		MunicipioExpedicion:       strings.TrimSpace(documento.MunicipioExpedicion),
		TituloOtorgado:            strings.TrimSpace(documento.TituloOtorgado),
		EstadoDocumentoCreadoID:   estadoGeneradoID,
	}, nil
}

type parametrizacionRegistroResponse struct {
	ID         int64 `json:"id"`
	FacultadID int64 `json:"facultad_id"`
	Vigencia   int   `json:"vigencia"`
	Activo     bool  `json:"activo"`
}

func validarParametrizacionRegistro(ctx context.Context, facultadID int64, vigencia int) error {
	rawQuery := fmt.Sprintf("facultad_id:%d,vigencia:%d,activo:true", facultadID, vigencia)
	endpoint := fmt.Sprintf("%s/parametrizacion_registro_facultad/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var parametrizaciones []parametrizacionRegistroResponse
	status, err := request.GetWithContext(ctx, endpoint, &parametrizaciones)
	if err != nil {
		return fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad returned status %d", ErrExternalService, status)
	}
	if len(parametrizaciones) == 0 {
		return fmt.Errorf("%w: no existe parametrizacion activa de registro para facultad %d y vigencia %d", ErrInvalidState, facultadID, vigencia)
	}
	return nil
}

func fechaCeremoniaDocumento(ctx context.Context, documentoID int64) (time.Time, error) {
	rawQuery := fmt.Sprintf("documento_digital_id:%d,activo:true", documentoID)
	endpoint := fmt.Sprintf("%s/ceremonia_estudiante/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var asignaciones []ceremoniaEstudianteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &asignaciones)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return time.Time{}, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante returned status %d", ErrExternalService, status)
	}
	for _, asignacion := range asignaciones {
		if asignacion.CeremoniaFacultadPrograma == nil ||
			asignacion.CeremoniaFacultadPrograma.CeremoniaGrado == nil ||
			strings.TrimSpace(asignacion.CeremoniaFacultadPrograma.CeremoniaGrado.FechaCeremonia) == "" {
			continue
		}
		fecha, err := parseFechaCeremoniaCRUD(asignacion.CeremoniaFacultadPrograma.CeremoniaGrado.FechaCeremonia)
		if err == nil {
			return fecha, nil
		}
	}
	return time.Time{}, fmt.Errorf("%w: el documento no tiene ceremonia asignada con fecha valida", ErrInvalidState)
}

func parseFechaCeremoniaCRUD(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("fecha vacia")
	}
	if fecha, err := time.Parse(time.RFC3339, value); err == nil {
		return fecha, nil
	}
	if len(value) >= 10 {
		return time.Parse("2006-01-02", value[:10])
	}
	return time.Time{}, fmt.Errorf("fecha invalida")
}

func asignarRegistroDiplomaDigital(ctx context.Context, diplomaID int64) (*diplomaDigitalResponse, error) {
	var diploma diplomaDigitalResponse
	status, err := request.PostWithContext(ctx, fmt.Sprintf("%s/diploma_digital/%d/registro", diplomasCrudURL(), diplomaID), map[string]interface{}{}, &diploma)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud asignar registro diploma_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud asignar registro diploma_digital returned status %d", ErrExternalService, status)
	}
	return &diploma, nil
}

func registrarFirmaDocumentoDiploma(ctx context.Context, documentoID int64, firmante *FirmanteActivo) (*firmaDocumentoResponse, error) {
	estadoCodigo := "FIRMA_PAR"
	if firmante.Orden >= 4 {
		estadoCodigo = "FIRMADO"
	}

	estadoID, err := resolverParametroID(ctx, 0, estadoCodigo)
	if err != nil {
		return nil, err
	}

	firmaFirmante, err := buscarFirmaFirmanteActiva(ctx, firmante.DocumentoIdentidad)
	if err != nil {
		return nil, err
	}

	var firmaFirmanteID *int64
	if firmaFirmante != nil {
		firmaFirmanteID = &firmaFirmante.ID
	}
	payload := registrarFirmaDocumentoRequest{
		FirmaFirmanteID:    firmaFirmanteID,
		RolFirmanteID:      int64(firmante.CargoID),
		DocumentoIdentidad: firmante.DocumentoIdentidad,
		EstadoFirmadoID:    &estadoID,
		Observacion:        fmt.Sprintf("Firma %s registrada en flujo de diploma", firmante.Rol),
	}

	var firmaDocumento firmaDocumentoResponse
	status, err := request.PostWithContext(
		ctx,
		fmt.Sprintf("%s/documento_digital/%d/firma", diplomasCrudURL(), documentoID),
		payload,
		&firmaDocumento,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud registrar firma_documento status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud registrar firma_documento returned status %d", ErrExternalService, status)
	}

	return &firmaDocumento, nil
}

func validarFlujoFirmaDocumento(ctx context.Context, documentoID int64, firmante *FirmanteActivo) error {
	if firmante == nil {
		return fmt.Errorf("%w: firmante is required", ErrInvalidInput)
	}

	estadoFirmadoID, err := resolverParametroID(ctx, 0, "FIRMADO")
	if err != nil {
		return err
	}

	var documento documentoDigitalResponse
	status, err := request.GetWithContext(
		ctx,
		fmt.Sprintf("%s/documento_digital/%d", diplomasCrudURL(), documentoID),
		&documento,
	)
	if err != nil {
		return fmt.Errorf("%w: diplomas_crud documento_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: diplomas_crud documento_digital returned status %d", ErrExternalService, status)
	}
	if documento.EstadoDocumentoID == estadoFirmadoID {
		return fmt.Errorf("%w: el documento ya esta firmado", ErrInvalidState)
	}

	firmas, err := listarFirmasDocumento(ctx, documentoID)
	if err != nil {
		return err
	}
	ordenesFirmadas := make(map[int]bool, len(firmas))
	for _, firma := range firmas {
		orden, ok := ordenRolFirmante(firma.RolFirmanteID)
		if !ok {
			continue
		}
		if orden == firmante.Orden {
			return fmt.Errorf("%w: ya existe firma registrada para el orden %d", ErrInvalidState, orden)
		}
		ordenesFirmadas[orden] = true
	}

	ordenEsperado := 1
	for ordenesFirmadas[ordenEsperado] {
		ordenEsperado++
	}
	if firmante.Orden != ordenEsperado {
		return fmt.Errorf("%w: el siguiente firmante debe ser orden %d", ErrInvalidState, ordenEsperado)
	}

	return nil
}

func listarFirmasDocumento(ctx context.Context, documentoID int64) ([]firmaDocumentoResponse, error) {
	rawQuery := fmt.Sprintf("documento_digital_id:%d,activo:true", documentoID)
	endpoint := fmt.Sprintf("%s/firma_documento/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var firmas []firmaDocumentoResponse
	status, err := request.GetWithContext(ctx, endpoint, &firmas)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud firma_documento status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud firma_documento returned status %d", ErrExternalService, status)
	}
	return firmas, nil
}

func ordenRolFirmante(rolFirmanteID int64) (int, bool) {
	rol, ok := rolesFirmaPermitidos[int(rolFirmanteID)]
	if !ok {
		return 0, false
	}
	return rol.Orden, true
}

type gestorDocumentalDocumentoResponse struct {
	Thumbnail   gestorDocumentalArchivo `json:"thumb:thumbnail"`
	FileContent gestorDocumentalArchivo `json:"file:content"`
	File        string                  `json:"file"`
}

type gestorDocumentalArchivo struct {
	Data     string `json:"data"`
	MimeType string `json:"mime-type"`
}

func consultarFirmaGraficaFirmante(ctx context.Context, documentoIdentidad int64) (string, error) {
	firmaFirmante, err := buscarFirmaFirmanteActiva(ctx, documentoIdentidad)
	if err != nil {
		return "", err
	}
	if firmaFirmante == nil || firmaFirmante.EnlaceFirma == "" {
		return "", fmt.Errorf("%w: el firmante no tiene firma registrada", ErrInvalidState)
	}

	baseURL := gestorDocumentalURL()
	if baseURL == "" {
		return "", fmt.Errorf("%w: GestorDocumentalURL is required", ErrInvalidInput)
	}
	endpoint := fmt.Sprintf("%s/%s", baseURL, url.PathEscape(firmaFirmante.EnlaceFirma))

	var documento gestorDocumentalDocumentoResponse
	status, err := request.GetWithContext(ctx, endpoint, &documento)
	if err != nil {
		return "", fmt.Errorf("%w: gestor_documental document firma status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("%w: gestor_documental document firma returned status %d", ErrExternalService, status)
	}

	if documento.File != "" {
		return normalizarFirmaGraficaBase64(documento.File), nil
	}
	if documento.Thumbnail.Data != "" {
		return documento.Thumbnail.Data, nil
	}
	if documento.FileContent.Data != "" {
		return documento.FileContent.Data, nil
	}
	return "", fmt.Errorf("%w: gestor_documental document firma missing file url", ErrExternalService)
}

func normalizarFirmaGraficaBase64(file string) string {
	file = strings.TrimSpace(file)
	if file == "" || strings.HasPrefix(file, "data:") {
		return file
	}
	mimeType := "image/png"
	if strings.HasPrefix(file, "/9j/") {
		mimeType = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, file)
}

func buscarFirmaFirmanteActiva(ctx context.Context, documentoIdentidad int64) (*FirmaFirmanteCRUDResponse, error) {
	rawQuery := fmt.Sprintf("documento_identidad:%d,activo:true", documentoIdentidad)
	endpoint := fmt.Sprintf("%s/firma_firmante/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var firmas []FirmaFirmanteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &firmas)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud firma_firmante status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud firma_firmante returned status %d", ErrExternalService, status)
	}
	if len(firmas) == 0 {
		return nil, nil
	}
	return &firmas[0], nil
}
