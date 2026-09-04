package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/udistrital/diplomas_mid/services"
)

type FirmanteController struct {
	APIController
	Service services.FirmanteService
}

func (c *FirmanteController) URLMapping() {}

func (c *FirmanteController) RolActivo() {
	documento, err := strconv.ParseInt(c.GetString("documento_identidad"), 10, 64)
	if err != nil {
		c.writeError(http.StatusBadRequest, errors.New("documento_identidad is required"))
		return
	}

	result, err := c.Service.ConsultarRolActivo(c.Ctx.Request.Context(), documento, time.Now())
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}

	c.writeJSON(http.StatusOK, result)
}
