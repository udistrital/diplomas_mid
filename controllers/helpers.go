package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/udistrital/diplomas_mid/services"
)

type APIController struct {
	beego.Controller
}

type apiError struct {
	Message string `json:"message"`
}

func (c *APIController) writeJSON(status int, payload interface{}) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = payload
	c.ServeJSON()
}

func (c *APIController) writeError(status int, err error) {
	message := "internal server error"
	if err != nil && status < http.StatusInternalServerError {
		message = err.Error()
	}
	if err != nil && status >= http.StatusInternalServerError {
		logs.Error(err.Error())
	}
	c.writeJSON(status, apiError{Message: message})
}

func decodeBody[T any](controller *beego.Controller) (*T, error) {
	var payload T
	if err := json.NewDecoder(controller.Ctx.Request.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func mapServiceError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, services.ErrInvalidInput) {
		return http.StatusBadRequest
	}
	if errors.Is(err, services.ErrNoActiveRole) {
		return http.StatusForbidden
	}
	if errors.Is(err, services.ErrExternalService) {
		return http.StatusBadGateway
	}
	return http.StatusInternalServerError
}
