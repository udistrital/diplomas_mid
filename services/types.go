package services

type PersonaFirma struct {
	Nombre         string   `json:"nombre"`
	Cargo          string   `json:"cargo,omitempty"`
	Oficina        string   `json:"oficina,omitempty"`
	TipoID         string   `json:"tipoId,omitempty"`
	Identificacion string   `json:"identificacion,omitempty"`
	OrdenCampos    []string `json:"orden_campos,omitempty"`
}

type FirmaDiplomaInput struct {
	Nombre                     string                 `json:"nombre"`
	Descripcion                string                 `json:"descripcion"`
	DocumentoIdentidadFirmante int64                  `json:"documento_identidad_firmante"`
	Metadatos                  map[string]interface{} `json:"metadatos"`
	Firmantes                  []PersonaFirma         `json:"firmantes,omitempty"`
	Representantes             []PersonaFirma         `json:"representantes"`
	File                       string                 `json:"file"`
}

type firmaElectronicaRequest struct {
	RepositorioDocumental string                 `json:"repositorio_documental"`
	DocumentoID           int64                  `json:"documento_id"`
	Nombre                string                 `json:"nombre"`
	Descripcion           string                 `json:"descripcion"`
	Metadatos             map[string]interface{} `json:"metadatos"`
	Firmantes             []PersonaFirma         `json:"firmantes"`
	Representantes        []PersonaFirma         `json:"representantes"`
	File                  string                 `json:"file"`
}

type firmaElectronicaResponse struct {
	Status                string                 `json:"Status"`
	FirmaID               string                 `json:"firma_id"`
	RepositorioDocumental string                 `json:"repositorio_documental"`
	DocumentoID           int64                  `json:"documento_id"`
	UUIDDocumento         string                 `json:"uuid_documento"`
	HashSHA256            string                 `json:"hash_sha256"`
	CodigoAutenticidad    string                 `json:"codigo_autenticidad"`
	FirmaEncriptada       string                 `json:"firma_encriptada"`
	Llaves                map[string]interface{} `json:"llaves"`
	Firmantes             map[string]interface{} `json:"firmantes"`
	QRURLSegura           string                 `json:"qr_url_segura"`
	DynamoDB              map[string]interface{} `json:"dynamodb"`
	File                  string                 `json:"file"`
}

type actualizarUUIDDocumentoRequest struct {
	UUIDDocumento string `json:"uuid_documento"`
}

type documentoDigitalResponse struct {
	ID            int64  `json:"id"`
	UUIDDocumento string `json:"uuid_documento"`
}

type S3ObjectRef struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	VersionID string `json:"version_id,omitempty"`
	ETag      string `json:"etag,omitempty"`
	URI       string `json:"uri"`
}

type FirmanteActivo struct {
	Rol                string `json:"rol"`
	Orden              int    `json:"orden"`
	CargoID            int    `json:"cargo_id"`
	Cargo              string `json:"cargo"`
	Nombre             string `json:"nombre"`
	DocumentoIdentidad int64  `json:"documento_identidad"`
}

type SubirFirmaFirmanteInput struct {
	DocumentoIdentidadFirmante int64                  `json:"documento_identidad_firmante"`
	IdTipoDocumento            int64                  `json:"IdTipoDocumento,omitempty"`
	Nombre                     string                 `json:"nombre,omitempty"`
	Descripcion                string                 `json:"descripcion,omitempty"`
	Metadatos                  map[string]interface{} `json:"metadatos,omitempty"`
	File                       string                 `json:"file"`
}

type subirDocumentoFirmaRequest struct {
	IdTipoDocumento int64                  `json:"IdTipoDocumento"`
	Nombre          string                 `json:"nombre"`
	Metadatos       map[string]interface{} `json:"metadatos"`
	Descripcion     string                 `json:"descripcion"`
	File            string                 `json:"file"`
}

type registrarFirmaFirmanteRequest struct {
	DocumentoIdentidad int64  `json:"documento_identidad"`
	EnlaceFirma        string `json:"enlace_firma"`
	Activo             bool   `json:"activo"`
}

type FirmaFirmanteCRUDResponse struct {
	ID                 int64  `json:"id"`
	DocumentoIdentidad int64  `json:"documento_identidad"`
	EnlaceFirma        string `json:"enlace_firma"`
	Activo             bool   `json:"activo"`
}

type SubirFirmaFirmanteResult struct {
	Firmante      FirmanteActivo            `json:"firmante"`
	EnlaceFirma   string                    `json:"enlace_firma"`
	Documento     map[string]interface{}    `json:"documento"`
	FirmaFirmante FirmaFirmanteCRUDResponse `json:"firma_firmante"`
}

type FirmaDiplomaResult struct {
	FirmaID               string                   `json:"firma_id"`
	RepositorioDocumental string                   `json:"repositorio_documental"`
	DocumentoID           int64                    `json:"documento_id"`
	UUIDDocumento         string                   `json:"uuid_documento"`
	HashSHA256            string                   `json:"hash_sha256"`
	CodigoAutenticidad    string                   `json:"codigo_autenticidad"`
	QRURLSegura           string                   `json:"qr_url_segura"`
	Firmante              FirmanteActivo           `json:"firmante"`
	S3                    S3ObjectRef              `json:"s3"`
	DynamoDB              map[string]interface{}   `json:"dynamodb"`
	DocumentoDigital      documentoDigitalResponse `json:"documento_digital"`
}
