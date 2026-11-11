package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/udistrital/diplomas_mid/services"
)

type EstudianteGradoController struct {
	APIController
	Service services.EstudianteGradoService
}

func (c *EstudianteGradoController) URLMapping() {}

func (c *EstudianteGradoController) AprobadosPorFacultad() {
	input := listarAprobadosInputFromQuery(c)
	result, err := c.Service.ListarAprobadosPorFacultad(c.requestContext(), &input)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusOK, result)
}

func (c *EstudianteGradoController) CrearDocumentosAprobados() {
	payload, err := decodeBody[services.CrearDocumentosAprobadosInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}

	result, err := c.Service.CrearDocumentosAprobados(c.requestContext(), payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	status := http.StatusCreated
	if result.DryRun {
		status = http.StatusOK
	}
	if result.Errores > 0 {
		status = http.StatusMultiStatus
	}
	c.writeJSON(status, result)
}

func listarAprobadosInputFromQuery(c *EstudianteGradoController) services.ListarAprobadosPorFacultadInput {
	return services.ListarAprobadosPorFacultadInput{
		PeriodoID:                  queryInt64(c, "periodo_id"),
		TipoDocumentoDigitalID:     queryInt64(c, "tipo_documento_digital_id"),
		TipoDocumentoDigitalCodigo: strings.TrimSpace(c.GetString("tipo_documento_digital_codigo")),
		CodigoEstudiante:           queryInt64(c, "codigo_estudiante"),
		CodigosEstudiante:          queryInt64List(c.GetString("codigos_estudiante")),
		FacultadID:                 queryInt64(c, "facultad_id"),
		ProgramaAcademicoID:        queryInt64(c, "programa_academico_id"),
		ExcluirRegistrados:         c.GetString("excluir_registrados") != "false",
	}
}

func queryInt64(c *EstudianteGradoController, key string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(c.GetString(key)), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func queryInt64List(raw string) []int64 {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && value > 0 {
			values = append(values, value)
		}
	}
	return values
}

func (c *EstudianteGradoController) PeriodosAcademicos() {
	result, err := services.ListarPeriodosAcademicos(c.requestContext())
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}
