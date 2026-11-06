package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/udistrital/diplomas_mid/services"
)

type FirmaDiplomaController struct {
	APIController
	Service services.FirmaDiplomaService
}

func (c *FirmaDiplomaController) URLMapping() {}

func (c *FirmaDiplomaController) Firmar() {
	documentoID, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid documento id"))
		return
	}

	payload, err := decodeBody[services.FirmaDiplomaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	result, err := c.Service.Firmar(c.requestContext(), documentoID, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusOK, result)
}

func (c *FirmaDiplomaController) Anular() {
	documentoID, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid documento id"))
		return
	}

	payload, err := decodeBody[services.AnularFirmaDiplomaInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	ctx := c.Ctx.Request.Context()
	if token := c.Ctx.Request.Header.Get("Authorization"); token != "" {
		ctx = context.WithValue(ctx, "Authorization", token)
	}
	result, err := c.Service.Anular(ctx, documentoID, payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusOK, result)
}
