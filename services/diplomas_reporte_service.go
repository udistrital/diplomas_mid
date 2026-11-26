package services

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/udistrital/utils_oas/v2/request"
)

type verificacionReporteDiploma struct {
	FirmaID            string `json:"firma_id"`
	DocumentoID        int64  `json:"documento_id"`
	UUID               string `json:"uuid_documento"`
	Repositorio        string `json:"repositorio_documental"`
	URL                string `json:"qr_url_segura"`
	CodigoAutenticidad string `json:"codigo_autenticidad"`
	HashSHA256         string `json:"hash_sha256"`
	Activo             bool   `json:"activo"`
}

func consultarVerificacionesReporte(ctx context.Context, items []diplomaZIPItem) (map[string]verificacionReporteDiploma, error) {
	base := strings.TrimRight(firmaElectronicaMidURL(), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("%w: URL de Firma Electrónica inválida", ErrInvalidInput)
	}
	result := make(map[string]verificacionReporteDiploma, len(items))
	for start := 0; start < len(items); start += 100 {
		end := start + 100
		if end > len(items) {
			end = len(items)
		}
		ids := make([]string, 0, end-start)
		wanted := map[string]int64{}
		for _, item := range items[start:end] {
			if item.Documento == nil {
				return nil, fmt.Errorf("%w: documento sin metadatos", ErrInvalidState)
			}
			if _, duplicado := wanted[item.UUID]; duplicado {
				return nil, fmt.Errorf("%w: UUID de documento ambiguo", ErrInvalidState)
			}
			ids = append(ids, item.UUID)
			wanted[item.UUID] = item.Documento.ID
		}
		payload := struct {
			UUIDs []string `json:"uuid_documentos"`
		}{ids}
		var response struct {
			Firmas []verificacionReporteDiploma `json:"firmas"`
		}
		status, err := request.PostWithContext(ctx, base+"/v2/firma_electronica/reporte", payload, &response)
		if err != nil || status < 200 || status >= 300 {
			return nil, fmt.Errorf("%w: no fue posible consultar enlaces de verificación para el reporte (status %d)", ErrExternalService, status)
		}
		for _, firma := range response.Firmas {
			documentID, ok := wanted[firma.UUID]
			link, parseErr := url.Parse(firma.URL)
			_, duplicada := result[firma.UUID]
			if !ok || duplicada || firma.DocumentoID != documentID || !firma.Activo || firma.Repositorio != "diplomas" || firma.FirmaID == "" || parseErr != nil || link.Host == "" || (link.Scheme != "https" && link.Scheme != "http") {
				return nil, fmt.Errorf("%w: metadatos de verificación inconsistentes para el reporte", ErrExternalService)
			}
			result[firma.UUID] = firma
		}
		for _, id := range ids {
			if _, ok := result[id]; !ok {
				return nil, fmt.Errorf("%w: no se encontró el enlace QR original del documento %d; no se generó un reporte incompleto", ErrInvalidState, wanted[id])
			}
		}
	}
	return result, nil
}

func numeroReporte(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
func int64Reporte(value *int64) string {
	if value == nil {
		return ""
	}
	return numeroReporte(*value)
}

func filaReporteDiploma(ctx context.Context, item diplomaZIPItem, diploma *diplomaDigitalResponse, firma verificacionReporteDiploma) []string {
	doc := item.Documento
	nombre, tipo, documento, municipio, titulo := doc.NombreEstudiante, doc.TipoDocumentoEstudiante, doc.NumeroDocumentoEstudiante, doc.MunicipioExpedicion, doc.TituloOtorgado
	var diplomaID, consecutivo int64
	var consecutivoFacultad, registroID, registro, folio, libro, fechaGrado, vigencia string
	facultadID := item.FacultadID
	if doc.Vigencia != nil {
		vigencia = numeroReporte(int64(*doc.Vigencia))
	}
	if diploma != nil {
		diplomaID = diploma.ID
		consecutivo = diploma.ConsecutivoDiploma
		nombre = defaultString(diploma.NombreEstudiante, nombre)
		tipo = defaultString(diploma.TipoDocumentoEstudiante, tipo)
		documento = defaultString(diploma.NumeroDocumentoEstudiante, documento)
		municipio = defaultString(diploma.MunicipioExpedicion, municipio)
		titulo = defaultString(diploma.TituloOtorgado, titulo)
		if diploma.FacultadID > 0 {
			facultadID = diploma.FacultadID
		}
		if diploma.Vigencia > 0 {
			vigencia = numeroReporte(int64(diploma.Vigencia))
		}
		// fecha_grado es una fecha civil persistida: no desplazarla por zona horaria.
		if !diploma.FechaGrado.IsZero() {
			fechaGrado = diploma.FechaGrado.Format("2006-01-02")
		}
		if diploma.ConsecutivoFacultad != nil {
			consecutivoFacultad = numeroReporte(int64(*diploma.ConsecutivoFacultad))
		}
		if r := diploma.RegistroGrado; r != nil {
			registroID = numeroReporte(r.ID)
			registro = numeroReporte(int64(r.NumeroRegistro))
			if f := r.FolioRegistro; f != nil {
				if f.NumeroFolio > 0 {
					folio = fmt.Sprintf("%03d", f.NumeroFolio)
				}
				if f.LibroRegistro != nil {
					libro = numeroReporte(int64(f.LibroRegistro.NumeroLibro))
				}
			}
		}
	}
	facultad, programa := "", ""
	if facultadID > 0 {
		facultad = nombreDependenciaOikos(ctx, facultadID)
	}
	if doc.ProgramaAcademicoID != nil && *doc.ProgramaAcademicoID > 0 {
		programa = nombreDependenciaOikos(ctx, *doc.ProgramaAcademicoID)
	}
	fecha := normalizarFechaZIP(item.FechaCeremonia)
	if len(fecha) == 8 {
		fecha = fecha[:4] + "-" + fecha[4:6] + "-" + fecha[6:]
	} else {
		fecha = ""
	}
	return []string{firma.URL, rutaDiplomaZIP(item), item.UUID, numeroReporte(doc.ID), numeroReporte(diplomaID), numeroReporte(doc.CodigoEstudiante), nombre, tipo, documento, municipio, titulo,
		numeroReporte(facultadID), facultad, int64Reporte(doc.ProgramaAcademicoID), programa, int64Reporte(doc.PeriodoID), vigencia, numeroReporte(item.CeremoniaID), item.Lugar, fecha, fechaGrado,
		numeroReporte(consecutivo), consecutivoFacultad, registroID, registro, folio, libro, firma.FirmaID, firma.CodigoAutenticidad, firma.HashSHA256}
}
