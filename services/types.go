package services

import "time"

type PersonaFirma struct {
	Nombre         string   `json:"nombre"`
	Cargo          string   `json:"cargo,omitempty"`
	Oficina        string   `json:"oficina,omitempty"`
	TipoID         string   `json:"tipoId,omitempty"`
	Identificacion string   `json:"identificacion,omitempty"`
	FirmaURL       string   `json:"firma_url,omitempty"`
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

type AnularFirmaDiplomaInput struct {
	DocumentoIdentidadFirmante int64 `json:"documento_identidad_firmante"`
}

type AnularFirmaDiplomaResult struct {
	DocumentoID int64 `json:"documento_id"`
	Orden       int   `json:"orden"`
	Anuladas    int   `json:"anuladas"`
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

type crearDocumentoDigitalRequest struct {
	TipoDocumentoDigitalID    int64  `json:"tipo_documento_digital_id"`
	EstadoDocumentoID         int64  `json:"estado_documento_id"`
	CodigoEstudiante          int64  `json:"codigo_estudiante"`
	FacultadID                int64  `json:"facultad_id"`
	ProgramaAcademicoID       int64  `json:"programa_academico_id"`
	PeriodoID                 int64  `json:"periodo_id"`
	Vigencia                  int    `json:"vigencia"`
	NombreEstudiante          string `json:"nombre_estudiante,omitempty"`
	TipoDocumentoEstudiante   string `json:"tipo_documento_estudiante,omitempty"`
	NumeroDocumentoEstudiante string `json:"numero_documento_estudiante,omitempty"`
	MunicipioExpedicion       string `json:"municipio_expedicion,omitempty"`
	TituloOtorgado            string `json:"titulo_otorgado,omitempty"`
	Activo                    bool   `json:"activo"`
}

type ParametrizacionRegistroInput struct {
	DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
	Vigencia                     int   `json:"vigencia"`
	LibroInicial                 int   `json:"libro_inicial"`
	FolioInicial                 int   `json:"folio_inicial"`
	RegistroInicial              int   `json:"registro_inicial"`
	ConsecutivoFacultadInicial   int   `json:"consecutivo_facultad_inicial"`
	FoliosPorLibro               int   `json:"folios_por_libro"`
	RegistrosPorFolio            int   `json:"registros_por_folio"`
}

type ParametrizacionRegistroResponse struct {
	ID                         int64 `json:"id"`
	FacultadID                 int64 `json:"facultad_id"`
	Vigencia                   int   `json:"vigencia"`
	LibroInicial               int   `json:"libro_inicial"`
	FolioInicial               int   `json:"folio_inicial"`
	RegistroInicial            int   `json:"registro_inicial"`
	ConsecutivoFacultadInicial int   `json:"consecutivo_facultad_inicial"`
	FoliosPorLibro             int   `json:"folios_por_libro"`
	RegistrosPorFolio          int   `json:"registros_por_folio"`
	Activo                     bool  `json:"activo"`
}

type documentoDigitalResponse struct {
	ID                        int64   `json:"id"`
	TipoDocumentoDigitalID    int64   `json:"tipo_documento_digital_id"`
	EstadoDocumentoID         int64   `json:"estado_documento_id"`
	CodigoEstudiante          int64   `json:"codigo_estudiante"`
	FacultadID                *int64  `json:"facultad_id,omitempty"`
	ProgramaAcademicoID       *int64  `json:"programa_academico_id,omitempty"`
	PeriodoID                 *int64  `json:"periodo_id,omitempty"`
	Vigencia                  *int    `json:"vigencia,omitempty"`
	NombreEstudiante          string  `json:"nombre_estudiante,omitempty"`
	TipoDocumentoEstudiante   string  `json:"tipo_documento_estudiante,omitempty"`
	NumeroDocumentoEstudiante string  `json:"numero_documento_estudiante,omitempty"`
	MunicipioExpedicion       string  `json:"municipio_expedicion,omitempty"`
	TituloOtorgado            string  `json:"titulo_otorgado,omitempty"`
	UUIDDocumento             *string `json:"uuid_documento,omitempty"`
	Activo                    bool    `json:"activo"`
}

type crearDiplomaDigitalRequest struct {
	FacultadID                int64     `json:"facultad_id"`
	Vigencia                  int       `json:"vigencia"`
	FechaGrado                time.Time `json:"fecha_grado"`
	NombreEstudiante          string    `json:"nombre_estudiante"`
	TipoDocumentoEstudiante   string    `json:"tipo_documento_estudiante"`
	NumeroDocumentoEstudiante string    `json:"numero_documento_estudiante"`
	MunicipioExpedicion       string    `json:"municipio_expedicion,omitempty"`
	TituloOtorgado            string    `json:"titulo_otorgado"`
	EstadoDocumentoCreadoID   int64     `json:"estado_documento_creado_id"`
}

type diplomaDigitalResponse struct {
	ID                        int64                     `json:"id"`
	DocumentoDigital          *documentoDigitalResponse `json:"documento_digital,omitempty"`
	FacultadID                int64                     `json:"facultad_id"`
	Vigencia                  int                       `json:"vigencia"`
	FechaGrado                time.Time                 `json:"fecha_grado"`
	NombreEstudiante          string                    `json:"nombre_estudiante"`
	TipoDocumentoEstudiante   string                    `json:"tipo_documento_estudiante"`
	NumeroDocumentoEstudiante string                    `json:"numero_documento_estudiante"`
	MunicipioExpedicion       string                    `json:"municipio_expedicion,omitempty"`
	TituloOtorgado            string                    `json:"titulo_otorgado"`
	ConsecutivoDiploma        int64                     `json:"consecutivo_diploma"`
	ConsecutivoFacultad       *int                      `json:"consecutivo_facultad,omitempty"`
	RegistroGrado             *registroGradoResponse    `json:"registro_grado,omitempty"`
	Activo                    bool                      `json:"activo"`
}

type libroRegistroResponse struct {
	ID          int64 `json:"id"`
	NumeroLibro int   `json:"numero_libro"`
}

type folioRegistroResponse struct {
	ID            int64                  `json:"id"`
	LibroRegistro *libroRegistroResponse `json:"libro_registro,omitempty"`
	NumeroFolio   int                    `json:"numero_folio"`
}

type registroGradoResponse struct {
	ID             int64                  `json:"id"`
	FolioRegistro  *folioRegistroResponse `json:"folio_registro,omitempty"`
	NumeroRegistro int                    `json:"numero_registro"`
}

type S3ObjectRef struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	VersionID string `json:"version_id,omitempty"`
	ETag      string `json:"etag,omitempty"`
	URI       string `json:"uri"`
}

type FirmanteActivo struct {
	Rol                string                     `json:"rol"`
	Orden              int                        `json:"orden"`
	CargoID            int                        `json:"cargo_id"`
	Cargo              string                     `json:"cargo"`
	Nombre             string                     `json:"nombre"`
	DocumentoIdentidad int64                      `json:"documento_identidad"`
	Dependencia        string                     `json:"dependencia,omitempty"`
	FacultadID         int64                      `json:"facultad_id,omitempty"`
	EnlaceFirma        string                     `json:"enlace_firma,omitempty"`
	FirmaImagen        string                     `json:"firma_imagen,omitempty"`
	FirmaFirmante      *FirmaFirmanteCRUDResponse `json:"firma_firmante,omitempty"`
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
	Preview               bool                     `json:"preview"`
	FirmaID               string                   `json:"firma_id"`
	RepositorioDocumental string                   `json:"repositorio_documental"`
	DocumentoID           int64                    `json:"documento_id"`
	UUIDDocumento         string                   `json:"uuid_documento"`
	HashSHA256            string                   `json:"hash_sha256"`
	CodigoAutenticidad    string                   `json:"codigo_autenticidad"`
	QRURLSegura           string                   `json:"qr_url_segura"`
	Firmante              FirmanteActivo           `json:"firmante"`
	S3                    *S3ObjectRef             `json:"s3,omitempty"`
	DynamoDB              map[string]interface{}   `json:"dynamodb"`
	DocumentoDigital      documentoDigitalResponse `json:"documento_digital"`
	FirmaDocumento        *firmaDocumentoResponse  `json:"firma_documento,omitempty"`
	File                  string                   `json:"file,omitempty"`
}

type registrarFirmaDocumentoRequest struct {
	FirmaFirmanteID    *int64 `json:"firma_firmante_id,omitempty"`
	RolFirmanteID      int64  `json:"rol_firmante_id"`
	DocumentoIdentidad int64  `json:"documento_identidad"`
	EstadoFirmadoID    *int64 `json:"estado_firmado_id,omitempty"`
	Observacion        string `json:"observacion,omitempty"`
}

type firmaDocumentoResponse struct {
	ID                 int64                      `json:"id"`
	DocumentoDigital   *documentoDigitalResponse  `json:"documento_digital,omitempty"`
	FirmaFirmante      *FirmaFirmanteCRUDResponse `json:"firma_firmante,omitempty"`
	RolFirmanteID      int64                      `json:"rol_firmante_id"`
	DocumentoIdentidad int64                      `json:"documento_identidad"`
	Activo             bool                       `json:"activo"`
}

type ListarCeremoniasInput struct {
	PeriodoID                    int64 `json:"periodo_id"`
	DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
	DocumentoIdentidadFirmante   int64 `json:"documento_identidad_firmante,omitempty"`
}

type ActualizarCeremoniaInput struct {
	PeriodoID                     int64  `json:"periodo_id"`
	FechaCeremonia                string `json:"fecha_ceremonia"`
	Lugar                         string `json:"lugar"`
	CantidadEstudiantesProgramada int    `json:"cantidad_estudiantes_programada"`
	Observacion                   string `json:"observacion,omitempty"`
	DocumentoIdentidadSecretario  int64  `json:"documento_identidad_secretario"`
}

type EliminarCeremoniaInput struct {
	DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
}

type CeremoniaResumen struct {
	PeriodoID                     int64  `json:"periodo_id"`
	ID                            int64  `json:"id"`
	RelacionID                    int64  `json:"relacion_id"`
	FacultadID                    int64  `json:"facultad_id"`
	FechaCeremonia                string `json:"fecha_ceremonia"`
	Lugar                         string `json:"lugar"`
	CantidadEstudiantesProgramada int    `json:"cantidad_estudiantes_programada"`
	CuposAsignados                int    `json:"cupos_asignados"`
	CuposDisponibles              int    `json:"cupos_disponibles"`
	EstadoFlujo                   string `json:"estado_flujo"`
	PuedeEditar                   bool   `json:"puede_editar"`
	PuedeEliminar                 bool   `json:"puede_eliminar"`
	MotivoBloqueo                 string `json:"motivo_bloqueo,omitempty"`
	Observacion                   string `json:"observacion,omitempty"`
	Activo                        bool   `json:"activo"`
}

type ProgramaAprobadoResumen struct {
	PeriodoID           int64  `json:"periodo_id"`
	ProgramaAcademicoID int64  `json:"programa_academico_id"`
	Nombre              string `json:"nombre"`
	Cantidad            int    `json:"cantidad"`
	CeremoniaID         int64  `json:"ceremonia_id,omitempty"`
	RelacionID          int64  `json:"relacion_id,omitempty"`
	Lugar               string `json:"lugar,omitempty"`
	FechaCeremonia      string `json:"fecha_ceremonia,omitempty"`
	EstadoFlujo         string `json:"estado_flujo,omitempty"`
	PuedeReasignar      bool   `json:"puede_reasignar"`
	MotivoBloqueo       string `json:"motivo_bloqueo,omitempty"`
}

type TableroCeremoniasResult struct {
	FacultadID          int64                     `json:"facultad_id"`
	TotalCeremonias     int                       `json:"total_ceremonias"`
	TotalCupos          int                       `json:"total_cupos"`
	CuposAsignados      int                       `json:"cupos_asignados"`
	CuposDisponibles    int                       `json:"cupos_disponibles"`
	Ceremonias          []CeremoniaResumen        `json:"ceremonias"`
	AprobadosTotal      int                       `json:"aprobados_total"`
	AprobadosProgramas  []ProgramaAprobadoResumen `json:"aprobados_programas"`
	PendientesProgramas []ProgramaAprobadoResumen `json:"pendientes_programas"`
	AsignadosProgramas  []ProgramaAprobadoResumen `json:"asignados_programas"`
}

type CeremoniaFirmaEstudianteResumen struct {
	AsignacionID              int64  `json:"asignacion_id"`
	DocumentoDigitalID        int64  `json:"documento_digital_id"`
	CodigoEstudiante          int64  `json:"codigo_estudiante"`
	ProgramaAcademicoID       int64  `json:"programa_academico_id,omitempty"`
	Programa                  string `json:"programa,omitempty"`
	NombreEstudiante          string `json:"nombre_estudiante,omitempty"`
	TipoDocumentoEstudiante   string `json:"tipo_documento_estudiante,omitempty"`
	NumeroDocumentoEstudiante string `json:"numero_documento_estudiante,omitempty"`
	TituloOtorgado            string `json:"titulo_otorgado,omitempty"`
	EstadoDocumentoID         int64  `json:"estado_documento_id"`
	FirmaSecretario           bool   `json:"firma_secretario"`
	FirmaDecano               bool   `json:"firma_decano"`
	FirmaSecretariaGeneral    bool   `json:"firma_secretaria_general"`
	FirmaRector               bool   `json:"firma_rector"`
	PuedeFirmar               bool   `json:"puede_firmar"`
	MotivoBloqueo             string `json:"motivo_bloqueo,omitempty"`
}

type CeremoniaFirmasResult struct {
	Ceremonia   CeremoniaResumen                  `json:"ceremonia"`
	Total       int                               `json:"total"`
	Firmados    int                               `json:"firmados"`
	Pendientes  int                               `json:"pendientes"`
	Estudiantes []CeremoniaFirmaEstudianteResumen `json:"estudiantes"`
}

type FirmaMasivaCeremoniaInput struct {
	DocumentoIdentidadSecretario int64   `json:"documento_identidad_secretario"`
	DocumentoIdentidadFirmante   int64   `json:"documento_identidad_firmante,omitempty"`
	DocumentoDigitalIDs          []int64 `json:"documento_digital_ids,omitempty"`
}

type FirmaMasivaCeremoniaItem struct {
	DocumentoDigitalID int64               `json:"documento_digital_id"`
	Firmado            bool                `json:"firmado"`
	Error              string              `json:"error,omitempty"`
	Result             *FirmaDiplomaResult `json:"result,omitempty"`
}

type FirmaMasivaCeremoniaResult struct {
	CeremoniaID int64                      `json:"ceremonia_id"`
	Total       int                        `json:"total"`
	Firmados    int                        `json:"firmados"`
	Omitidos    int                        `json:"omitidos"`
	Errores     int                        `json:"errores"`
	Message     string                     `json:"message,omitempty"`
	Resultados  []FirmaMasivaCeremoniaItem `json:"resultados"`
}

type AsignarEstudiantesCeremoniaInput struct {
	PeriodoID                    int64                          `json:"periodo_id"`
	DocumentoIdentidadSecretario int64                          `json:"documento_identidad_secretario"`
	Programas                    []AsignarProgramaCeremoniaItem `json:"programas"`
}

type AsignarProgramaCeremoniaItem struct {
	ProgramaAcademicoID int64 `json:"programa_academico_id"`
	Cantidad            int   `json:"cantidad,omitempty"`
}

type AsignarEstudiantesCeremoniaResult struct {
	CeremoniaID int64 `json:"ceremonia_id"`
	Asignados   int   `json:"asignados"`
	Omitidos    int   `json:"omitidos"`
	Errores     int   `json:"errores"`
}

type ceremoniaEstudianteCRUDRequest struct {
	CeremoniaFacultadPrograma ceremoniaFacultadProgramaRef `json:"ceremonia_facultad_programa"`
	DocumentoDigital          documentoDigitalRef          `json:"documento_digital"`
	FechaCreacion             string                       `json:"fecha_creacion,omitempty"`
	Activo                    bool                         `json:"activo"`
}

type ceremoniaFacultadProgramaRef struct {
	ID int64 `json:"id"`
}

type documentoDigitalRef struct {
	ID int64 `json:"id"`
}

type ceremoniaEstudianteCRUDResponse struct {
	ID                        int64                                  `json:"id"`
	CeremoniaFacultadPrograma *ceremoniaFacultadProgramaCRUDResponse `json:"ceremonia_facultad_programa,omitempty"`
	DocumentoDigital          *documentoDigitalResponse              `json:"documento_digital,omitempty"`
	FechaCreacion             string                                 `json:"fecha_creacion,omitempty"`
	Activo                    bool                                   `json:"activo"`
}

type QuitarProgramaCeremoniaInput struct {
	DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
	ProgramaAcademicoID          int64 `json:"programa_academico_id"`
}

type EstudiantesProgramaCeremoniaInput struct {
	PeriodoID                    int64 `json:"periodo_id"`
	DocumentoIdentidadSecretario int64 `json:"documento_identidad_secretario"`
	ProgramaAcademicoID          int64 `json:"programa_academico_id"`
}

type EstudiantesProgramaCeremoniaResult struct {
	ProgramaAcademicoID int64                     `json:"programa_academico_id"`
	Nombre              string                    `json:"nombre"`
	Total               int                       `json:"total"`
	Estudiantes         []EstudianteAprobadoGrado `json:"estudiantes"`
}

type CrearCeremoniaInput struct {
	PeriodoID                     int64  `json:"periodo_id"`
	FechaCeremonia                string `json:"fecha_ceremonia"`
	Lugar                         string `json:"lugar"`
	CantidadEstudiantesProgramada int    `json:"cantidad_estudiantes_programada"`
	Observacion                   string `json:"observacion,omitempty"`
	DocumentoIdentidadSecretario  int64  `json:"documento_identidad_secretario"`
}

type CrearCeremoniaResult struct {
	Ceremonia           ceremoniaGradoCRUDResponse              `json:"ceremonia"`
	Facultad            ceremoniaFacultadProgramaCRUDResponse   `json:"facultad"`
	SecretarioAcademico FirmanteActivo                          `json:"secretario_academico"`
	FacultadesProgramas []ceremoniaFacultadProgramaCRUDResponse `json:"facultades_programas,omitempty"`
}

type ceremoniaGradoCRUDRequest struct {
	PeriodoID                     int64  `json:"periodo_id"`
	FechaCeremonia                string `json:"fecha_ceremonia"`
	Lugar                         string `json:"lugar"`
	CantidadEstudiantesProgramada int    `json:"cantidad_estudiantes_programada"`
	Observacion                   string `json:"observacion,omitempty"`
	Activo                        bool   `json:"activo"`
}

type ceremoniaGradoCRUDResponse struct {
	PeriodoID                     int64  `json:"periodo_id"`
	ID                            int64  `json:"id"`
	FechaCeremonia                string `json:"fecha_ceremonia"`
	Lugar                         string `json:"lugar"`
	CantidadEstudiantesProgramada int    `json:"cantidad_estudiantes_programada"`
	Observacion                   string `json:"observacion,omitempty"`
	Activo                        bool   `json:"activo"`
}

type ceremoniaRef struct {
	ID int64 `json:"id"`
}

type ceremoniaFacultadProgramaCRUDRequest struct {
	CeremoniaGrado                ceremoniaRef `json:"ceremonia_grado"`
	FacultadID                    int64        `json:"facultad_id"`
	ProgramaAcademicoID           int64        `json:"programa_academico_id"`
	CantidadEstudiantesProgramada *int         `json:"cantidad_estudiantes_programada,omitempty"`
	Activo                        bool         `json:"activo"`
}

type ceremoniaFacultadProgramaCRUDResponse struct {
	ID                            int64                       `json:"id"`
	CeremoniaGrado                *ceremoniaGradoCRUDResponse `json:"ceremonia_grado,omitempty"`
	FacultadID                    int64                       `json:"facultad_id"`
	ProgramaAcademicoID           int64                       `json:"programa_academico_id"`
	CantidadEstudiantesProgramada *int                        `json:"cantidad_estudiantes_programada,omitempty"`
	Activo                        bool                        `json:"activo"`
}
