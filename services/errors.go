package services

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrNoActiveRole    = errors.New("no tiene un rol activo para firmar diplomas")
	ErrExternalService = errors.New("external service error")
)
