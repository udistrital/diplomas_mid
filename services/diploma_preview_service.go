package services

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/phpdave11/gofpdf"
	"github.com/udistrital/utils_oas/v2/request"
)

type DiplomaPreviewService struct{}

type DiplomaPreviewResult struct {
	Preview          bool                     `json:"preview"`
	DocumentoID      int64                    `json:"documento_id"`
	DiplomaID        int64                    `json:"diploma_id,omitempty"`
	TipoDocumento    string                   `json:"tipo_documento"`
	DocumentoDigital documentoDigitalResponse `json:"documento_digital"`
	DiplomaDigital   *diplomaDigitalResponse  `json:"diploma_digital,omitempty"`
	Metadatos        DiplomaPreviewMetadatos  `json:"metadatos"`
	File             string                   `json:"file"`
}

type DiplomaPreviewMetadatos struct {
	Titulo                  string `json:"titulo"`
	NombreEstudiante        string `json:"nombre_estudiante"`
	TipoDocumentoEstudiante string `json:"tipo_documento_estudiante"`
	NumeroDocumento         string `json:"numero_documento"`
	LugarExpedicion         string `json:"lugar_expedicion"`
	FechaCeremonia          string `json:"fecha_ceremonia"`
	DiplomaID               int64  `json:"diploma_id,omitempty"`
	Libro                   int    `json:"libro,omitempty"`
	Folio                   int    `json:"folio,omitempty"`
	NumeroRegistro          int    `json:"numero_registro,omitempty"`
	Facultad                string `json:"facultad"`
}

type diplomaSignatureRender struct {
	Orden    int
	Nombre   string
	Cargo    string
	Imagen   string
	ImageRef string
}

type pdfImageObject struct {
	Name   string
	Width  int
	Height int
	Data   []byte
}

type diplomaTemplateData struct {
	CSSPath          string
	CambriaPath      string
	CambriaBoldPath  string
	EngraversPath    string
	MostrarQRDemo    bool
	EscudoSrc        template.URL
	Titulo           string
	NombreEstudiante string
	Documento        string
	FechaCeremonia   string
	Firmas           []diplomaTemplateSignature
	RegistroLinea    string
	NumeroDiploma    string
}

type diplomaTemplateSignature struct {
	Left   string
	Label  string
	Imagen template.URL
}

func (s DiplomaPreviewService) Generar(ctx context.Context, documentoID int64) (*DiplomaPreviewResult, error) {
	return generarDiplomaPreviewConOpciones(ctx, documentoID, nil, true)
}

func generarDiplomaPreview(ctx context.Context, documentoID int64, firmaAdicional *FirmanteActivo) (*DiplomaPreviewResult, error) {
	return generarDiplomaPreviewConOpciones(ctx, documentoID, firmaAdicional, false)
}

func generarDiplomaPreviewConOpciones(ctx context.Context, documentoID int64, firmaAdicional *FirmanteActivo, mostrarQRDemo bool) (*DiplomaPreviewResult, error) {
	if documentoID <= 0 {
		return nil, fmt.Errorf("%w: documento id is required", ErrInvalidInput)
	}

	documento, err := consultarDocumentoDigital(ctx, documentoID)
	if err != nil {
		return nil, err
	}

	diploma, err := buscarDiplomaDigitalExistente(ctx, documentoID)
	if err != nil {
		return nil, err
	}

	metadatos := construirMetadatosDiplomaPreview(ctx, documento, diploma)
	firmas, err := consultarFirmasDiplomaPreview(ctx, documentoID, firmaAdicional)
	if err != nil {
		return nil, err
	}
	pdf, err := construirPDFDiplomaPreview(metadatos, firmas, mostrarQRDemo)
	if err != nil {
		return nil, err
	}
	tipoDocumento := "DIPLOMA"
	if documento.TipoDocumentoDigitalID > 0 {
		tipoDocumento = strconv.FormatInt(documento.TipoDocumentoDigitalID, 10)
	}

	result := &DiplomaPreviewResult{
		Preview:          true,
		DocumentoID:      documentoID,
		TipoDocumento:    tipoDocumento,
		DocumentoDigital: *documento,
		DiplomaDigital:   diploma,
		Metadatos:        metadatos,
		File:             base64.StdEncoding.EncodeToString(pdf),
	}
	if diploma != nil {
		result.DiplomaID = diploma.ID
	}
	return result, nil
}

func consultarFirmasDiplomaPreview(ctx context.Context, documentoID int64, firmaAdicional *FirmanteActivo) ([]diplomaSignatureRender, error) {
	firmasDocumento, err := listarFirmasDocumento(ctx, documentoID)
	if err != nil {
		return nil, err
	}

	firmas := make([]diplomaSignatureRender, 0, len(firmasDocumento)+1)
	ordenes := make(map[int]struct{}, len(firmasDocumento)+1)
	for _, firmaDocumento := range firmasDocumento {
		orden, ok := ordenRolFirmante(firmaDocumento.RolFirmanteID)
		if !ok {
			continue
		}
		firma, err := construirFirmaPreview(ctx, orden, firmaDocumento.DocumentoIdentidad)
		if err != nil {
			return nil, err
		}
		firmas = append(firmas, firma)
		ordenes[orden] = struct{}{}
	}

	if firmaAdicional != nil {
		if _, existe := ordenes[firmaAdicional.Orden]; !existe {
			firma, err := construirFirmaPreviewDesdeFirmante(ctx, firmaAdicional)
			if err != nil {
				return nil, err
			}
			firmas = append(firmas, firma)
		}
	}

	return firmas, nil
}

func construirFirmaPreview(ctx context.Context, orden int, documentoIdentidad int64) (diplomaSignatureRender, error) {
	nombre := strconv.FormatInt(documentoIdentidad, 10)
	firmante, err := FirmanteService{}.ConsultarRolActivo(ctx, documentoIdentidad, time.Now())
	if err == nil && firmante != nil {
		nombre = firmante.Nombre
	}

	imagen, err := consultarFirmaGraficaFirmante(ctx, documentoIdentidad)
	if err != nil {
		if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrExternalService) {
			return construirFirmaPreviewHistorica(orden, documentoIdentidad, nombre, ""), nil
		}
		return diplomaSignatureRender{}, err
	}
	return construirFirmaPreviewHistorica(orden, documentoIdentidad, nombre, imagen), nil
}

func construirFirmaPreviewHistorica(orden int, documentoIdentidad int64, nombre, imagen string) diplomaSignatureRender {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		nombre = strconv.FormatInt(documentoIdentidad, 10)
	}
	return diplomaSignatureRender{
		Orden:  orden,
		Nombre: nombre,
		Cargo:  cargoPreviewPorOrden(orden),
		Imagen: imagen,
	}
}

func construirFirmaPreviewDesdeFirmante(ctx context.Context, firmante *FirmanteActivo) (diplomaSignatureRender, error) {
	imagen, err := consultarFirmaGraficaFirmante(ctx, firmante.DocumentoIdentidad)
	if err != nil {
		if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrExternalService) {
			return diplomaSignatureRender{
				Orden:  firmante.Orden,
				Nombre: firmante.Nombre,
				Cargo:  firmante.Cargo,
			}, nil
		}
		return diplomaSignatureRender{}, err
	}
	return diplomaSignatureRender{
		Orden:  firmante.Orden,
		Nombre: firmante.Nombre,
		Cargo:  firmante.Cargo,
		Imagen: imagen,
	}, nil
}

func cargoPreviewPorOrden(orden int) string {
	switch orden {
	case 1:
		return "SECRETARIO ACADEMICO"
	case 2:
		return "DECANO"
	case 3:
		return "SECRETARIO GENERAL"
	case 4:
		return "RECTOR"
	default:
		return fmt.Sprintf("FIRMANTE ORDEN %d", orden)
	}
}

func consultarDocumentoDigital(ctx context.Context, documentoID int64) (*documentoDigitalResponse, error) {
	var documento documentoDigitalResponse
	status, err := request.GetWithContext(ctx, fmt.Sprintf("%s/documento_digital/%d", diplomasCrudURL(), documentoID), &documento)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud documento_digital returned status %d", ErrExternalService, status)
	}
	return &documento, nil
}

func construirMetadatosDiplomaPreview(ctx context.Context, documento *documentoDigitalResponse, diploma *diplomaDigitalResponse) DiplomaPreviewMetadatos {
	metadatos := DiplomaPreviewMetadatos{
		Titulo:                  strings.TrimSpace(documento.TituloOtorgado),
		NombreEstudiante:        strings.TrimSpace(documento.NombreEstudiante),
		TipoDocumentoEstudiante: strings.TrimSpace(documento.TipoDocumentoEstudiante),
		NumeroDocumento:         strings.TrimSpace(documento.NumeroDocumentoEstudiante),
		LugarExpedicion:         strings.TrimSpace(documento.MunicipioExpedicion),
		Facultad:                facultadTexto(ctx, documento.FacultadID),
	}
	if diploma != nil {
		metadatos.DiplomaID = diploma.ID
		metadatos.Titulo = defaultString(diploma.TituloOtorgado, metadatos.Titulo)
		metadatos.NombreEstudiante = defaultString(diploma.NombreEstudiante, metadatos.NombreEstudiante)
		metadatos.TipoDocumentoEstudiante = defaultString(diploma.TipoDocumentoEstudiante, metadatos.TipoDocumentoEstudiante)
		metadatos.NumeroDocumento = defaultString(diploma.NumeroDocumentoEstudiante, metadatos.NumeroDocumento)
		metadatos.LugarExpedicion = defaultString(diploma.MunicipioExpedicion, metadatos.LugarExpedicion)
		metadatos.FechaCeremonia = fechaCeremoniaBogota(diploma.FechaGrado)
		if diploma.RegistroGrado != nil {
			metadatos.NumeroRegistro = diploma.RegistroGrado.NumeroRegistro
			if diploma.RegistroGrado.FolioRegistro != nil {
				metadatos.Folio = diploma.RegistroGrado.FolioRegistro.NumeroFolio
				if diploma.RegistroGrado.FolioRegistro.LibroRegistro != nil {
					metadatos.Libro = diploma.RegistroGrado.FolioRegistro.LibroRegistro.NumeroLibro
				}
			}
		}
		metadatos.Facultad = facultadTexto(ctx, &diploma.FacultadID)
	}
	if metadatos.FechaCeremonia == "" {
		metadatos.FechaCeremonia = fechaCeremoniaBogota(time.Now())
	}
	return metadatos
}

func facultadTexto(ctx context.Context, facultadID *int64) string {
	if facultadID == nil || *facultadID <= 0 {
		return "FACULTAD"
	}
	if nombre := nombreDependenciaOikos(ctx, *facultadID); nombre != "" {
		return nombre
	}
	return fmt.Sprintf("FACULTAD %d", *facultadID)
}

func fechaCeremoniaBogota(fecha time.Time) string {
	if fecha.IsZero() {
		return ""
	}
	meses := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	fecha = fechaEnBogota(fecha)
	return fmt.Sprintf("En la ciudad de Bogota a los %d dias del mes de %s de %d", fecha.Day(), meses[int(fecha.Month())-1], fecha.Year())
}

func fechaEnBogota(fecha time.Time) time.Time {
	return fecha.In(time.FixedZone("America/Bogota", -5*60*60))
}

func construirPDFDiplomaPreview(m DiplomaPreviewMetadatos, firmas []diplomaSignatureRender, mostrarQRDemo bool) ([]byte, error) {
	if err := validarFuentesDiploma(); err != nil {
		return nil, err
	}
	return construirPDFDiplomaPreviewChrome(m, firmas, mostrarQRDemo)
}

func construirPDFDiplomaPreviewChrome(m DiplomaPreviewMetadatos, firmas []diplomaSignatureRender, mostrarQRDemo bool) ([]byte, error) {
	htmlContent, err := construirHTMLDiplomaPreview(m, firmas, mostrarQRDemo)
	if err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp("", "diploma-preview-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "diploma.html")
	pdfPath := filepath.Join(tmpDir, "diploma.pdf")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0600); err != nil {
		return nil, err
	}

	chromePath, err := exec.LookPath("google-chrome")
	if err != nil {
		chromePath, err = exec.LookPath("chromium")
		if err != nil {
			chromePath, err = exec.LookPath("chromium-browser")
			if err != nil {
				return nil, fmt.Errorf("%w: google-chrome/chromium is required to render diploma PDF", ErrInvalidInput)
			}
		}
	}

	cmd := exec.Command(
		chromePath,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--no-pdf-header-footer",
		"--user-data-dir="+filepath.Join(tmpDir, "chrome-user"),
		"--print-to-pdf="+pdfPath,
		"file://"+htmlPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: chrome render diploma failed: %v: %s", ErrExternalService, err, strings.TrimSpace(string(output)))
	}
	return os.ReadFile(pdfPath)
}

func construirHTMLDiplomaPreview(m DiplomaPreviewMetadatos, firmas []diplomaSignatureRender, mostrarQRDemo bool) (string, error) {
	escudoSrc, err := escudoInstitucionalTemplateSrc()
	if err != nil {
		return "", err
	}
	documento := strings.TrimSpace(strings.Join([]string{m.TipoDocumentoEstudiante, m.NumeroDocumento}, " "))
	if m.LugarExpedicion != "" {
		documento = documento + " de " + m.LugarExpedicion
	}

	data, err := construirDatosTemplateDiploma(m, firmas, mostrarQRDemo, escudoSrc, documento)
	if err != nil {
		return "", err
	}

	templatePath := resolveLocalPath("assets/templates/diploma.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("%w: diploma template parse failed: %v", ErrInvalidInput, err)
	}

	var output strings.Builder
	if err := tmpl.Execute(&output, data); err != nil {
		return "", fmt.Errorf("%w: diploma template execute failed: %v", ErrInvalidInput, err)
	}
	return output.String(), nil
}

func normalizarDataURIImagen(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		return value
	}
	return "data:image/png;base64," + value
}

func construirDatosTemplateDiploma(m DiplomaPreviewMetadatos, firmas []diplomaSignatureRender, mostrarQRDemo bool, escudoSrc template.URL, documento string) (*diplomaTemplateData, error) {
	cssPath, err := filepath.Abs(resolveLocalPath("assets/templates/diploma.css"))
	if err != nil {
		return nil, err
	}
	engraversPath, err := filepath.Abs(diplomaEngraversFontPath())
	if err != nil {
		return nil, err
	}
	cambriaPath, err := filepath.Abs(diplomaCambriaFontPath())
	if err != nil {
		return nil, err
	}
	cambriaBoldPath, err := filepath.Abs(diplomaCambriaBoldFontPath())
	if err != nil {
		return nil, err
	}

	return &diplomaTemplateData{
		CSSPath:          filepath.ToSlash(cssPath),
		CambriaPath:      filepath.ToSlash(cambriaPath),
		CambriaBoldPath:  filepath.ToSlash(cambriaBoldPath),
		EngraversPath:    filepath.ToSlash(engraversPath),
		MostrarQRDemo:    mostrarQRDemo,
		EscudoSrc:        escudoSrc,
		Titulo:           formatoTituloDiploma(defaultString(m.Titulo, "TITULO OTORGADO")),
		NombreEstudiante: formatoNombreDiploma(defaultString(m.NombreEstudiante, "NOMBRE DEL ESTUDIANTE")),
		Documento:        strings.TrimPrefix(documento, "CC "),
		FechaCeremonia:   strings.ToUpper(m.FechaCeremonia),
		Firmas:           construirFirmasTemplateDiploma(firmas),
		RegistroLinea:    lineaRegistroDiploma(m),
		NumeroDiploma:    valorInt64(m.DiplomaID),
	}, nil
}

func lineaRegistroDiploma(m DiplomaPreviewMetadatos) string {
	return fmt.Sprintf(
		"Registro No. %s %s Folio No. %s Libro No. %s",
		siglaFacultadRegistro(m.Facultad),
		valorInt(m.NumeroRegistro),
		valorFolioDiploma(m.Folio),
		valorInt(m.Libro),
	)
}

func valorFolioDiploma(value int) string {
	if value <= 0 {
		return "1"
	}
	return fmt.Sprintf("%03d", value)
}

func siglaFacultadRegistro(facultad string) string {
	value := strings.TrimSpace(strings.ToUpper(facultad))
	if value == "" {
		return "F"
	}
	parts := strings.Fields(value)
	if len(parts) == 2 && parts[0] == "FACULTAD" {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return "F" + parts[1]
		}
	}

	stopWords := map[string]struct{}{
		"DE": {}, "DEL": {}, "LA": {}, "LAS": {}, "LOS": {}, "EL": {}, "Y": {}, "E": {},
	}
	var initials strings.Builder
	for _, part := range parts {
		cleaned := strings.Trim(part, ".,;:-_()[]{}")
		if cleaned == "" {
			continue
		}
		if _, skip := stopWords[cleaned]; skip {
			continue
		}
		for _, r := range cleaned {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				initials.WriteRune(r)
				break
			}
		}
	}
	if initials.Len() == 0 {
		return "F"
	}
	return initials.String()
}

func construirFirmasTemplateDiploma(firmas []diplomaSignatureRender) []diplomaTemplateSignature {
	signatureSlots := []struct {
		orden int
		x     float64
		label string
	}{
		{4, 62, "RECTOR"},
		{3, 154, "SECRETARIO GENERAL"},
		{2, 246, "DECANO DE LA FACULTAD"},
		{1, 338, "SECRETARIO ACADEMICO"},
	}
	firmasPorOrden := make(map[int]diplomaSignatureRender, len(firmas))
	for _, firma := range firmas {
		firmasPorOrden[firma.Orden] = firma
	}

	result := make([]diplomaTemplateSignature, 0, len(signatureSlots))
	for _, slot := range signatureSlots {
		firma := firmasPorOrden[slot.orden]
		var image template.URL
		if strings.TrimSpace(firma.Imagen) != "" {
			image = template.URL(normalizarDataURIImagen(firma.Imagen))
		}
		result = append(result, diplomaTemplateSignature{
			Left:   fmt.Sprintf("%.2fmm", slot.x-30),
			Label:  slot.label,
			Imagen: image,
		})
	}
	return result
}

func formatoTituloDiploma(value string) string {
	return titleCaseConectores(value)
}

func formatoNombreDiploma(value string) string {
	return titleCasePalabras(reordenarNombreApellidos(value))
}

func reordenarNombreApellidos(value string) string {
	words := strings.Fields(value)
	if len(words) <= 1 {
		return strings.TrimSpace(value)
	}
	if strings.Contains(value, ",") {
		parts := strings.SplitN(value, ",", 2)
		apellidos := strings.Fields(parts[0])
		nombres := strings.Fields(parts[1])
		if len(nombres) > 0 && len(apellidos) > 0 {
			return strings.Join(append(nombres, apellidos...), " ")
		}
	}

	nombresCount := inferirCantidadNombres(words)
	if nombresCount <= 0 || nombresCount >= len(words) {
		return strings.Join(words, " ")
	}
	apellidos := words[:len(words)-nombresCount]
	nombres := words[len(words)-nombresCount:]
	return strings.Join(append(nombres, apellidos...), " ")
}

func inferirCantidadNombres(words []string) int {
	switch len(words) {
	case 2:
		return 1
	case 3:
		return 1
	}

	if len(words) >= 4 {
		return len(words) - 2
	}
	return 1
}

func titleCaseConectores(value string) string {
	words := strings.Fields(value)
	for index, word := range words {
		lower := strings.ToLower(word)
		if index > 0 && conectoresTitulo[quitarTildes(lower)] {
			words[index] = lower
			continue
		}
		words[index] = capitalizarPalabra(lower)
	}
	return strings.Join(words, " ")
}

func titleCasePalabras(value string) string {
	words := strings.Fields(value)
	for index, word := range words {
		words[index] = capitalizarPalabra(strings.ToLower(word))
	}
	return strings.Join(words, " ")
}

var conectoresTitulo = map[string]bool{
	"de": true, "del": true, "la": true, "las": true, "los": true,
	"y": true, "e": true, "en": true, "con": true,
}

func capitalizarPalabra(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	for index := 1; index < len(runes); index++ {
		runes[index] = unicode.ToLower(runes[index])
	}
	return string(runes)
}

func quitarTildes(value string) string {
	replacer := strings.NewReplacer(
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	)
	return replacer.Replace(value)
}

func barcodeDemoHTML() string {
	var builder strings.Builder
	for index := 0; index < 28; index++ {
		width := "0.45mm"
		if index%4 == 0 {
			width = "0.8mm"
		}
		if index%3 == 0 {
			builder.WriteString(`<span style="width: 0.3mm; background: transparent;"></span>`)
			continue
		}
		builder.WriteString(`<span style="width: ` + width + `;"></span>`)
	}
	return builder.String()
}

func construirPDFDiplomaPreviewGofpdf(m DiplomaPreviewMetadatos, firmas []diplomaSignatureRender) ([]byte, error) {
	const (
		pageWidthMM  = 400.0
		pageHeightMM = 280.0
	)
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: pageWidthMM, Ht: pageHeightMM},
	})
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8Font("Cambria", "", diplomaCambriaFontPath())
	pdf.AddUTF8Font("Cambria", "B", diplomaCambriaBoldFontPath())
	pdf.AddUTF8Font("Engravers", "B", diplomaEngraversFontPath())
	pdf.AddPage()
	pdf.SetTextColor(0, 0, 0)

	logo, err := escudoInstitucionalRecortadoPNG()
	if err != nil {
		return nil, err
	}
	pdf.RegisterImageOptionsReader("escudo-ud", gofpdf.ImageOptions{ImageType: "png"}, bytes.NewReader(logo))
	pdf.ImageOptions("escudo-ud", 184, 32, 32, 0, false, gofpdf.ImageOptions{ImageType: "png"}, 0, "")

	pdfTextCenterMM(pdf, "Cambria", "B", 8, 19, "REPUBLICA DE COLOMBIA")
	pdfTextCenterMM(pdf, "Cambria", "B", 7, 24, "MINISTERIO DE EDUCACION NACIONAL Y EN SU NOMBRE")
	pdfTextCenterFitMM(pdf, "Cambria", "B", 19, 14, 86, 348, "LA UNIVERSIDAD DISTRITAL FRANCISCO JOSE DE CALDAS")
	pdfTextCenterMM(pdf, "Cambria", "", 7, 106, "CONFIERE EL TITULO DE")
	pdfTextCenterWrappedMM(pdf, "Engravers", "B", 19, 14, 122, 325, 12, defaultString(m.Titulo, "TITULO OTORGADO"))
	pdfTextCenterMM(pdf, "Cambria", "", 11, 143, "A")
	pdfTextCenterWrappedMM(pdf, "Engravers", "B", 27, 17, 158, 350, 15, defaultString(m.NombreEstudiante, "NOMBRE DEL ESTUDIANTE"))
	documento := strings.TrimSpace(strings.Join([]string{m.TipoDocumentoEstudiante, m.NumeroDocumento}, " "))
	if m.LugarExpedicion != "" {
		documento = documento + " de " + m.LugarExpedicion
	}
	pdfTextCenterMM(pdf, "Cambria", "B", 8, 176, strings.TrimSpace("Con C.C. No. "+strings.TrimPrefix(documento, "CC ")))
	pdfTextCenterMM(pdf, "Cambria", "B", 6.2, 188, "QUIEN CUMPLIO CON LAS CONDICIONES ACADEMICAS REQUERIDAS. EN TESTIMONIO DE ELLO OTORGA EL PRESENTE")
	pdfTextCenterMM(pdf, "Cambria", "B", 15, 199, "DIPLOMA")
	pdfTextCenterMM(pdf, "Cambria", "B", 6.8, 214, strings.ToUpper(m.FechaCeremonia))

	dibujarFirmasDiplomaPDF(pdf, firmas)
	dibujarQRDemoGofpdf(pdf, 15, 254, 8)
	dibujarCodigoBarrasDemoGofpdf(pdf, 190, 253, 28, 7)
	pdfTextCenterMM(pdf, "Cambria", "", 6, 255, fmt.Sprintf("No. %s", valorInt64(m.DiplomaID)))
	pdfTextCenterMM(pdf, "Cambria", "", 5.8, 247, fmt.Sprintf("Registro No. %s", valorInt(m.NumeroRegistro)))
	pdfTextCenterMM(pdf, "Cambria", "", 5.8, 271, fmt.Sprintf("Folio No. %s", valorInt(m.Folio)))
	pdfTextCenterMM(pdf, "Cambria", "", 5.8, 295, fmt.Sprintf("Libro No. %s", valorInt(m.Libro)))
	pdfTextCenterFitMM(pdf, "Cambria", "", 5, 4.5, 267, 105, m.Facultad)

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func validarFuentesDiploma() error {
	required := map[string]string{
		"Cambria":                    diplomaCambriaFontPath(),
		"Cambria Bold":               diplomaCambriaBoldFontPath(),
		"Engravers Old English Bold": diplomaEngraversFontPath(),
	}
	for name, path := range required {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%w: fuente requerida para diploma no encontrada: %s (%s)", ErrInvalidInput, name, path)
		}
	}
	return nil
}

func escudoInstitucionalTemplateSrc() (template.URL, error) {
	path := diplomaInstitutionalLogoPath()
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return template.URL("file://" + filepath.ToSlash(absPath)), nil
	}

	escudo, err := escudoInstitucionalRecortadoPNG()
	if err != nil {
		return "", err
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(escudo)), nil
}

func escudoInstitucionalRecortadoPNG() ([]byte, error) {
	raw, err := os.ReadFile(diplomaInstitutionalLogoPath())
	if err != nil {
		return nil, fmt.Errorf("%w: no se pudo leer escudo institucional: %v", ErrExternalService, err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: escudo institucional no es una imagen valida: %v", ErrExternalService, err)
	}
	crop := image.Rect(105, 62, 395, 366).Intersect(img.Bounds())
	if crop.Empty() {
		crop = img.Bounds()
	}
	dst := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(dst, dst.Bounds(), img, crop.Min, draw.Src)
	transparentarFondoClaro(dst)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func transparentarFondoClaro(img *image.RGBA) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			offset := img.PixOffset(x, y)
			r := img.Pix[offset]
			g := img.Pix[offset+1]
			b := img.Pix[offset+2]
			if r > 238 && g > 238 && b > 238 {
				img.Pix[offset+3] = 0
				continue
			}
			if r > 225 && g > 225 && b > 225 {
				img.Pix[offset+3] = 80
			}
		}
	}
}

func pdfTextCenterMM(pdf *gofpdf.Fpdf, family, style string, size, y float64, text string) {
	pdf.SetFont(family, style, size)
	pdf.SetXY(0, y)
	pdf.CellFormat(400, size*0.45, normalizarTextoPDF(text), "", 0, "C", false, 0, "")
}

func pdfTextCenterFitMM(pdf *gofpdf.Fpdf, family, style string, size, minSize, y, maxWidth float64, text string) {
	text = normalizarTextoPDF(text)
	for size > minSize {
		pdf.SetFont(family, style, size)
		if pdf.GetStringWidth(text) <= maxWidth {
			break
		}
		size -= 0.5
	}
	pdfTextCenterMM(pdf, family, style, size, y, text)
}

func pdfTextCenterWrappedMM(pdf *gofpdf.Fpdf, family, style string, size, minSize, y, maxWidth, lineHeight float64, text string) {
	text = normalizarTextoPDF(text)
	pdf.SetFont(family, style, size)
	lines := wrapPDFTextMM(pdf, text, maxWidth)
	if len(lines) > 2 {
		lines = []string{text}
	}
	if len(lines) == 1 {
		pdfTextCenterFitMM(pdf, family, style, size, minSize, y, maxWidth, lines[0])
		return
	}
	startY := y - (float64(len(lines)-1) * lineHeight / 2)
	for index, line := range lines {
		pdfTextCenterFitMM(pdf, family, style, size, minSize, startY+float64(index)*lineHeight, maxWidth, line)
	}
}

func wrapPDFTextMM(pdf *gofpdf.Fpdf, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if pdf.GetStringWidth(candidate) <= maxWidth {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func dibujarFirmasDiplomaPDF(pdf *gofpdf.Fpdf, firmas []diplomaSignatureRender) {
	slots := []struct {
		orden int
		x     float64
		label string
	}{
		{4, 62, "RECTOR"},
		{3, 154, "SECRETARIO GENERAL"},
		{2, 246, "DECANO DE LA FACULTAD"},
		{1, 338, "SECRETARIO ACADEMICO"},
	}
	firmasPorOrden := make(map[int]diplomaSignatureRender, len(firmas))
	for _, firma := range firmas {
		firmasPorOrden[firma.Orden] = firma
	}
	for _, slot := range slots {
		if firma, ok := firmasPorOrden[slot.orden]; ok && strings.TrimSpace(firma.Imagen) != "" {
			if raw, err := decodeDataURIBase64(firma.Imagen); err == nil {
				imageName := fmt.Sprintf("firma-%d", slot.orden)
				pdf.RegisterImageOptionsReader(imageName, gofpdf.ImageOptions{ImageType: "png"}, bytes.NewReader(raw))
				pdf.ImageOptions(imageName, slot.x-21, 230, 42, 0, false, gofpdf.ImageOptions{ImageType: "png"}, 0, "")
			}
		}
		pdf.SetDrawColor(20, 20, 20)
		pdf.SetLineWidth(0.2)
		pdf.Line(slot.x-24, 244, slot.x+24, 244)
		pdfTextAtCenterMM(pdf, "Cambria", "B", 5.2, slot.x, 249, slot.label)
	}
}

func pdfTextAtCenterMM(pdf *gofpdf.Fpdf, family, style string, size, centerX, y float64, text string) {
	pdf.SetFont(family, style, size)
	text = normalizarTextoPDF(text)
	width := pdf.GetStringWidth(text)
	pdf.Text(centerX-width/2, y, text)
}

func dibujarQRDemoGofpdf(pdf *gofpdf.Fpdf, x, y, size float64) {
	cell := size / 9
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetFillColor(0, 0, 0)
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			if row < 2 || col < 2 || row == col || (row+col)%4 == 0 {
				pdf.Rect(x+float64(col)*cell, y+float64(row)*cell, cell*0.85, cell*0.85, "F")
			}
		}
	}
}

func dibujarCodigoBarrasDemoGofpdf(pdf *gofpdf.Fpdf, x, y, width, height float64) {
	pdf.SetFillColor(0, 0, 0)
	for index := 0; index < 28; index++ {
		barWidth := 0.35
		if index%4 == 0 {
			barWidth = 0.7
		}
		if index%3 != 0 {
			pdf.Rect(x+float64(index)*width/28, y, barWidth, height, "F")
		}
	}
}

func prepararImagenesDiplomaPDF(firmas []diplomaSignatureRender) ([]pdfImageObject, error) {
	logoRaw, err := os.ReadFile(diplomaInstitutionalLogoPath())
	if err != nil {
		return nil, fmt.Errorf("%w: no se pudo leer escudo institucional: %v", ErrExternalService, err)
	}
	logo, err := crearPDFImageObjectRecortado("EscudoUD", logoRaw, 1, image.Rect(105, 62, 395, 366))
	if err != nil {
		return nil, fmt.Errorf("%w: escudo institucional no es una imagen valida: %v", ErrExternalService, err)
	}

	imagenes := make([]pdfImageObject, 0, len(firmas)+1)
	imagenes = append(imagenes, logo)
	for index := range firmas {
		if strings.TrimSpace(firmas[index].Imagen) == "" {
			continue
		}
		raw, err := decodeDataURIBase64(firmas[index].Imagen)
		if err != nil {
			return nil, err
		}
		imageObject, err := crearPDFImageObject(fmt.Sprintf("Im%d", len(imagenes)+1), raw, 1)
		if err != nil {
			return nil, err
		}
		firmas[index].ImageRef = imageObject.Name
		imagenes = append(imagenes, imageObject)
	}
	return imagenes, nil
}

func decodeDataURIBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%w: firma grafica vacia", ErrExternalService)
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: firma grafica data uri invalida", ErrExternalService)
		}
		value = parts[1]
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: firma grafica base64 invalida: %v", ErrExternalService, err)
	}
	return raw, nil
}

func crearPDFImageObject(name string, raw []byte, opacity float64) (pdfImageObject, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return pdfImageObject{}, err
	}
	return crearPDFImageObjectDesdeImagen(name, img, img.Bounds(), opacity)
}

func crearPDFImageObjectRecortado(name string, raw []byte, opacity float64, crop image.Rectangle) (pdfImageObject, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return pdfImageObject{}, err
	}
	crop = crop.Intersect(img.Bounds())
	if crop.Empty() {
		crop = img.Bounds()
	}
	return crearPDFImageObjectDesdeImagen(name, img, crop, opacity)
}

func crearPDFImageObjectDesdeImagen(name string, img image.Image, bounds image.Rectangle, opacity float64) (pdfImageObject, error) {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	width := bounds.Dx()
	height := bounds.Dy()
	rgb := make([]byte, 0, width*height*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			a = uint32(float64(a) * opacity)
			rgb = append(
				rgb,
				byte(compositeSobreBlanco(r, a)>>8),
				byte(compositeSobreBlanco(g, a)>>8),
				byte(compositeSobreBlanco(b, a)>>8),
			)
		}
	}

	var compressed bytes.Buffer
	writer, err := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	if err != nil {
		return pdfImageObject{}, err
	}
	if _, err := writer.Write(rgb); err != nil {
		return pdfImageObject{}, err
	}
	if err := writer.Close(); err != nil {
		return pdfImageObject{}, err
	}

	return pdfImageObject{
		Name:   name,
		Width:  width,
		Height: height,
		Data:   compressed.Bytes(),
	}, nil
}

func pdfDrawImage(content *strings.Builder, imageRef string, x, y, width, height float64) {
	content.WriteString(fmt.Sprintf("q %.2f 0 0 %.2f %.2f %.2f cm /%s Do Q\n", width, height, x, y, imageRef))
}

func compositeSobreBlanco(color uint32, alpha uint32) uint32 {
	const white = 0xffff
	return ((color * alpha) + (white * (white - alpha))) / white
}

func pdfDrawSignature(content *strings.Builder, firma diplomaSignatureRender) {
	if firma.ImageRef == "" {
		return
	}
	centerXByOrden := map[int]float64{
		1: 957,
		2: 692,
		3: 412,
		4: 152,
	}
	centerX, ok := centerXByOrden[firma.Orden]
	if !ok {
		return
	}
	width := 142.0
	height := 58.0
	x := centerX - width/2
	y := 166.0
	content.WriteString(fmt.Sprintf("q 1 1 1 rg %.2f %.2f %.2f %.2f re f Q\n", x-8, y-22, width+16, height+38))
	pdfDrawImage(content, firma.ImageRef, x, y, width, height)
	pdfTextCenter(content, "/F1", 6.5, centerX, y-8, firma.Nombre)
	pdfTextCenter(content, "/F1", 5.8, centerX, y-17, limitarTexto(firma.Cargo, 62))
}

func limitarTexto(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max])
}

func pdfSignatureSlot(content *strings.Builder, x, y float64, label string) {
	content.WriteString(fmt.Sprintf("q 0.12 0.12 0.12 RG 0.5 w %.2f %.2f m %.2f %.2f l S Q\n", x-58, y+28, x+92, y+28))
	pdfTextCenter(content, "/F2", 10, x+17, y, label)
}

func pdfDemoQR(content *strings.Builder, x, y, size float64) {
	cell := size / 9
	content.WriteString(fmt.Sprintf("q 1 1 1 rg %.2f %.2f %.2f %.2f re f 0 0 0 RG 0.5 w %.2f %.2f %.2f %.2f re S\n", x, y, size, size, x, y, size, size))
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			if row < 2 || col < 2 || row == col || (row+col)%4 == 0 {
				content.WriteString(fmt.Sprintf("0 0 0 rg %.2f %.2f %.2f %.2f re f\n", x+float64(col)*cell, y+float64(row)*cell, cell*0.85, cell*0.85))
			}
		}
	}
	content.WriteString("Q\n")
	pdfTextCenter(content, "/F1", 5.5, x+size/2, y-9, "QR DEMO")
}

func pdfTextCenter(content *strings.Builder, font string, size, centerX, y float64, text string) {
	width := estimatePDFTextWidth(text, size)
	pdfText(content, font, size, centerX-width, y, text)
}

func pdfTextCenterFit(content *strings.Builder, font string, size, minSize, centerX, y, maxWidth float64, text string) {
	text = strings.TrimSpace(text)
	for size > minSize && estimatePDFTextWidth(text, size)*2 > maxWidth {
		size -= 0.5
	}
	pdfTextCenter(content, font, size, centerX, y, text)
}

func pdfTextCenterWrapped(content *strings.Builder, font string, size, minSize, centerX, y, maxWidth, lineHeight float64, text string) {
	lines := wrapPDFText(text, maxWidth, size)
	if len(lines) > 2 {
		lines = []string{text}
	}
	if len(lines) == 1 {
		pdfTextCenterFit(content, font, size, minSize, centerX, y, maxWidth, lines[0])
		return
	}
	startY := y + lineHeight/2
	for index, line := range lines {
		pdfTextCenterFit(content, font, size, minSize, centerX, startY-float64(index)*lineHeight, maxWidth, line)
	}
}

func wrapPDFText(text string, maxWidth, size float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if estimatePDFTextWidth(candidate, size)*2 <= maxWidth {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	lines = append(lines, current)
	return lines
}

func estimatePDFTextWidth(text string, size float64) float64 {
	return float64(len([]rune(text))) * size * 0.28
}

func pdfText(content *strings.Builder, font string, size, x, y float64, text string) {
	text = normalizarTextoPDF(text)
	content.WriteString(fmt.Sprintf("BT %s %.2f Tf 1 0 0 1 %.2f %.2f Tm <%s> Tj ET\n", font, size, x, y, pdfHexText(text)))
}

func normalizarTextoPDF(text string) string {
	replacer := strings.NewReplacer(
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	)
	return replacer.Replace(text)
}

func pdfHexText(text string) string {
	encoded := utf16.Encode([]rune(text))
	buf := bytes.NewBuffer([]byte{0xfe, 0xff})
	for _, value := range encoded {
		buf.WriteByte(byte(value >> 8))
		buf.WriteByte(byte(value))
	}
	return strings.ToUpper(hex.EncodeToString(buf.Bytes()))
}

func buildSimplePDF(width, height float64, content string, images []pdfImageObject) []byte {
	xobjects := ""
	if len(images) > 0 {
		const firstImageObject = 9
		var xobjectBuilder strings.Builder
		xobjectBuilder.WriteString("/XObject <<")
		for index, img := range images {
			xobjectBuilder.WriteString(fmt.Sprintf(" /%s %d 0 R", img.Name, firstImageObject+index))
		}
		xobjectBuilder.WriteString(" >>")
		xobjects = xobjectBuilder.String()
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 4 0 R /F2 5 0 R /F3 6 0 R /F4 7 0 R >> %s >> /Contents 8 0 R >>", width, height, xobjects),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Times-Roman >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Times-Bold >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Times-Italic >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len([]byte(content)), content),
	}
	for _, img := range images {
		objects = append(objects, fmt.Sprintf(
			"<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream",
			img.Width,
			img.Height,
			len(img.Data),
			string(img.Data),
		))
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = pdf.Len()
		pdf.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", index+1, object))
	}
	xrefOffset := pdf.Len()
	pdf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1))
	for _, offset := range offsets[1:] {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
	}
	pdf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xrefOffset))
	return pdf.Bytes()
}

func valorInt(value int) string {
	if value <= 0 {
		return "PENDIENTE"
	}
	return strconv.Itoa(value)
}

func valorInt64(value int64) string {
	if value <= 0 {
		return "PENDIENTE"
	}
	return strconv.FormatInt(value, 10)
}

// Preparar solo permite reservar números para documentos de la facultad autorizada.
func (s DiplomaPreviewService) Preparar(ctx context.Context, documentoID, documentoSecretario int64) (*DiplomaPreviewResult, error) {
	if documentoID <= 0 || documentoSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento y secretario son obligatorios", ErrInvalidInput)
	}
	secretario, err := (CeremoniaService{}).secretarioAcademicoAutorizado(ctx, documentoSecretario)
	if err != nil {
		return nil, err
	}
	documento, err := consultarDocumentoDigital(ctx, documentoID)
	if err != nil {
		return nil, err
	}
	if documento.FacultadID == nil || *documento.FacultadID != secretario.FacultadID {
		return nil, ErrNoActiveRole
	}
	if err := asegurarRegistroDiploma(ctx, documentoID); err != nil {
		return nil, err
	}
	return s.Generar(ctx, documentoID)
}
