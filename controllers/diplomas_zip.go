package controllers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/udistrital/diplomas_mid/services"
)

type DiplomasZIPController struct {
	APIController
	Service services.DiplomasZIPService
}

func (c *DiplomasZIPController) Opciones() {
	token := c.Ctx.Request.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Bearer ") {
		c.writeError(http.StatusUnauthorized, services.ErrNoActiveRole)
		return
	}
	ctx := context.WithValue(c.Ctx.Request.Context(), "Authorization", token)
	result, err := c.Service.Catalogo(ctx)
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.writeJSON(http.StatusOK, result)
}

func (c *DiplomasZIPController) Descargar() {
	token := c.Ctx.Request.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Bearer ") {
		c.writeError(http.StatusUnauthorized, services.ErrNoActiveRole)
		return
	}
	ceremoniaID, _ := strconv.ParseInt(c.GetString("ceremonia_id"), 10, 64)
	facultadID, _ := strconv.ParseInt(c.GetString("facultad_id"), 10, 64)
	incluirReporte := false
	if raw := c.GetString("incluir_reporte"); raw != "" {
		var err error
		incluirReporte, err = strconv.ParseBool(raw)
		if err != nil {
			c.writeError(http.StatusBadRequest, services.ErrInvalidInput)
			return
		}
	}
	ctx := context.WithValue(c.Ctx.Request.Context(), "Authorization", token)
	content, err := c.Service.Generar(ctx, services.DiplomasZIPInput{CeremoniaID: ceremoniaID, FacultadID: facultadID, IncluirReporte: incluirReporte})
	if err != nil {
		c.writeError(mapServiceError(err), err)
		return
	}
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\"diplomas_registro_control.zip\"")
	c.Ctx.Output.Header("Cache-Control", "no-store")
	c.Ctx.Output.SetStatus(http.StatusOK)
	_, _ = c.Ctx.ResponseWriter.Write(content)
}
