package controllers

import (
	"net/http"
	"strconv"

	"github.com/udistrital/diplomas_mid/services"
)

type CeremoniaController struct {
	APIController
	Service services.CeremoniaService
}

func (c *CeremoniaController) URLMapping() {}

func (c *CeremoniaController) writeFirmaMasiva(result *services.FirmaMasivaCeremoniaResult) {
	status := http.StatusOK
	if result != nil && result.Firmados == 0 && result.Errores > 0 {
		status = http.StatusConflict
	}
	c.writeJSON(status, result)
}

func (c *CeremoniaController) Listar() {
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_secretario"), 10, 64)
	result, err := c.Service.Listar(c.requestContext(), &services.ListarCeremoniasInput{PeriodoID: periodoQuery(c.GetString("periodo_id")), DocumentoIdentidadSecretario: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ObtenerParametrizacionRegistro() {
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_secretario"), 10, 64)
	vigencia, _ := strconv.Atoi(c.GetString("vigencia"))
	result, err := c.Service.ObtenerParametrizacionRegistro(c.requestContext(), documento, vigencia)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	if result == nil {
		c.writeJSON(http.StatusOK, map[string]interface{}{"vigencia": vigencia, "existe": false})
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) GuardarParametrizacionRegistro() {
	payload, err := decodeBody[services.ParametrizacionRegistroInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.GuardarParametrizacionRegistro(c.requestContext(), payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ListarDecano() {
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_decano"), 10, 64)
	result, err := c.Service.ListarDecano(c.requestContext(), &services.ListarCeremoniasInput{DocumentoIdentidadSecretario: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ListarSecretariaGeneral() {
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_firmante"), 10, 64)
	result, err := c.Service.ListarSecretariaGeneral(c.requestContext(), &services.ListarCeremoniasInput{DocumentoIdentidadFirmante: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ListarRector() {
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_firmante"), 10, 64)
	result, err := c.Service.ListarRector(c.requestContext(), &services.ListarCeremoniasInput{DocumentoIdentidadFirmante: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) Actualizar() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.ActualizarCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.Actualizar(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) Eliminar() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_secretario"), 10, 64)
	err = c.Service.Eliminar(c.requestContext(), id, &services.EliminarCeremoniaInput{DocumentoIdentidadSecretario: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, map[string]bool{"deleted": true})
}

func (c *CeremoniaController) Crear() {
	payload, err := decodeBody[services.CrearCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.Crear(c.requestContext(), payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusCreated, result)
}

func (c *CeremoniaController) Asignar() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.AsignarEstudiantesCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.AsignarEstudiantes(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) QuitarPrograma() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.QuitarProgramaCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.QuitarPrograma(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ListarFirmasDecano() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_decano"), 10, 64)
	result, err := c.Service.ListarFirmasDecano(c.requestContext(), id, &services.ListarCeremoniasInput{DocumentoIdentidadSecretario: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) FirmarMasivamenteDecano() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.FirmaMasivaCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.FirmarMasivamenteDecano(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeFirmaMasiva(result)
}

func (c *CeremoniaController) ListarFirmasSecretariaGeneral() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_firmante"), 10, 64)
	result, err := c.Service.ListarFirmasSecretariaGeneral(c.requestContext(), id, &services.ListarCeremoniasInput{DocumentoIdentidadFirmante: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) FirmarMasivamenteSecretariaGeneral() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.FirmaMasivaCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.FirmarMasivamenteSecretariaGeneral(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeFirmaMasiva(result)
}

func (c *CeremoniaController) ListarFirmasRector() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_firmante"), 10, 64)
	result, err := c.Service.ListarFirmasRector(c.requestContext(), id, &services.ListarCeremoniasInput{DocumentoIdentidadFirmante: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) FirmarMasivamenteRector() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.FirmaMasivaCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.FirmarMasivamenteRector(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeFirmaMasiva(result)
}

func (c *CeremoniaController) EstudiantesPrograma() {
	programaID, err := strconv.ParseInt(c.Ctx.Input.Param(":programa_id"), 10, 64)
	if err != nil || programaID <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_secretario"), 10, 64)
	result, err := c.Service.EstudiantesPrograma(c.requestContext(), &services.EstudiantesProgramaCeremoniaInput{PeriodoID: periodoQuery(c.GetString("periodo_id")), DocumentoIdentidadSecretario: documento, ProgramaAcademicoID: programaID})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) ListarFirmas() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	documento, _ := strconv.ParseInt(c.GetString("documento_identidad_secretario"), 10, 64)
	result, err := c.Service.ListarFirmas(c.requestContext(), id, &services.ListarCeremoniasInput{DocumentoIdentidadSecretario: documento})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *CeremoniaController) FirmarMasivamente() {
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
		return
	}
	payload, err := decodeBody[services.FirmaMasivaCeremoniaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, err)
		return
	}
	result, err := c.Service.FirmarMasivamente(c.requestContext(), id, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeFirmaMasiva(result)
}

func periodoQuery(raw string) int64 { id, _ := strconv.ParseInt(raw, 10, 64); return id }
