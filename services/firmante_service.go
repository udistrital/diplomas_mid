package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/udistrital/utils_oas/v2/request"
)

type FirmanteService struct{}

type rolFirma struct {
	Codigo string
	Orden  int
}

type supervisorContratoResponse struct {
	ID                    int       `json:"Id"`
	Nombre                string    `json:"Nombre"`
	Documento             int64     `json:"Documento"`
	Cargo                 string    `json:"Cargo"`
	SedeSupervisor        string    `json:"SedeSupervisor"`
	DependenciaSupervisor string    `json:"DependenciaSupervisor"`
	Tipo                  int       `json:"Tipo"`
	Estado                bool      `json:"Estado"`
	DigitoVerificacion    int       `json:"DigitoVerificacion"`
	FechaInicio           time.Time `json:"FechaInicio"`
	FechaFin              time.Time `json:"FechaFin"`
	CargoID               struct {
		ID    int    `json:"Id"`
		Cargo string `json:"Cargo"`
	} `json:"CargoId"`
}

var rolesFirmaPermitidos = map[int]rolFirma{
	158: {Codigo: "secretario_academico", Orden: 1},
	159: {Codigo: "secretario_academico", Orden: 1},
	160: {Codigo: "secretario_academico", Orden: 1},
	343: {Codigo: "secretario_academico", Orden: 1},
	275: {Codigo: "secretario_academico", Orden: 1},
	161: {Codigo: "secretario_academico", Orden: 1},
	270: {Codigo: "decano", Orden: 2},
	46:  {Codigo: "decano", Orden: 2},
	296: {Codigo: "decano", Orden: 2},
	44:  {Codigo: "decano", Orden: 2},
	42:  {Codigo: "decano", Orden: 2},
	297: {Codigo: "decano", Orden: 2},
	45:  {Codigo: "decano", Orden: 2},
	106: {Codigo: "secretaria_general", Orden: 3},
	100: {Codigo: "rector", Orden: 4},
}

func (s FirmanteService) ConsultarRolActivo(ctx context.Context, documentoIdentidad int64, fecha time.Time) (*FirmanteActivo, error) {
	if documentoIdentidad <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}

	fechaConsulta := fecha.Format("2006-01-02")
	query := fmt.Sprintf(
		"Documento:%d,FechaInicio__lte:%s,FechaFin__gte:%s",
		documentoIdentidad,
		fechaConsulta,
		fechaConsulta,
	)
	endpoint := fmt.Sprintf(
		"%s/v1/supervisor_contrato?query=%s",
		administrativaAmazonAPIURL(),
		url.QueryEscape(query),
	)

	var supervisores []supervisorContratoResponse
	status, err := request.GetWithContext(ctx, endpoint, &supervisores)
	if err != nil {
		return nil, fmt.Errorf("administrativa_amazon_api supervisor_contrato status %d: %w", status, err)
	}

	for _, supervisor := range supervisores {
		cargoID := supervisor.CargoID.ID
		rol, ok := rolesFirmaPermitidos[cargoID]
		if !ok {
			continue
		}
		cargo := supervisor.Cargo
		if cargo == "" {
			cargo = supervisor.CargoID.Cargo
		}
		firmante := &FirmanteActivo{
			Rol:                rol.Codigo,
			Orden:              rol.Orden,
			CargoID:            cargoID,
			Cargo:              cargo,
			Nombre:             supervisor.Nombre,
			DocumentoIdentidad: supervisor.Documento,
			Dependencia:        supervisor.DependenciaSupervisor,
		}

		firma, err := s.consultarFirmaActiva(ctx, supervisor.Documento)
		if err != nil {
			return nil, fmt.Errorf("%w: consultar rúbrica activa: %w", ErrExternalService, err)
		}
		if firma != nil {
			firmante.EnlaceFirma = firma.EnlaceFirma
			firmante.FirmaFirmante = firma
			if firmaImagen, err := consultarFirmaGraficaFirmante(ctx, supervisor.Documento); err == nil {
				firmante.FirmaImagen = firmaImagen
			}
		}

		return firmante, nil
	}

	return nil, ErrNoActiveRole
}

func (s FirmanteService) consultarFirmaActiva(ctx context.Context, documentoIdentidad int64) (*FirmaFirmanteCRUDResponse, error) {
	query := fmt.Sprintf("documento_identidad:%d,activo:true", documentoIdentidad)
	endpoint := fmt.Sprintf("%s/firma_firmante/?query=%s", diplomasCrudURL(), url.QueryEscape(query))

	var firmas []FirmaFirmanteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &firmas)
	if status == 404 {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("diplomas_crud firma_firmante status %d: %w", status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("diplomas_crud firma_firmante returned status %d", status)
	}
	if len(firmas) == 0 {
		return nil, nil
	}
	if firmas[0].EnlaceFirma == "" {
		return nil, errors.New("firma_firmante sin enlace_firma")
	}
	return &firmas[0], nil
}
