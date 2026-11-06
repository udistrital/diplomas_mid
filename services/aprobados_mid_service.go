package services

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/udistrital/utils_oas/v2/request"
)

type PeriodoAcademico struct {
	ID                int64  `json:"Id"`
	Nombre            string `json:"Nombre"`
	Year              int    `json:"Year"`
	Ciclo             string `json:"Ciclo"`
	CodigoAbreviacion string `json:"CodigoAbreviacion"`
	Activo            bool   `json:"Activo"`
}

func ListarPeriodosAcademicos(ctx context.Context) ([]PeriodoAcademico, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var response struct {
		Success bool
		Status  string
		Data    *[]PeriodoAcademico
	}
	endpoint := parametrosCrudURL() + "/periodo?query=CodigoAbreviacion:PA&sortby=Year&sortby=Ciclo&order=desc&limit=-1"
	status, err := request.GetWithContext(ctx, endpoint, &response)
	if err != nil || status != 200 || !response.Success || response.Status != "200" || response.Data == nil {
		return nil, fmt.Errorf("%w: Parámetros períodos status %d: %v", ErrExternalService, status, err)
	}
	result := make([]PeriodoAcademico, 0)
	seen := map[int64]bool{}
	for _, p := range *response.Data {
		if p.CodigoAbreviacion != "PA" {
			continue
		}
		ciclo, e := strconv.Atoi(p.Ciclo)
		if p.ID <= 0 || p.Year <= 0 || e != nil || ciclo <= 0 || seen[p.ID] {
			return nil, fmt.Errorf("%w: período inválido o repetido", ErrExternalService)
		}
		seen[p.ID] = true
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Year != result[j].Year {
			return result[i].Year > result[j].Year
		}
		a, _ := strconv.Atoi(result[i].Ciclo)
		b, _ := strconv.Atoi(result[j].Ciclo)
		if a != b {
			return a > b
		}
		return result[i].ID > result[j].ID
	})
	return result, nil
}

func obtenerPeriodoAcademico(ctx context.Context, id int64) (PeriodoAcademico, error) {
	if id <= 0 {
		return PeriodoAcademico{}, fmt.Errorf("%w: periodo_id es obligatorio", ErrInvalidInput)
	}
	periodos, err := ListarPeriodosAcademicos(ctx)
	if err != nil {
		return PeriodoAcademico{}, err
	}
	for _, p := range periodos {
		if p.ID == id {
			return p, nil
		}
	}
	return PeriodoAcademico{}, fmt.Errorf("%w: periodo_id no existe o no es PA", ErrInvalidInput)
}

type solicitudAprobadaMID struct {
	SolicitudID         int64 `json:"SolicitudId"`
	TerceroID           int64 `json:"TerceroId"`
	CodigoEstudiante    string
	PeriodoID           int64 `json:"PeriodoId"`
	ProgramaAcademicoID int64 `json:"ProgramaAcademicoId"`
	DependenciaOikosID  int64 `json:"DependenciaOikosId"`
	FechaAprobacion     string
}

func consultarAprobadosMID(ctx context.Context, periodoID int64) ([]EstudianteAprobadoGrado, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	periodo, err := obtenerPeriodoAcademico(ctx, periodoID)
	if err != nil {
		return nil, err
	}
	if pazYSalvosMidURL() == "" {
		return nil, fmt.Errorf("%w: falta PAZ_Y_SALVOS_MID_URL", ErrExternalService)
	}
	solicitudes := []solicitudAprobadaMID{}
	seen := map[int64]bool{}
	codes := map[string]bool{}
	total := -1
	for offset := 0; ; {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var response struct {
			Success bool
			Status  int
			Data    *struct {
				Solicitudes *[]solicitudAprobadaMID
				Total       *int
			}
		}
		endpoint := fmt.Sprintf("%s/solicitud-grado/paz-salvos/aprobados?periodo_id=%d&limit=20&offset=%d", pazYSalvosMidURL(), periodoID, offset)
		status, e := request.GetWithContext(ctx, endpoint, &response)
		if e != nil || status != 200 || !response.Success || response.Status != 200 || response.Data == nil || response.Data.Solicitudes == nil || response.Data.Total == nil {
			return nil, fmt.Errorf("%w: MID aprobados status %d: %v", ErrExternalService, status, e)
		}
		d := response.Data
		if *d.Total < 0 || (total >= 0 && total != *d.Total) {
			return nil, fmt.Errorf("%w: total MID inconsistente", ErrExternalService)
		}
		total = *d.Total
		page := *d.Solicitudes
		if len(page) > 20 || offset+len(page) > total || (len(page) == 0 && offset < total) {
			return nil, fmt.Errorf("%w: página MID incoherente", ErrExternalService)
		}
		for _, item := range page {
			if item.SolicitudID <= 0 || seen[item.SolicitudID] || codes[item.CodigoEstudiante] || item.PeriodoID != periodoID || item.DependenciaOikosID <= 0 || item.TerceroID <= 0 {
				return nil, fmt.Errorf("%w: solicitud MID inválida o duplicada", ErrExternalService)
			}
			seen[item.SolicitudID] = true
			codes[item.CodigoEstudiante] = true
			solicitudes = append(solicitudes, item)
		}
		offset += len(page)
		if offset == total {
			break
		}
	}
	result := make([]EstudianteAprobadoGrado, 0, len(solicitudes))
	facultades := map[int64]int64{}
	ciclo, _ := strconv.Atoi(periodo.Ciclo)
	for _, item := range solicitudes {
		codigo, e := strconv.ParseInt(item.CodigoEstudiante, 10, 64)
		// Persisted CRUD accepts numeric codes. Reject lossy conversions rather than changing identity.
		if e != nil || codigo <= 0 || strconv.FormatInt(codigo, 10) != item.CodigoEstudiante {
			return nil, fmt.Errorf("%w: código %q incompatible con contrato numérico de documentos", ErrExternalService, item.CodigoEstudiante)
		}
		facultad := facultades[item.DependenciaOikosID]
		if facultad == 0 {
			facultad, err = resolverFacultadOikos(ctx, item.DependenciaOikosID, map[int64]bool{})
			if err != nil {
				return nil, fmt.Errorf("estudiante %s: %w", item.CodigoEstudiante, err)
			}
			facultades[item.DependenciaOikosID] = facultad
		}
		result = append(result, EstudianteAprobadoGrado{ID: item.SolicitudID, CodigoEstudiante: codigo, CodigoEstudianteTexto: item.CodigoEstudiante, PeriodoID: periodoID, ProgramaAcademicoMIDID: item.ProgramaAcademicoID, IDProyectoOikos: item.DependenciaOikosID, IDFacultadOikos: facultad, AnioInsGrado: periodo.Year, PerInsGrado: ciclo, Activo: true, FechaModificacion: item.FechaAprobacion})
	}
	return result, nil
}

func resolverFacultadOikos(ctx context.Context, hija int64, visitados map[int64]bool) (int64, error) {
	if visitados[hija] || len(visitados) >= 10 {
		return 0, fmt.Errorf("%w: jerarquía Oikos cíclica", ErrExternalService)
	}
	visitados[hija] = true
	base := strings.TrimSuffix(strings.TrimSuffix(oikosAPIURL(), "/v1"), "/v2")
	if base == "" {
		return 0, fmt.Errorf("%w: falta OIKOS_API_URL", ErrExternalService)
	}
	var relaciones []struct {
		ID     int64 `json:"Id"`
		Activo bool
		HijaID struct {
			ID     int64 `json:"Id"`
			Activo bool
		} `json:"HijaId"`
		PadreID struct {
			ID     int64 `json:"Id"`
			Nombre string
			Activo bool
		} `json:"PadreId"`
	}
	endpoint := base + "/v2/dependencia_padre/?query=" + url.QueryEscape(fmt.Sprintf("HijaId:%d", hija))
	status, err := request.GetWithContext(ctx, endpoint, &relaciones)
	if err != nil || status != 200 {
		return 0, fmt.Errorf("%w: Oikos status %d: %v", ErrExternalService, status, err)
	}
	padres := map[int64]string{}
	for _, r := range relaciones {
		if r.Activo && r.HijaID.ID == hija && r.HijaID.Activo && r.PadreID.Activo && r.PadreID.ID > 0 {
			padres[r.PadreID.ID] = r.PadreID.Nombre
		}
	}
	if len(padres) != 1 {
		return 0, fmt.Errorf("%w: facultad Oikos ambigua o ausente para %d", ErrExternalService, hija)
	}
	for id, nombre := range padres {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(nombre)), "FACULTAD ") {
			return id, nil
		}
		return resolverFacultadOikos(ctx, id, visitados)
	}
	return 0, fmt.Errorf("%w: facultad Oikos ausente", ErrExternalService)
}
