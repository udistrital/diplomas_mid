package controllers

import (
	"errors"
	"net/http"

	"github.com/udistrital/diplomas_mid/services"
)

type FirmaFirmanteController struct {
	APIController
	Service services.FirmaFirmanteService
}

func (c *FirmaFirmanteController) URLMapping() {}

func (c *FirmaFirmanteController) SubirFirma() {
	payload, err := decodeBody[services.SubirFirmaFirmanteInput](&c.Controller)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	result, err := c.Service.SubirFirma(c.Ctx.Request.Context(), payload)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusCreated, result)
}
