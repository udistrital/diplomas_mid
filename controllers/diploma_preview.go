package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/udistrital/diplomas_mid/services"
)

type DiplomaPreviewController struct {
	APIController
	Service services.DiplomaPreviewService
}

func (c *DiplomaPreviewController) URLMapping() {}

func (c *DiplomaPreviewController) Generar() {
	documentoID, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid documento id"))
		return
	}

	result, err := c.Service.Generar(c.requestContext(), documentoID)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusOK, result)
}

// Preparar reserva el registro antes de mostrar la primera vista previa del secretario.
func (c *DiplomaPreviewController) Preparar() {
	documentoID, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || documentoID <= 0 {
		c.writeError(http.StatusBadRequest, errors.New("invalid documento id"))
		return
	}
	payload, err := decodeBody[struct {
		DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
	}](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid request body"))
		return
	}
	result, err := c.Service.Preparar(c.requestContext(), documentoID, payload.DocumentoIdentidadSecretario)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}
