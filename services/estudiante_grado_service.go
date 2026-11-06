package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/udistrital/utils_oas/v2/request"
)

type EstudianteGradoService struct{}

const (
	codigoTipoDocumentoDigitalDefault  = "DIP_NORMAL"
	codigoEstadoDocumentoDefault       = "PEND_GEN"
	codigoEstadoDocumentoCreadoDefault = "GENERADO"
)

type parametroItem struct {
	ID                int64
	CodigoAbreviacion string
	Activo            bool
}

func (p *parametroItem) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.ID = numberFromKeys(raw, "Id", "id")
	p.CodigoAbreviacion = stringFromKeys(raw, "CodigoAbreviacion", "codigo_abreviacion")
	p.Activo = boolFromKeys(raw, "Activo", "activo")
	return nil
}

type DatosDiplomaEstudiante struct {
	Nombre               string `json:"nombre"`
	NumeroIdentificacion string `json:"numero_identificacion"`
	Titulo               string `json:"titulo"`
	TipoDocumento        string `json:"tipo_documento"`
	MunicipioExpedicion  string `json:"municipio_expedicion"`
}

type EstudianteAprobadoGrado struct {
	PeriodoID               int64                   `json:"periodo_id"`
	CodigoEstudianteTexto   string                  `json:"codigo_estudiante_texto"`
	ProgramaAcademicoMIDID  int64                   `json:"programa_academico_mid_id"`
	ID                      int64                   `json:"Id"`
	CodigoEstudiante        int64                   `json:"CodigoEstudiante"`
	IDFacultadOikos         int64                   `json:"IdFacultadOikos"`
	IDProyectoOikos         int64                   `json:"IdProyectoOikos"`
	IDFacultadGedep         int64                   `json:"IdFacultadGedep"`
	IDProyectoAccra         int64                   `json:"IdProyectoAccra"`
	AnioInsGrado            int                     `json:"AnioInsGrado"`
	PerInsGrado             int                     `json:"PerInsGrado"`
	Academico               bool                    `json:"Academico"`
	Financiero              bool                    `json:"Financiero"`
	Biblioteca              bool                    `json:"Biblioteca"`
	Laboratorios            bool                    `json:"Laboratorios"`
	Bienestar               bool                    `json:"Bienestar"`
	Urelinter               bool                    `json:"Urelinter"`
	Orc                     bool                    `json:"Orc"`
	ObservacionCoordinacion string                  `json:"ObservacionCoordinacion"`
	ObservacionBiblioteca   string                  `json:"ObservacionBiblioteca"`
	ObservacionLaboratorios string                  `json:"ObservacionLaboratorios"`
	ObservacionBienestar    string                  `json:"ObservacionBienestar"`
	ObservacionUrelinter    string                  `json:"ObservacionUrelinter"`
	ObservacionOrc          string                  `json:"ObservacionOrc"`
	ObservacionFinanciera   string                  `json:"ObservacionFinanciera"`
	Activo                  bool                    `json:"Activo"`
	FechaCreacion           string                  `json:"FechaCreacion"`
	FechaModificacion       string                  `json:"FechaModificacion"`
	DatosDiploma            *DatosDiplomaEstudiante `json:"datos_diploma,omitempty"`
}

type EstudiantesAprobadosFacultad struct {
	FacultadID  int64                     `json:"facultad_id"`
	Total       int                       `json:"total"`
	Estudiantes []EstudianteAprobadoGrado `json:"estudiantes"`
}

type EstudianteAprobadoError struct {
	CodigoEstudiante int64  `json:"codigo_estudiante"`
	FacultadID       int64  `json:"facultad_id"`
	Error            string `json:"error"`
}

type EstudiantesAprobadosGradoResult struct {
	Total               int                            `json:"total"`
	Errores             int                            `json:"errores"`
	Facultades          []EstudiantesAprobadosFacultad `json:"facultades"`
	EstudiantesConError []EstudianteAprobadoError      `json:"estudiantes_con_error,omitempty"`
}

type ListarAprobadosPorFacultadInput struct {
	PeriodoID                  int64   `json:"periodo_id"`
	TipoDocumentoDigitalID     int64   `json:"tipo_documento_digital_id,omitempty"`
	TipoDocumentoDigitalCodigo string  `json:"tipo_documento_digital_codigo,omitempty"`
	CodigoEstudiante           int64   `json:"codigo_estudiante,omitempty"`
	CodigosEstudiante          []int64 `json:"codigos_estudiante,omitempty"`
	FacultadID                 int64   `json:"facultad_id,omitempty"`
	ProgramaAcademicoID        int64   `json:"programa_academico_id,omitempty"`
	ExcluirRegistrados         bool    `json:"excluir_registrados"`
}

type CrearDocumentosAprobadosInput struct {
	PeriodoID                   int64     `json:"periodo_id"`
	TipoDocumentoDigitalID      int64     `json:"tipo_documento_digital_id,omitempty"`
	TipoDocumentoDigitalCodigo  string    `json:"tipo_documento_digital_codigo,omitempty"`
	EstadoDocumentoID           int64     `json:"estado_documento_id,omitempty"`
	EstadoDocumentoCodigo       string    `json:"estado_documento_codigo,omitempty"`
	CodigoEstudiante            int64     `json:"codigo_estudiante,omitempty"`
	CodigosEstudiante           []int64   `json:"codigos_estudiante,omitempty"`
	FacultadID                  int64     `json:"facultad_id,omitempty"`
	ProgramaAcademicoID         int64     `json:"programa_academico_id,omitempty"`
	CrearDiplomas               bool      `json:"crear_diplomas,omitempty"`
	FechaGrado                  time.Time `json:"fecha_grado,omitempty"`
	EstadoDocumentoCreadoID     int64     `json:"estado_documento_creado_id,omitempty"`
	EstadoDocumentoCreadoCodigo string    `json:"estado_documento_creado_codigo,omitempty"`
	DryRun                      bool      `json:"dry_run,omitempty"`
}

type CrearDocumentoAprobadoItem struct {
	Estudiante       EstudianteAprobadoGrado      `json:"estudiante"`
	Payload          crearDocumentoDigitalRequest `json:"payload"`
	DiplomaPayload   *crearDiplomaDigitalRequest  `json:"diploma_payload,omitempty"`
	DocumentoDigital *documentoDigitalResponse    `json:"documento_digital,omitempty"`
	DiplomaDigital   *diplomaDigitalResponse      `json:"diploma_digital,omitempty"`
	Creado           bool                         `json:"creado"`
	Existente        bool                         `json:"existente"`
	DiplomaCreado    bool                         `json:"diploma_creado"`
	DiplomaExistente bool                         `json:"diploma_existente"`
	Error            string                       `json:"error,omitempty"`
}

type CrearDocumentosAprobadosFacultad struct {
	FacultadID         int64                        `json:"facultad_id"`
	Total              int                          `json:"total"`
	Creados            int                          `json:"creados"`
	Existentes         int                          `json:"existentes"`
	DiplomasCreados    int                          `json:"diplomas_creados"`
	DiplomasExistentes int                          `json:"diplomas_existentes"`
	Errores            int                          `json:"errores"`
	Documentos         []CrearDocumentoAprobadoItem `json:"documentos"`
}

type CrearDocumentosAprobadosResult struct {
	Total              int                                `json:"total"`
	Creados            int                                `json:"creados"`
	Existentes         int                                `json:"existentes"`
	DiplomasCreados    int                                `json:"diplomas_creados"`
	DiplomasExistentes int                                `json:"diplomas_existentes"`
	Errores            int                                `json:"errores"`
	DryRun             bool                               `json:"dry_run"`
	Facultades         []CrearDocumentosAprobadosFacultad `json:"facultades"`
}

func (e EstudianteAprobadoGrado) DocumentoDigitalRequest(tipoDocumentoDigitalID, estadoDocumentoID int64) crearDocumentoDigitalRequest {
	payload := crearDocumentoDigitalRequest{
		TipoDocumentoDigitalID: tipoDocumentoDigitalID,
		EstadoDocumentoID:      estadoDocumentoID,
		CodigoEstudiante:       e.CodigoEstudiante,
		FacultadID:             e.IDFacultadOikos,
		ProgramaAcademicoID:    e.IDProyectoOikos,
		PeriodoID:              e.PeriodoID,
		Vigencia:               e.AnioInsGrado,
		Activo:                 true,
	}
	if e.DatosDiploma != nil {
		payload.NombreEstudiante = e.DatosDiploma.Nombre
		payload.TipoDocumentoEstudiante = e.DatosDiploma.TipoDocumento
		payload.NumeroDocumentoEstudiante = e.DatosDiploma.NumeroIdentificacion
		payload.MunicipioExpedicion = e.DatosDiploma.MunicipioExpedicion
		payload.TituloOtorgado = e.DatosDiploma.Titulo
	}
	return payload
}

func (e EstudianteAprobadoGrado) DiplomaDigitalRequest(fechaGrado time.Time, estadoDocumentoCreadoID int64) crearDiplomaDigitalRequest {
	payload := crearDiplomaDigitalRequest{
		FacultadID:              e.IDFacultadOikos,
		Vigencia:                e.AnioInsGrado,
		FechaGrado:              fechaGrado,
		EstadoDocumentoCreadoID: estadoDocumentoCreadoID,
	}
	if e.DatosDiploma != nil {
		payload.TituloOtorgado = e.DatosDiploma.Titulo
		payload.NombreEstudiante = e.DatosDiploma.Nombre
		payload.NumeroDocumentoEstudiante = e.DatosDiploma.NumeroIdentificacion
		payload.TipoDocumentoEstudiante = e.DatosDiploma.TipoDocumento
		payload.MunicipioExpedicion = e.DatosDiploma.MunicipioExpedicion
	}
	return payload
}

func (s EstudianteGradoService) ListarAprobadosPorFacultad(ctx context.Context, input *ListarAprobadosPorFacultadInput) (*EstudiantesAprobadosGradoResult, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: periodo_id es obligatorio", ErrInvalidInput)
	}
	estudiantes, err := s.consultarEstudiantesAprobados(ctx, input.PeriodoID)
	if err != nil {
		return nil, err
	}
	if input != nil {
		tipoDocumentoDigitalID, err := resolverParametroID(ctx, input.TipoDocumentoDigitalID, input.TipoDocumentoDigitalCodigo)
		if err != nil {
			return nil, err
		}
		estudiantes = filtrarEstudiantesAprobados(estudiantes, *input)
		if input.ExcluirRegistrados {
			estudiantes, err = excluirEstudiantesConDocumentoActivo(ctx, estudiantes, tipoDocumentoDigitalID)
			if err != nil {
				return nil, err
			}
		}
	}
	var errores []EstudianteAprobadoError
	estudiantes, errores = enriquecerEstudiantesAprobadosDisponibles(ctx, estudiantes)

	result := agruparEstudiantesAprobadosPorFacultad(estudiantes)
	result.Errores = len(errores)
	result.EstudiantesConError = errores
	return &result, nil
}

func (s EstudianteGradoService) CrearDocumentosAprobados(ctx context.Context, input *CrearDocumentosAprobadosInput) (*CrearDocumentosAprobadosResult, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.PeriodoID <= 0 {
		return nil, fmt.Errorf("%w: periodo_id es obligatorio", ErrInvalidInput)
	}

	tipoDocumentoDigitalID, err := resolverParametroID(
		ctx,
		input.TipoDocumentoDigitalID,
		defaultString(input.TipoDocumentoDigitalCodigo, codigoTipoDocumentoDigitalDefault),
	)
	if err != nil {
		return nil, err
	}
	estadoDocumentoID, err := resolverParametroID(
		ctx,
		input.EstadoDocumentoID,
		defaultString(input.EstadoDocumentoCodigo, codigoEstadoDocumentoDefault),
	)
	if err != nil {
		return nil, err
	}
	if input.CrearDiplomas && input.FechaGrado.IsZero() {
		return nil, fmt.Errorf("%w: fecha_grado is required when crear_diplomas is true", ErrInvalidInput)
	}
	estadoDocumentoCreadoID := input.EstadoDocumentoCreadoID
	if input.CrearDiplomas {
		estadoDocumentoCreadoID, err = resolverParametroID(
			ctx,
			input.EstadoDocumentoCreadoID,
			defaultString(input.EstadoDocumentoCreadoCodigo, codigoEstadoDocumentoCreadoDefault),
		)
		if err != nil {
			return nil, err
		}
	}

	estudiantes, err := s.consultarEstudiantesAprobados(ctx, input.PeriodoID)
	if err != nil {
		return nil, err
	}
	estudiantes = filtrarEstudiantesAprobados(estudiantes, ListarAprobadosPorFacultadInput{
		CodigoEstudiante:    input.CodigoEstudiante,
		CodigosEstudiante:   input.CodigosEstudiante,
		FacultadID:          input.FacultadID,
		ProgramaAcademicoID: input.ProgramaAcademicoID,
	})

	result := CrearDocumentosAprobadosResult{
		Total:  len(estudiantes),
		DryRun: input.DryRun,
	}
	grupos := make(map[int64]*CrearDocumentosAprobadosFacultad)

	for _, estudiante := range estudiantes {
		estudianteEnriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante)
		if err != nil {
			item := CrearDocumentoAprobadoItem{
				Estudiante: estudiante,
				Error:      err.Error(),
			}
			result.Errores++
			agregarDocumentoAGrupo(grupos, item)
			continue
		}
		estudiante = estudianteEnriquecido
		payload := estudiante.DocumentoDigitalRequest(tipoDocumentoDigitalID, estadoDocumentoID)
		item := CrearDocumentoAprobadoItem{
			Estudiante: estudiante,
			Payload:    payload,
		}
		if input.CrearDiplomas {
			diplomaPayload := estudiante.DiplomaDigitalRequest(input.FechaGrado, estadoDocumentoCreadoID)
			item.DiplomaPayload = &diplomaPayload
		}

		if !input.DryRun {
			documento, err := buscarDocumentoDigitalExistente(ctx, payload)
			if err != nil {
				item.Error = err.Error()
				result.Errores++
			} else if documento != nil {
				item.DocumentoDigital = documento
				item.Existente = true
				result.Existentes++
			} else {
				documento, err = crearDocumentoDigital(ctx, payload)
				if err != nil {
					item.Error = err.Error()
					result.Errores++
				} else {
					item.DocumentoDigital = documento
					item.Creado = true
					result.Creados++
				}
			}
			if item.Error == "" && input.CrearDiplomas && item.DocumentoDigital != nil {
				diploma, err := buscarDiplomaDigitalExistente(ctx, item.DocumentoDigital.ID)
				if err != nil {
					item.Error = err.Error()
					result.Errores++
				} else if diploma != nil {
					item.DiplomaDigital = diploma
					item.DiplomaExistente = true
					result.DiplomasExistentes++
				} else {
					diploma, err = crearDiplomaDigital(ctx, item.DocumentoDigital.ID, *item.DiplomaPayload)
					if err != nil {
						item.Error = err.Error()
						result.Errores++
					} else {
						item.DiplomaDigital = diploma
						item.DiplomaCreado = true
						result.DiplomasCreados++
					}
				}
			}
		}

		agregarDocumentoAGrupo(grupos, item)
	}

	facultadIDs := make([]int64, 0, len(grupos))
	for facultadID := range grupos {
		facultadIDs = append(facultadIDs, facultadID)
	}
	sort.Slice(facultadIDs, func(i, j int) bool {
		return facultadIDs[i] < facultadIDs[j]
	})

	result.Facultades = make([]CrearDocumentosAprobadosFacultad, 0, len(facultadIDs))
	for _, facultadID := range facultadIDs {
		result.Facultades = append(result.Facultades, *grupos[facultadID])
	}

	return &result, nil
}

func filtrarEstudiantesAprobadosPorCodigo(estudiantes []EstudianteAprobadoGrado, codigoEstudiante int64) []EstudianteAprobadoGrado {
	return filtrarEstudiantesAprobados(estudiantes, ListarAprobadosPorFacultadInput{CodigoEstudiante: codigoEstudiante})
}

func filtrarEstudiantesAprobados(estudiantes []EstudianteAprobadoGrado, input ListarAprobadosPorFacultadInput) []EstudianteAprobadoGrado {
	codigos := make(map[int64]struct{}, len(input.CodigosEstudiante)+1)
	if input.CodigoEstudiante > 0 {
		codigos[input.CodigoEstudiante] = struct{}{}
	}
	for _, codigo := range input.CodigosEstudiante {
		if codigo > 0 {
			codigos[codigo] = struct{}{}
		}
	}
	if len(codigos) == 0 && input.FacultadID <= 0 && input.ProgramaAcademicoID <= 0 {
		return estudiantes
	}

	filtrados := make([]EstudianteAprobadoGrado, 0, len(estudiantes))
	for _, estudiante := range estudiantes {
		if len(codigos) > 0 {
			if _, ok := codigos[estudiante.CodigoEstudiante]; !ok {
				continue
			}
		}
		if input.FacultadID > 0 && estudiante.IDFacultadOikos != input.FacultadID {
			continue
		}
		if input.ProgramaAcademicoID > 0 && estudiante.IDProyectoOikos != input.ProgramaAcademicoID {
			continue
		}
		filtrados = append(filtrados, estudiante)
	}
	return filtrados
}

func (s EstudianteGradoService) consultarEstudiantesAprobados(ctx context.Context, periodoID int64) ([]EstudianteAprobadoGrado, error) {
	return consultarAprobadosMID(ctx, periodoID)
}

func enriquecerEstudiantesAprobados(ctx context.Context, estudiantes []EstudianteAprobadoGrado) ([]EstudianteAprobadoGrado, error) {
	for index, estudiante := range estudiantes {
		estudianteEnriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante)
		if err != nil {
			return nil, err
		}
		estudiantes[index] = estudianteEnriquecido
	}

	return estudiantes, nil
}

func enriquecerEstudiantesAprobadosDisponibles(ctx context.Context, estudiantes []EstudianteAprobadoGrado) ([]EstudianteAprobadoGrado, []EstudianteAprobadoError) {
	enriquecidos := make([]EstudianteAprobadoGrado, 0, len(estudiantes))
	errores := make([]EstudianteAprobadoError, 0)
	for _, estudiante := range estudiantes {
		estudianteEnriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante)
		if err != nil {
			errores = append(errores, EstudianteAprobadoError{
				CodigoEstudiante: estudiante.CodigoEstudiante,
				FacultadID:       estudiante.IDFacultadOikos,
				Error:            err.Error(),
			})
			continue
		}
		enriquecidos = append(enriquecidos, estudianteEnriquecido)
	}

	return enriquecidos, errores
}

func enriquecerEstudianteAprobado(ctx context.Context, estudiante EstudianteAprobadoGrado) (EstudianteAprobadoGrado, error) {
	datosDiploma, err := consultarDatosDiplomaEstudiante(ctx, estudiante.CodigoEstudiante)
	if err != nil {
		return estudiante, err
	}
	estudiante.DatosDiploma = datosDiploma
	return estudiante, nil
}

func consultarDatosDiplomaEstudiante(ctx context.Context, codigoEstudiante int64) (*DatosDiplomaEstudiante, error) {
	endpoint := fmt.Sprintf("%s/v1/estudiantes/datos-diploma/%d", strings.TrimRight(academicaCoreServiceURL(), "/"), codigoEstudiante)

	var datos DatosDiplomaEstudiante
	status, err := request.GetWithContext(ctx, endpoint, &datos)
	if err != nil {
		return nil, fmt.Errorf("%w: academica_core_service datos-diploma estudiante %d status %d: %v", ErrExternalService, codigoEstudiante, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: academica_core_service datos-diploma estudiante %d returned status %d", ErrExternalService, codigoEstudiante, status)
	}

	return &datos, nil
}

func resolverParametroID(ctx context.Context, id int64, codigo string) (int64, error) {
	codigo = strings.TrimSpace(codigo)
	if codigo == "" {
		if id > 0 {
			return id, nil
		}
		return 0, nil
	}

	parametro, err := consultarParametroPorCodigo(ctx, codigo)
	if err != nil {
		return 0, err
	}
	if parametro == nil {
		return 0, fmt.Errorf("%w: parametro codigo_abreviacion %s not found", ErrInvalidInput, codigo)
	}
	if !parametro.Activo {
		return 0, fmt.Errorf("%w: parametro codigo_abreviacion %s inactive", ErrInvalidInput, codigo)
	}
	return parametro.ID, nil
}

func consultarParametroPorCodigo(ctx context.Context, codigo string) (*parametroItem, error) {
	endpoint := fmt.Sprintf(
		"%s/parametro?query=%s",
		parametrosCrudURL(),
		url.QueryEscape("CodigoAbreviacion:"+codigo),
	)

	var raw json.RawMessage
	status, err := request.GetWithContext(ctx, endpoint, &raw)
	if err != nil {
		return nil, fmt.Errorf("%w: parametros_crud parametro %s status %d: %v", ErrExternalService, codigo, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: parametros_crud parametro %s returned status %d", ErrExternalService, codigo, status)
	}

	parametros, err := parseParametroResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: parametros_crud parametro %s invalid response: %v", ErrExternalService, codigo, err)
	}
	for _, parametro := range parametros {
		if strings.EqualFold(parametro.CodigoAbreviacion, codigo) {
			return &parametro, nil
		}
	}
	return nil, nil
}

func parseParametroResponse(raw json.RawMessage) ([]parametroItem, error) {
	var wrapper struct {
		Data []parametroItem `json:"Data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Data != nil {
		return wrapper.Data, nil
	}

	var direct []parametroItem
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, err
	}
	return direct, nil
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func numberFromKeys(raw map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case float64:
			return int64(value)
		case string:
			var number int64
			if _, err := fmt.Sscanf(value, "%d", &number); err == nil {
				return number
			}
		}
	}
	return 0
}

func stringFromKeys(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok {
			return value
		}
	}
	return ""
}

func boolFromKeys(raw map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case bool:
			return value
		case string:
			return strings.EqualFold(value, "true")
		}
	}
	return false
}

func excluirEstudiantesConDocumentoActivo(ctx context.Context, estudiantes []EstudianteAprobadoGrado, tipoDocumentoDigitalID int64) ([]EstudianteAprobadoGrado, error) {
	disponibles := make([]EstudianteAprobadoGrado, 0, len(estudiantes))
	for _, estudiante := range estudiantes {
		payload := estudiante.DocumentoDigitalRequest(tipoDocumentoDigitalID, 0)
		documento, err := buscarDocumentoDigitalExistente(ctx, payload)
		if err != nil {
			return nil, err
		}
		if documento == nil {
			disponibles = append(disponibles, estudiante)
		}
	}
	return disponibles, nil
}

func agregarDocumentoAGrupo(grupos map[int64]*CrearDocumentosAprobadosFacultad, item CrearDocumentoAprobadoItem) {
	facultadID := item.Estudiante.IDFacultadOikos
	grupo := grupos[facultadID]
	if grupo == nil {
		grupo = &CrearDocumentosAprobadosFacultad{FacultadID: facultadID}
		grupos[facultadID] = grupo
	}
	grupo.Total++
	if item.Creado {
		grupo.Creados++
	}
	if item.Existente {
		grupo.Existentes++
	}
	if item.DiplomaCreado {
		grupo.DiplomasCreados++
	}
	if item.DiplomaExistente {
		grupo.DiplomasExistentes++
	}
	if item.Error != "" {
		grupo.Errores++
	}
	grupo.Documentos = append(grupo.Documentos, item)
}

func buscarDocumentoDigitalExistente(ctx context.Context, payload crearDocumentoDigitalRequest) (*documentoDigitalResponse, error) {
	rawQuery := fmt.Sprintf(
		"codigo_estudiante:%d,vigencia:%d,programa_academico_id:%d,periodo_id:%d,activo:true",
		payload.CodigoEstudiante,
		payload.Vigencia,
		payload.ProgramaAcademicoID,
		payload.PeriodoID,
	)
	if payload.TipoDocumentoDigitalID > 0 {
		rawQuery = fmt.Sprintf("%s,tipo_documento_digital_id:%d", rawQuery, payload.TipoDocumentoDigitalID)
	}
	endpoint := fmt.Sprintf("%s/documento_digital/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var documentos []documentoDigitalResponse
	status, err := request.GetWithContext(ctx, endpoint, &documentos)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital returned status %d", ErrExternalService, status)
	}
	if len(documentos) == 0 {
		return nil, nil
	}

	return &documentos[0], nil
}

func crearDocumentoDigital(ctx context.Context, payload crearDocumentoDigitalRequest) (*documentoDigitalResponse, error) {
	var documento documentoDigitalResponse
	status, err := request.PostWithContext(ctx, diplomasCrudURL()+"/documento_digital/", payload, &documento)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud crear documento_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud crear documento_digital returned status %d", ErrExternalService, status)
	}

	return &documento, nil
}

func buscarDiplomaDigitalExistente(ctx context.Context, documentoDigitalID int64) (*diplomaDigitalResponse, error) {
	rawQuery := fmt.Sprintf("documento_digital_id:%d", documentoDigitalID)
	endpoint := fmt.Sprintf("%s/diploma_digital/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))

	var diplomas []diplomaDigitalResponse
	status, err := request.GetWithContext(ctx, endpoint, &diplomas)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud diploma_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud diploma_digital returned status %d", ErrExternalService, status)
	}
	if len(diplomas) == 0 {
		return nil, nil
	}

	return &diplomas[0], nil
}

func crearDiplomaDigital(ctx context.Context, documentoDigitalID int64, payload crearDiplomaDigitalRequest) (*diplomaDigitalResponse, error) {
	endpoint := fmt.Sprintf("%s/documento_digital/%d/diploma", diplomasCrudURL(), documentoDigitalID)

	var diploma diplomaDigitalResponse
	status, err := request.PostWithContext(ctx, endpoint, payload, &diploma)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud crear diploma_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud crear diploma_digital returned status %d", ErrExternalService, status)
	}

	return &diploma, nil
}

func agruparEstudiantesAprobadosPorFacultad(estudiantes []EstudianteAprobadoGrado) EstudiantesAprobadosGradoResult {
	grupos := make(map[int64][]EstudianteAprobadoGrado)
	for _, estudiante := range estudiantes {
		grupos[estudiante.IDFacultadOikos] = append(grupos[estudiante.IDFacultadOikos], estudiante)
	}

	facultadIDs := make([]int64, 0, len(grupos))
	for facultadID := range grupos {
		facultadIDs = append(facultadIDs, facultadID)
	}
	sort.Slice(facultadIDs, func(i, j int) bool {
		return facultadIDs[i] < facultadIDs[j]
	})

	facultades := make([]EstudiantesAprobadosFacultad, 0, len(facultadIDs))
	for _, facultadID := range facultadIDs {
		estudiantesFacultad := grupos[facultadID]
		facultades = append(facultades, EstudiantesAprobadosFacultad{
			FacultadID:  facultadID,
			Total:       len(estudiantesFacultad),
			Estudiantes: estudiantesFacultad,
		})
	}

	return EstudiantesAprobadosGradoResult{
		Total:      len(estudiantes),
		Facultades: facultades,
	}
}
