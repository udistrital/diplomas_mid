package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/udistrital/utils_oas/request"
)

type FirmaDiplomaService struct {
	S3 S3Service
}

func (s FirmaDiplomaService) Firmar(ctx context.Context, documentoID int64, input *FirmaDiplomaInput) (*FirmaDiplomaResult, error) {
	if documentoID <= 0 {
		return nil, fmt.Errorf("%w: documento id is required", ErrInvalidInput)
	}
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.File == "" {
		return nil, fmt.Errorf("%w: file is required", ErrInvalidInput)
	}
	if input.DocumentoIdentidadFirmante <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}

	firmanteActivo, err := FirmanteService{}.ConsultarRolActivo(ctx, input.DocumentoIdentidadFirmante, time.Now())
	if err != nil {
		return nil, err
	}
	firmantes := []PersonaFirma{{
		Nombre:         firmanteActivo.Nombre,
		Cargo:          firmanteActivo.Cargo,
		TipoID:         "CC",
		Identificacion: strconv.FormatInt(firmanteActivo.DocumentoIdentidad, 10),
	}}
	metadatos := cloneMetadatos(input.Metadatos)
	metadatos["rol_firmante"] = firmanteActivo.Rol
	metadatos["orden_firmante"] = firmanteActivo.Orden
	metadatos["cargo_id_firmante"] = firmanteActivo.CargoID
	metadatos["documento_identidad_firmante"] = firmanteActivo.DocumentoIdentidad

	firmaPayload := []firmaElectronicaRequest{{
		RepositorioDocumental: "diplomas",
		DocumentoID:           documentoID,
		Nombre:                input.Nombre,
		Descripcion:           input.Descripcion,
		Metadatos:             metadatos,
		Firmantes:             firmantes,
		Representantes:        input.Representantes,
		File:                  input.File,
	}}

	var firmaResponse firmaElectronicaResponse
	status, err := request.PostWithContext(ctx, firmaElectronicaMidURL()+"/v2/firma_electronica", firmaPayload, &firmaResponse)
	if err != nil {
		return nil, fmt.Errorf("firma_electronica_mid status %d: %w", status, err)
	}
	if firmaResponse.UUIDDocumento == "" || firmaResponse.File == "" {
		return nil, fmt.Errorf("firma_electronica_mid response missing uuid_documento or file")
	}

	s3Service := s.S3
	s3Ref, err := s3Service.PutDiploma(ctx, firmaResponse.UUIDDocumento, firmaResponse.File)
	if err != nil {
		return nil, err
	}

	var documentoResponse documentoDigitalResponse
	updatePayload := actualizarUUIDDocumentoRequest{UUIDDocumento: firmaResponse.UUIDDocumento}
	status, err = request.PutWithContext(
		ctx,
		fmt.Sprintf("%s/documento_digital/%d/uuid", diplomasCrudURL(), documentoID),
		updatePayload,
		&documentoResponse,
	)
	if err != nil {
		return nil, fmt.Errorf("diplomas_crud actualizar uuid status %d: %w", status, err)
	}

	return &FirmaDiplomaResult{
		FirmaID:               firmaResponse.FirmaID,
		RepositorioDocumental: firmaResponse.RepositorioDocumental,
		DocumentoID:           documentoID,
		UUIDDocumento:         firmaResponse.UUIDDocumento,
		HashSHA256:            firmaResponse.HashSHA256,
		CodigoAutenticidad:    firmaResponse.CodigoAutenticidad,
		QRURLSegura:           firmaResponse.QRURLSegura,
		Firmante:              *firmanteActivo,
		S3:                    s3Ref,
		DynamoDB:              firmaResponse.DynamoDB,
		DocumentoDigital:      documentoResponse,
	}, nil
}

func cloneMetadatos(input map[string]interface{}) map[string]interface{} {
	metadatos := make(map[string]interface{}, len(input)+4)
	for key, value := range input {
		metadatos[key] = value
	}
	return metadatos
}
