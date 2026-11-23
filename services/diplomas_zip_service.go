package services

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type DiplomasZIPService struct{}
type DiplomasZIPInput struct {
	CeremoniaID int64
	FacultadID  int64
}
type RegistroControlCatalogo struct {
	Facultades []RegistroControlFacultad `json:"facultades"`
}
type RegistroControlFacultad struct {
	ID         int64                      `json:"id"`
	Nombre     string                     `json:"nombre"`
	Ceremonias []RegistroControlCeremonia `json:"ceremonias"`
}
type RegistroControlCeremonia struct {
	ID                  int64  `json:"id"`
	Fecha               string `json:"fecha"`
	Lugar               string `json:"lugar"`
	DiplomasDisponibles int    `json:"diplomas_disponibles"`
}
type diplomaZIPItem struct {
	FacultadID, CeremoniaID, Codigo int64
	FechaCeremonia, UUID            string
	Lugar                           string
	Documento                       *documentoDigitalResponse
}

func (DiplomasZIPService) Catalogo(ctx context.Context) (*RegistroControlCatalogo, error) {
	archivosS3, err := listarDiplomasOriginalesS3(ctx)
	if err != nil {
		return nil, err
	}
	relaciones, err := (CeremoniaService{}).listarRelacionesActivas(ctx)
	if err != nil {
		return nil, err
	}
	type clave struct{ facultad, ceremonia int64 }
	datos := map[clave]RegistroControlCeremonia{}
	nombres := map[int64]string{}
	documentos := map[clave]map[int64]struct{}{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo {
			continue
		}
		k := clave{relacion.FacultadID, relacion.CeremoniaGrado.ID}
		asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
		if err != nil {
			return nil, err
		}
		if documentos[k] == nil {
			documentos[k] = map[int64]struct{}{}
		}
		for _, asignacion := range asignaciones {
			doc := asignacion.DocumentoDigital
			if asignacion.Activo && doc != nil && doc.UUIDDocumento != nil && archivosS3[strings.TrimSpace(*doc.UUIDDocumento)] != "" {
				documentos[k][doc.ID] = struct{}{}
			}
		}
		datos[k] = RegistroControlCeremonia{ID: k.ceremonia, Fecha: relacion.CeremoniaGrado.FechaCeremonia, Lugar: relacion.CeremoniaGrado.Lugar}
		if _, ok := nombres[k.facultad]; !ok {
			nombres[k.facultad] = defaultString(nombreDependenciaOikos(ctx, k.facultad), fmt.Sprintf("Facultad %d", k.facultad))
		}
	}
	porFacultad := map[int64][]RegistroControlCeremonia{}
	for k, ceremonia := range datos {
		ceremonia.DiplomasDisponibles = len(documentos[k])
		porFacultad[k.facultad] = append(porFacultad[k.facultad], ceremonia)
	}
	result := &RegistroControlCatalogo{Facultades: []RegistroControlFacultad{}}
	for id, ceremonias := range porFacultad {
		sort.Slice(ceremonias, func(i, j int) bool { return ceremonias[i].Fecha > ceremonias[j].Fecha })
		result.Facultades = append(result.Facultades, RegistroControlFacultad{ID: id, Nombre: nombres[id], Ceremonias: ceremonias})
	}
	sort.Slice(result.Facultades, func(i, j int) bool { return result.Facultades[i].Nombre < result.Facultades[j].Nombre })
	return result, nil
}

func (DiplomasZIPService) Generar(ctx context.Context, input DiplomasZIPInput) ([]byte, error) {
	relaciones, err := (CeremoniaService{}).listarRelacionesActivas(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]diplomaZIPItem, 0)
	vistos := map[int64]struct{}{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo || (input.CeremoniaID > 0 && relacion.CeremoniaGrado.ID != input.CeremoniaID) || (input.FacultadID > 0 && relacion.FacultadID != input.FacultadID) {
			continue
		}
		asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
		if err != nil {
			return nil, err
		}
		for _, asignacion := range asignaciones {
			doc := asignacion.DocumentoDigital
			if !asignacion.Activo || doc == nil || doc.UUIDDocumento == nil || strings.TrimSpace(*doc.UUIDDocumento) == "" {
				continue
			}
			if _, ok := vistos[doc.ID]; ok {
				continue
			}
			vistos[doc.ID] = struct{}{}
			items = append(items, diplomaZIPItem{FacultadID: relacion.FacultadID, CeremoniaID: relacion.CeremoniaGrado.ID,
				Codigo: doc.CodigoEstudiante, FechaCeremonia: relacion.CeremoniaGrado.FechaCeremonia,
				UUID: strings.TrimSpace(*doc.UUIDDocumento), Lugar: relacion.CeremoniaGrado.Lugar, Documento: doc})
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: no hay diplomas definitivos para los filtros seleccionados", ErrInvalidState)
	}
	archivosS3, err := listarDiplomasOriginalesS3(ctx)
	if err != nil {
		return nil, err
	}
	disponibles := items[:0]
	for _, item := range items {
		if archivosS3[item.UUID] != "" {
			disponibles = append(disponibles, item)
		}
	}
	items = disponibles
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: los diplomas seleccionados no tienen un PDF original almacenado en S3", ErrInvalidState)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].FacultadID != items[j].FacultadID {
			return items[i].FacultadID < items[j].FacultadID
		}
		if items[i].CeremoniaID != items[j].CeremoniaID {
			return items[i].CeremoniaID < items[j].CeremoniaID
		}
		return items[i].Codigo < items[j].Codigo
	})
	verificaciones, err := consultarVerificacionesReporte(ctx, items)
	if err != nil {
		return nil, err
	}
	filas := make([][]string, 0, len(items))
	client, err := nuevoClienteS3(ctx)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	zw := zip.NewWriter(&output)
	for _, item := range items {
		diploma, err := buscarDiplomaDigitalExistente(ctx, item.Documento.ID)
		if err != nil {
			return nil, err
		}
		key := archivosS3[item.UUID]
		object, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(diplomasS3Bucket()), Key: aws.String(key)})
		if err != nil {
			return nil, fmt.Errorf("obtener diploma original de s3: %w", err)
		}
		createErr := escribirDiplomaZIP(zw, item, object.Body)
		closeErr := object.Body.Close()
		if createErr != nil {
			return nil, createErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		filas = append(filas, filaReporteDiploma(ctx, item, diploma, verificaciones[item.UUID]))
	}
	reporte, err := crearReporteDiplomasExcel(filas)
	if err != nil {
		return nil, err
	}
	entry, err := zw.Create("reporte_diplomas.xlsx")
	if err != nil {
		return nil, err
	}
	if _, err := entry.Write(reporte); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func listarDiplomasOriginalesS3(ctx context.Context) (map[string]string, error) {
	client, err := nuevoClienteS3(ctx)
	if err != nil {
		return nil, err
	}
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(diplomasS3Bucket()),
		Prefix: aws.String("diplomas/"),
	})
	archivos := make(map[string]string)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listar diplomas originales en s3: %w", err)
		}
		for _, object := range page.Contents {
			if object.Key == nil || !strings.HasSuffix(strings.ToLower(*object.Key), ".pdf") {
				continue
			}
			partes := strings.SplitN(*object.Key, "/", 3)
			if len(partes) == 3 && partes[0] == "diplomas" && partes[1] != "" && archivos[partes[1]] == "" {
				archivos[partes[1]] = *object.Key
			}
		}
	}
	return archivos, nil
}

func rutaDiplomaZIP(item diplomaZIPItem) string {
	fecha := normalizarFechaZIP(item.FechaCeremonia)
	return fmt.Sprintf("Facultad_%d/Ceremonia_%s_%d/%s_Diploma_%d.pdf", item.FacultadID, fecha, item.CeremoniaID, fecha, item.Codigo)
}

func escribirDiplomaZIP(zw *zip.Writer, item diplomaZIPItem, contenido io.Reader) error {
	entry, err := zw.Create(rutaDiplomaZIP(item))
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, contenido)
	return err
}

func normalizarFechaZIP(value string) string {
	value = strings.TrimSpace(value)
	if fecha, err := time.Parse(time.RFC3339, value); err == nil {
		return fechaEnBogota(fecha).Format("20060102")
	}
	if len(value) >= 10 {
		return strings.ReplaceAll(value[:10], "-", "")
	}
	return "sin_fecha"
}

func nuevoClienteS3(ctx context.Context) (*s3.Client, error) {
	options := []func(*config.LoadOptions) error{config.WithRegion(awsRegion())}
	if awsAccessKeyID() != "" && awsSecretAccessKey() != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(awsAccessKeyID(), awsSecretAccessKey(), "")))
	}
	if awsEndpointURL() != "" {
		options = append(options, config.WithBaseEndpoint(awsEndpointURL()))
	}
	cfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true }), nil
}
