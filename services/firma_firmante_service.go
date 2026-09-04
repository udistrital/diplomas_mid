package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/gift"
	"github.com/udistrital/utils_oas/request"
)

const (
	idTipoDocumentoFirmaDiploma int64 = 38
	signatureProcessorVersion         = "v17-gift-black-ink-clean"

	// Limites de seguridad para evitar consumo excesivo de memoria/CPU.
	maxSignatureImageBytes = 8 * 1024 * 1024 // 8 MiB decodificados
	maxSignatureWidth      = 4096
	maxSignatureHeight     = 4096
	maxSignaturePixels     = 12_000_000

	// Umbrales generales, independientes del color de la tinta.
	//
	// chromaDistance mide cuanto cambia el color respecto al fondo ignorando
	// en gran medida cambios uniformes de iluminacion.
	strongChromaDistance      = 48
	weakChromaDistance        = 24
	colorStrongChromaDistance = 34
	colorWeakChromaDistance   = 12

	// localDarkness mide cuanto mas oscuro es el pixel respecto a su fondo local.
	// Permite detectar tinta negra/gris sin depender de saturacion o matiz.
	strongLocalDarkness = 78
	weakLocalDarkness   = 40

	// Rangos usados para producir una mascara suave de tinta.
	// El alpha no se cuantiza: se mantiene continuo para evitar bordes pixelados.
	chromaAlphaFullAt      = 110
	colorChromaAlphaFullAt = 82
	darkAlphaFullAt        = 135

	// Curva de alpha para aspecto analogico. Los valores muy debiles se descartan,
	// y la transicion hasta tinta opaca usa smoothstep para conservar antialiasing
	// sin dejar un halo gris alrededor del trazo.
	analogAlphaCutoff   = 36
	analogAlphaOpaqueAt = 190

	// Acabado final de alta calidad. Se escala y filtra con gift para suavizar
	// escalones sin convertir la firma en un trazo digital duro.
	preferredOutputScale = 4
	maxOutputWidth       = 2400
	maxOutputHeight      = 1800
	analogBlurSigma      = 0.18
	analogSharpenSigma   = 0.65
	analogSharpenAmount  = 0.08

	signatureInkR = 0
	signatureInkG = 0
	signatureInkB = 0

	colorInkMinStrongPixels = 30
	colorInkMinPixelRatio   = 1500

	// Los píxeles de alpha bajo solo se conservan cuando tienen apoyo espacial.
	// Esto limpia halos y puntos aislados sin tocar el centro fuerte del trazo.
	isolatedPixelAlphaMax    = 110
	isolatedPixelNeighborMin = 2
	isolatedNeighborAlphaMin = 25

	// Radio adaptativo del fondo local. Se ajusta al tamano de la imagen.
	minLocalBackgroundRadius = 12
	maxLocalBackgroundRadius = 36

	// Padding final alrededor de la firma recortada.
	signatureCropPadding = 12
)

type FirmaFirmanteService struct{}

type inkBand struct {
	minY   int
	maxY   int
	minX   int
	maxX   int
	pixels int
}

func (s FirmaFirmanteService) SubirFirma(ctx context.Context, input *SubirFirmaFirmanteInput) (*SubirFirmaFirmanteResult, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.DocumentoIdentidadFirmante <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	if strings.TrimSpace(input.File) == "" {
		return nil, fmt.Errorf("%w: file is required", ErrInvalidInput)
	}

	firmanteActivo, err := FirmanteService{}.ConsultarRolActivo(
		ctx,
		input.DocumentoIdentidadFirmante,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	firmaNormalizada, err := normalizarFirmaPNG(input.File)
	if err != nil {
		return nil, err
	}

	// No modificamos input.File. El objeto recibido pertenece al caller.
	documentoPayload := []subirDocumentoFirmaRequest{
		buildDocumentoFirmaPayload(input, firmanteActivo, firmaNormalizada),
	}

	var documentoResponse interface{}
	status, err := request.PostWithContext(
		ctx,
		documentosCrudGestorDocumentalURL(),
		documentoPayload,
		&documentoResponse,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: documentos_crud gestor_documental status %d: %v",
			ErrExternalService,
			status,
			err,
		)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf(
			"%w: documentos_crud gestor_documental returned status %d",
			ErrExternalService,
			status,
		)
	}

	documento := unwrapDocumentoResponse(documentoResponse)
	enlaceFirma, err := extractString(
		documento,
		"Enlace", "enlace",
		"Id", "id",
		"UUID", "uuid",
		"Uid", "uid",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: documentos_crud gestor_documental response missing Enlace: %v; response_shape=%s",
			ErrExternalService,
			err,
			describeResponseShape(documentoResponse),
		)
	}

	crudPayload := registrarFirmaFirmanteRequest{
		DocumentoIdentidad: firmanteActivo.DocumentoIdentidad,
		EnlaceFirma:        enlaceFirma,
		Activo:             true,
	}

	var firmaFirmante FirmaFirmanteCRUDResponse
	status, err = request.PostWithContext(
		ctx,
		diplomasCrudURL()+"/firma_firmante/",
		crudPayload,
		&firmaFirmante,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: diplomas_crud firma_firmante status %d: %v",
			ErrExternalService,
			status,
			err,
		)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf(
			"%w: diplomas_crud firma_firmante returned status %d",
			ErrExternalService,
			status,
		)
	}

	return &SubirFirmaFirmanteResult{
		Firmante:      *firmanteActivo,
		EnlaceFirma:   enlaceFirma,
		Documento:     documento,
		FirmaFirmante: firmaFirmante,
	}, nil
}

func buildDocumentoFirmaPayload(
	input *SubirFirmaFirmanteInput,
	firmante *FirmanteActivo,
	firmaNormalizada string,
) subirDocumentoFirmaRequest {
	idTipoDocumento := input.IdTipoDocumento
	if idTipoDocumento == 0 {
		idTipoDocumento = idTipoDocumentoFirmaDiploma
	}

	nombre := strings.TrimSpace(input.Nombre)
	if nombre == "" {
		nombre = "firma_diploma"
	}

	descripcion := strings.TrimSpace(input.Descripcion)
	if descripcion == "" {
		descripcion = strconv.FormatInt(firmante.DocumentoIdentidad, 10)
	}

	metadatos := cloneMetadatos(input.Metadatos)
	metadatos["firmante"] = firmante.Nombre
	metadatos["documento_identidad"] = firmante.DocumentoIdentidad
	metadatos["cargo"] = firmante.Cargo
	metadatos["cargo_id"] = firmante.CargoID
	metadatos["rol"] = firmante.Rol

	return subirDocumentoFirmaRequest{
		IdTipoDocumento: idTipoDocumento,
		Nombre:          nombre,
		Metadatos:       metadatos,
		Descripcion:     descripcion,
		File:            firmaNormalizada,
	}
}

func unwrapDocumentoResponse(response interface{}) map[string]interface{} {
	if data, ok := response.(map[string]interface{}); ok {
		for _, key := range []string{
			"Body", "body",
			"res", "Res",
			"data", "Data",
			"documento", "Documento",
		} {
			if value, exists := data[key]; exists {
				return unwrapDocumentoResponse(value)
			}
		}
		return data
	}

	if items, ok := response.([]interface{}); ok && len(items) > 0 {
		return unwrapDocumentoResponse(items[0])
	}

	return map[string]interface{}{}
}

func describeResponseShape(response interface{}) string {
	switch typed := response.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key, value := range typed {
			if key == "file" || key == "File" {
				keys = append(keys, key+":<redacted>")
				continue
			}
			keys = append(keys, key+":"+describeResponseShape(value))
		}
		sort.Strings(keys)
		return "{" + strings.Join(keys, ",") + "}"

	case []interface{}:
		if len(typed) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[len=%d first=%s]", len(typed), describeResponseShape(typed[0]))

	case string:
		return "string"
	case float64:
		return "number"
	case json.Number:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", response)
	}
}

func extractString(data map[string]interface{}, keys ...string) (string, error) {
	for _, key := range keys {
		value, ok := data[key]
		if !ok || value == nil {
			continue
		}

		switch typed := value.(type) {
		case string:
			value := strings.TrimSpace(typed)
			if value != "" {
				return value, nil
			}

		case json.Number:
			value := strings.TrimSpace(typed.String())
			if value != "" {
				return value, nil
			}

		case float64:
			// encoding/json suele decodificar numeros de interface{} como float64.
			// Solo aceptamos valores enteros para evitar URLs o IDs corruptos.
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10), nil
			}
		}
	}

	return "", fmt.Errorf("valid string/identifier not found in keys: %v", keys)
}

func normalizarFirmaPNG(fileBase64 string) (string, error) {
	log.Printf("[firma] signature_processor=%s", signatureProcessorVersion)
	rawBase64 := stripDataURI(fileBase64)
	if rawBase64 == "" {
		return "", fmt.Errorf("%w: file is empty", ErrInvalidInput)
	}

	// Rechazamos antes de decodificar para evitar reservar memoria con Base64 gigantes.
	maxEncodedLen := base64.StdEncoding.EncodedLen(maxSignatureImageBytes) + 4
	if len(rawBase64) > maxEncodedLen {
		return "", fmt.Errorf(
			"%w: image exceeds maximum allowed size of %d MiB",
			ErrInvalidInput,
			maxSignatureImageBytes/(1024*1024),
		)
	}

	imageBytes, err := decodeBase64Image(rawBase64)
	if err != nil {
		return "", fmt.Errorf("%w: file must be valid base64", ErrInvalidInput)
	}
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("%w: decoded image is empty", ErrInvalidInput)
	}
	if len(imageBytes) > maxSignatureImageBytes {
		return "", fmt.Errorf(
			"%w: image exceeds maximum allowed size of %d MiB",
			ErrInvalidInput,
			maxSignatureImageBytes/(1024*1024),
		)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("%w: file must be a valid png or jpg image", ErrInvalidInput)
	}

	if format != "png" && format != "jpeg" {
		return "", fmt.Errorf("%w: only png and jpg images are supported", ErrInvalidInput)
	}

	if err := validateImageDimensions(config.Width, config.Height); err != nil {
		return "", err
	}

	src, decodedFormat, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return "", fmt.Errorf("%w: file must be a valid png or jpg image", ErrInvalidInput)
	}
	if decodedFormat != "png" && decodedFormat != "jpeg" {
		return "", fmt.Errorf("%w: only png and jpg images are supported", ErrInvalidInput)
	}
	src = preprocessSignatureSource(src)

	bounds := src.Bounds()
	if err := validateImageDimensions(bounds.Dx(), bounds.Dy()); err != nil {
		return "", err
	}

	background := estimateBackgroundModel(src)
	localRadius := adaptiveLocalBackgroundRadius(bounds.Dx(), bounds.Dy())
	lumaIntegral, integralStride := buildLumaIntegral(src, background.Luma)
	colorInkMode := detectColorInkSignature(
		src,
		background,
		lumaIntegral,
		integralStride,
		localRadius,
	)

	// V12 no depende del color de la tinta.
	//
	// Se combinan dos evidencias:
	//   1. distancia cromatica respecto al fondo global, para tinta de color;
	//   2. oscuridad respecto al fondo local, para tinta negra/gris y para
	//      compensar sombras o iluminacion no uniforme del papel.
	//
	// La mascara usa histeresis: los pixeles debiles solo sobreviven cuando estan
	// conectados a pixeles claramente identificados como tinta.
	normalized, maskStats := buildAdaptiveSignatureMask(
		src,
		background,
		lumaIntegral,
		integralStride,
		localRadius,
		colorInkMode,
	)
	log.Printf(
		"[firma] mask strong=%d weak=%d kept=%d color_ink=%t bg_rgb=(%d,%d,%d) bg_luma=%d local_radius=%d size=%dx%d",
		maskStats.StrongPixels,
		maskStats.WeakPixels,
		maskStats.KeptPixels,
		colorInkMode,
		background.R,
		background.G,
		background.B,
		background.Luma,
		localRadius,
		bounds.Dx(),
		bounds.Dy(),
	)

	// No conservamos solamente el componente conectado mas grande. Una firma real
	// puede contener puntos, subrayados o trazos separados. La banda principal
	// permite descartar elementos claramente separados verticalmente, por ejemplo
	// texto impreso debajo de la firma.
	signatureOnly := keepPrimarySignatureBand(normalized)

	// Primero quitamos únicamente píxeles débiles aislados. Los centros fuertes
	// del trazo se conservan aunque formen líneas muy finas.
	isolatedCleaned := removeIsolatedWeakPixels(signatureOnly)

	// Después se eliminan componentes diminutos completos. No se aplica erosión
	// ni dilatación: la geometría original de la firma se conserva.
	cleanupMinPixels := adaptiveColorAgnosticNoiseThreshold(bounds.Dx(), bounds.Dy())
	if colorInkMode {
		cleanupMinPixels = adaptiveColorInkNoiseThreshold(bounds.Dx(), bounds.Dy())
	}
	cleaned := removeSmallNoiseComponents(isolatedCleaned, cleanupMinPixels)
	closed := closeSmallAlphaGaps(cleaned, 1)

	log.Printf(
		"[firma] cleanup min_component=%d before=%d isolated=%d after=%d closed=%d",
		cleanupMinPixels,
		countVisiblePixels(signatureOnly),
		countVisiblePixels(isolatedCleaned),
		countVisiblePixels(cleaned),
		countVisiblePixels(closed),
	)

	cropped := cropTransparent(closed)
	if cropped.Bounds().Dx() == 0 || cropped.Bounds().Dy() == 0 {
		return "", fmt.Errorf(
			"%w: file does not contain visible signature strokes",
			ErrInvalidInput,
		)
	}

	// No usamos nearest-neighbor ni cuantizacion de alpha. El acabado con gift
	// interpola el trazo y aplica filtros suaves para aspecto mas analogico.
	scale := adaptiveOutputScale(cropped.Bounds().Dx(), cropped.Bounds().Dy())
	finalImage := applyAnalogSignatureFilters(cropped, scale)
	finalImage = cleanupResampledAlpha(finalImage)

	log.Printf(
		"[firma] output_filter=gift_lanczos blur=%.2f unsharp=(%.2f,%.2f) scale=%dx original=%dx%d final=%dx%d",
		analogBlurSigma,
		analogSharpenSigma,
		analogSharpenAmount,
		scale,
		cropped.Bounds().Dx(),
		cropped.Bounds().Dy(),
		finalImage.Bounds().Dx(),
		finalImage.Bounds().Dy(),
	)

	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := encoder.Encode(&output, finalImage); err != nil {
		return "", fmt.Errorf("encode normalized signature: %w", err)
	}

	return base64.StdEncoding.EncodeToString(output.Bytes()), nil
}

func decodeBase64Image(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}

	// Algunos clientes eliminan el padding '=' del Base64.
	decoded, rawErr := base64.RawStdEncoding.DecodeString(value)
	if rawErr == nil {
		return decoded, nil
	}

	return nil, err
}

func validateImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: image has invalid dimensions", ErrInvalidInput)
	}
	if width > maxSignatureWidth || height > maxSignatureHeight {
		return fmt.Errorf(
			"%w: image dimensions exceed maximum %dx%d",
			ErrInvalidInput,
			maxSignatureWidth,
			maxSignatureHeight,
		)
	}

	pixels := int64(width) * int64(height)
	if pixels > maxSignaturePixels {
		return fmt.Errorf(
			"%w: image contains too many pixels; maximum is %d",
			ErrInvalidInput,
			maxSignaturePixels,
		)
	}

	return nil
}

func stripDataURI(fileBase64 string) string {
	value := strings.TrimSpace(fileBase64)
	if comma := strings.Index(value, ","); strings.HasPrefix(value, "data:") && comma >= 0 {
		value = value[comma+1:]
	}

	// Algunos clientes agregan espacios o saltos de linea al Base64.
	value = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, value)

	return value
}

type backgroundModel struct {
	R    int
	G    int
	B    int
	Luma int
}

type signatureMaskStats struct {
	StrongPixels int
	WeakPixels   int
	KeptPixels   int
}

type signaturePixelMetrics struct {
	ChromaDistance int
	ColorDistance  int
	LocalDarkness  int
}

// estimateBackgroundModel obtiene un color de fondo robusto usando la mediana
// de una franja del borde de la imagen. La mediana tolera que una pequena parte
// del borde contenga tinta, ruido o sombras.
func estimateBackgroundModel(img image.Image) backgroundModel {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= 0 || height <= 0 {
		return backgroundModel{R: 255, G: 255, B: 255, Luma: 255}
	}

	border := clampInt(minInt(width, height)/20, 4, 32)

	step := 1
	totalPixels := int64(width) * int64(height)
	if totalPixels > 250_000 {
		step = 2
	}
	if totalPixels > 1_000_000 {
		step = 4
	}
	if totalPixels > 4_000_000 {
		step = 8
	}

	rs := make([]int, 0, (width+height)*2)
	gs := make([]int, 0, (width+height)*2)
	bs := make([]int, 0, (width+height)*2)

	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			tx := x - bounds.Min.X
			ty := y - bounds.Min.Y

			if tx >= border &&
				tx < width-border &&
				ty >= border &&
				ty < height-border {
				continue
			}

			p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if p.A == 0 {
				continue
			}

			rs = append(rs, int(p.R))
			gs = append(gs, int(p.G))
			bs = append(bs, int(p.B))
		}
	}

	if len(rs) == 0 {
		return backgroundModel{R: 255, G: 255, B: 255, Luma: 255}
	}

	sort.Ints(rs)
	sort.Ints(gs)
	sort.Ints(bs)

	middle := len(rs) / 2
	r := rs[middle]
	g := gs[middle]
	b := bs[middle]
	luma := rgbLuma(r, g, b)

	return backgroundModel{
		R:    r,
		G:    g,
		B:    b,
		Luma: luma,
	}
}

// buildLumaIntegral crea una imagen integral de luminancia. Permite calcular el
// promedio de una ventana local en O(1) por pixel sin realizar desenfoques
// costosos ni crear archivos intermedios BMP.
func buildLumaIntegral(img image.Image, transparentBackgroundLuma int) ([]uint32, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	stride := width + 1

	integral := make([]uint32, (height+1)*stride)

	for y := 0; y < height; y++ {
		var rowSum uint32

		for x := 0; x < width; x++ {
			p := color.NRGBAModel.Convert(
				img.At(bounds.Min.X+x, bounds.Min.Y+y),
			).(color.NRGBA)

			luma := transparentBackgroundLuma
			if p.A != 0 {
				luma = rgbLuma(int(p.R), int(p.G), int(p.B))
			}
			rowSum += uint32(clampInt(luma, 0, 255))

			integral[(y+1)*stride+(x+1)] =
				integral[y*stride+(x+1)] + rowSum
		}
	}

	return integral, stride
}

func adaptiveLocalBackgroundRadius(width, height int) int {
	if width <= 0 || height <= 0 {
		return minLocalBackgroundRadius
	}

	return clampInt(
		minInt(width, height)/18,
		minLocalBackgroundRadius,
		maxLocalBackgroundRadius,
	)
}

func localMeanLuma(
	integral []uint32,
	stride int,
	width int,
	height int,
	x int,
	y int,
	radius int,
) int {
	if len(integral) == 0 || stride <= 0 || width <= 0 || height <= 0 {
		return 255
	}

	left := maxInt(0, x-radius)
	top := maxInt(0, y-radius)
	right := minInt(width-1, x+radius)
	bottom := minInt(height-1, y+radius)

	x1 := left
	y1 := top
	x2 := right + 1
	y2 := bottom + 1

	sum := int64(integral[y2*stride+x2]) -
		int64(integral[y1*stride+x2]) -
		int64(integral[y2*stride+x1]) +
		int64(integral[y1*stride+x1])

	area := int64((right - left + 1) * (bottom - top + 1))
	if area <= 0 {
		return 255
	}

	return clampInt(int(sum/area), 0, 255)
}

func preprocessSignatureSource(src image.Image) image.Image {
	bounds := src.Bounds()
	if bounds.Dx() < 80 || bounds.Dy() < 80 {
		return src
	}

	filter := gift.New(
		gift.Median(3, false),
		gift.GaussianBlur(0.25),
	)
	dst := image.NewNRGBA(filter.Bounds(src.Bounds()))
	filter.Draw(dst, src)
	return dst
}

// buildAdaptiveSignatureMask crea una mascara independiente del color.
//
// Evidencia cromatica:
// compara las diferencias R-G, G-B y B-R del pixel con las del fondo. Como un
// cambio uniforme de iluminacion afecta de forma parecida a R, G y B, esta
// medida es mucho menos sensible a sombras que una distancia RGB pura.
//
// Evidencia acromatica:
// compara la luminancia del pixel con un fondo local calculado mediante una
// ventana grande. Esto permite detectar tinta negra o gris.
//
// Se aplican dos umbrales: fuerte y debil. Un pixel debil solo se conserva si
// esta conectado a una semilla fuerte, evitando gran parte del ruido aislado.
func buildAdaptiveSignatureMask(
	src image.Image,
	background backgroundModel,
	lumaIntegral []uint32,
	integralStride int,
	localRadius int,
	colorInkMode bool,
) (*image.NRGBA, signatureMaskStats) {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	output := image.NewNRGBA(image.Rect(0, 0, width, height))

	if width == 0 || height == 0 {
		return output, signatureMaskStats{}
	}

	total := width * height

	// Guardamos primero la evidencia por píxel. Luego suavizamos únicamente el
	// mapa de confianza, no la imagen final. De esta forma se reduce ruido JPEG
	// y variaciones del papel sin deformar la firma.
	chromaMap := make([]uint16, total)
	darknessMap := make([]uint16, total)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x

			localLuma := localMeanLuma(
				lumaIntegral,
				integralStride,
				width,
				height,
				x,
				y,
				localRadius,
			)

			metrics := calculateSignaturePixelMetrics(
				src.At(bounds.Min.X+x, bounds.Min.Y+y),
				background,
				localLuma,
			)

			chromaMap[index] = uint16(clampInt(metrics.ChromaDistance, 0, 65535))
			darknessMap[index] = uint16(clampInt(metrics.LocalDarkness, 0, 65535))
		}
	}

	smoothedChroma := smoothConfidenceMap3x3(chromaMap, width, height)
	smoothedDarkness := smoothConfidenceMap3x3(darknessMap, width, height)

	weak := make([]bool, total)
	queued := make([]bool, total)
	queue := make([]int32, 0, minInt(total, 4096))

	stats := signatureMaskStats{}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			// El centro fuerte se decide con la medición original para no perder
			// trazos finos. El umbral débil usa el mapa suavizado y exige soporte
			// de los vecinos, por lo que halos y puntos aislados dejan de propagarse.
			strongChromaThreshold := strongChromaDistance
			weakChromaThreshold := weakChromaDistance
			if colorInkMode {
				strongChromaThreshold = colorStrongChromaDistance
				weakChromaThreshold = colorWeakChromaDistance
			}

			isStrong := int(chromaMap[index]) >= strongChromaThreshold
			if !colorInkMode {
				isStrong = isStrong || int(darknessMap[index]) >= strongLocalDarkness
			}

			isWeak := isStrong || int(smoothedChroma[index]) >= weakChromaThreshold
			if !colorInkMode {
				isWeak = isWeak || int(smoothedDarkness[index]) >= weakLocalDarkness
			}

			if isWeak {
				weak[index] = true
				stats.WeakPixels++
			}

			if isStrong {
				stats.StrongPixels++
				if !queued[index] {
					queued[index] = true
					queue = append(queue, int32(index))
				}
			}
		}
	}

	directions := [...]image.Point{
		{X: -1, Y: -1},
		{X: 0, Y: -1},
		{X: 1, Y: -1},
		{X: -1, Y: 0},
		{X: 1, Y: 0},
		{X: -1, Y: 1},
		{X: 0, Y: 1},
		{X: 1, Y: 1},
	}

	for head := 0; head < len(queue); head++ {
		index := int(queue[head])
		if index < 0 || index >= total || !weak[index] {
			continue
		}

		x := index % width
		y := index / width

		localLuma := localMeanLuma(
			lumaIntegral,
			integralStride,
			width,
			height,
			x,
			y,
			localRadius,
		)

		metrics := calculateSignaturePixelMetrics(
			src.At(bounds.Min.X+x, bounds.Min.Y+y),
			background,
			localLuma,
		)

		rendered := renderSignaturePixel(
			src.At(bounds.Min.X+x, bounds.Min.Y+y),
			metrics,
			colorInkMode,
		)
		if rendered.A != 0 {
			output.SetNRGBA(x, y, rendered)
			stats.KeptPixels++
		}

		for _, direction := range directions {
			nx := x + direction.X
			ny := y + direction.Y

			if nx < 0 || ny < 0 || nx >= width || ny >= height {
				continue
			}

			nextIndex := ny*width + nx
			if queued[nextIndex] || !weak[nextIndex] {
				continue
			}

			queued[nextIndex] = true
			queue = append(queue, int32(nextIndex))
		}
	}

	return output, stats
}

func detectColorInkSignature(
	src image.Image,
	background backgroundModel,
	lumaIntegral []uint32,
	integralStride int,
	localRadius int,
) bool {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return false
	}

	step := 1
	totalPixels := width * height
	if totalPixels > 250_000 {
		step = 2
	}
	if totalPixels > 1_000_000 {
		step = 4
	}

	strongColorPixels := 0
	requiredPixels := maxInt(colorInkMinStrongPixels, totalPixels/colorInkMinPixelRatio)

	for y := 0; y < height; y += step {
		for x := 0; x < width; x += step {
			localLuma := localMeanLuma(
				lumaIntegral,
				integralStride,
				width,
				height,
				x,
				y,
				localRadius,
			)
			metrics := calculateSignaturePixelMetrics(
				src.At(bounds.Min.X+x, bounds.Min.Y+y),
				background,
				localLuma,
			)
			if metrics.ChromaDistance >= strongChromaDistance {
				strongColorPixels += step * step
				if strongColorPixels >= requiredPixels {
					return true
				}
			}
		}
	}

	return false
}

// smoothConfidenceMap3x3 suaviza únicamente la evidencia de tinta. El centro
// recibe peso 4 y cada vecino peso 1, de modo que un trazo fino real conserva
// influencia mientras un único píxel ruidoso pierde fuerza.
func smoothConfidenceMap3x3(values []uint16, width, height int) []uint16 {
	if width <= 0 || height <= 0 || len(values) != width*height {
		return values
	}

	output := make([]uint16, len(values))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			sum := int(values[index]) * 4
			weight := 4

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}

					nx := x + dx
					ny := y + dy
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}

					sum += int(values[ny*width+nx])
					weight++
				}
			}

			if weight > 0 {
				output[index] = uint16(sum / weight)
			}
		}
	}

	return output
}

func calculateSignaturePixelMetrics(
	pixel color.Color,
	background backgroundModel,
	localBackgroundLuma int,
) signaturePixelMetrics {
	p := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	if p.A == 0 {
		return signaturePixelMetrics{}
	}

	r := int(p.R)
	g := int(p.G)
	b := int(p.B)

	// Distancia RGB global. Se conserva como diagnostico/evidencia secundaria,
	// aunque la mascara se apoya principalmente en cromaticidad + fondo local.
	dr := r - background.R
	dg := g - background.G
	db := b - background.B
	colorDistance := int(math.Sqrt(float64(dr*dr + dg*dg + db*db)))

	// Distancia cromatica respecto al fondo. El uso de diferencias entre canales
	// hace que una sombra gris uniforme tenga poca respuesta, mientras que tinta
	// roja, azul, verde, morada, etc. produzca una respuesta alta.
	bgRG := background.R - background.G
	bgGB := background.G - background.B
	bgBR := background.B - background.R

	pixelRG := r - g
	pixelGB := g - b
	pixelBR := b - r

	dRG := pixelRG - bgRG
	dGB := pixelGB - bgGB
	dBR := pixelBR - bgBR

	chromaDistance := int(math.Sqrt(float64(
		dRG*dRG + dGB*dGB + dBR*dBR,
	)))

	pixelLuma := rgbLuma(r, g, b)

	// Mezcla 75% del promedio local con 25% del fondo global. El promedio local
	// sigue sombras suaves del papel; el fondo global evita que el propio trazo
	// oscuro reduzca demasiado la referencia dentro de la ventana.
	blendedBackgroundLuma := (localBackgroundLuma*3 + background.Luma) / 4
	localDarkness := blendedBackgroundLuma - pixelLuma
	if localDarkness < 0 {
		localDarkness = 0
	}

	return signaturePixelMetrics{
		ChromaDistance: chromaDistance,
		ColorDistance:  colorDistance,
		LocalDarkness:  localDarkness,
	}
}

// renderSignaturePixel conserva el borde natural mediante alpha gradual y
// evita usar sombras del papel cuando ya se detecto una firma de tinta color.
func renderSignaturePixel(
	source color.Color,
	metrics signaturePixelMetrics,
	colorInkMode bool,
) color.NRGBA {
	p := color.NRGBAModel.Convert(source).(color.NRGBA)
	if p.A == 0 {
		return color.NRGBA{}
	}

	chromaLow := weakChromaDistance
	chromaHigh := chromaAlphaFullAt
	if colorInkMode {
		chromaLow = colorWeakChromaDistance
		chromaHigh = colorChromaAlphaFullAt
	}

	alphaByChroma := scaleToByte(metrics.ChromaDistance, chromaLow, chromaHigh)
	alphaByDarkness := scaleToByte(
		metrics.LocalDarkness,
		weakLocalDarkness,
		darkAlphaFullAt,
	)

	alpha := alphaByChroma
	if !colorInkMode {
		alpha = maxInt(alphaByChroma, alphaByDarkness)
	}

	// Respeta transparencia de entrada si el archivo original ya era PNG.
	alpha = alpha * int(p.A) / 255

	finalAlpha := analogSignatureAlpha(alpha)
	if finalAlpha == 0 {
		return color.NRGBA{}
	}

	return signatureInkPixel(finalAlpha)
}

// analogSignatureAlpha aplica una curva smoothstep entre el nivel de descarte
// y el punto opaco. A diferencia de la cuantizacion de V13, no crea escalones
// de 160/210/255 que luego se ven como dientes o bloques al ampliar.
func analogSignatureAlpha(alpha int) uint8 {
	alpha = clampInt(alpha, 0, 255)
	if alpha <= analogAlphaCutoff {
		return 0
	}
	if alpha >= analogAlphaOpaqueAt {
		return 255
	}

	t := float64(alpha-analogAlphaCutoff) /
		float64(analogAlphaOpaqueAt-analogAlphaCutoff)
	// smoothstep: 3t² - 2t³. Tiene derivada cero en los extremos, por lo que
	// la transicion visual entre fondo y tinta es suave y limpia.
	t = t * t * (3.0 - 2.0*t)
	return uint8(clampInt(int(t*255.0+0.5), 0, 255))
}

func adaptiveOutputScale(width, height int) int {
	if width <= 0 || height <= 0 {
		return 1
	}

	scale := preferredOutputScale
	for scale > 1 && (width*scale > maxOutputWidth || height*scale > maxOutputHeight) {
		scale--
	}
	if scale < 1 {
		return 1
	}
	return scale
}

func applyAnalogSignatureFilters(src *image.NRGBA, scale int) *image.NRGBA {
	if src == nil {
		return src
	}

	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return src
	}

	filters := make([]gift.Filter, 0, 3)
	if scale > 1 {
		filters = append(filters, gift.Resize(srcW*scale, srcH*scale, gift.LanczosResampling))
	}
	filters = append(filters,
		gift.GaussianBlur(analogBlurSigma),
		gift.UnsharpMask(analogSharpenSigma, analogSharpenAmount, 0),
	)

	filter := gift.New(filters...)
	dst := image.NewNRGBA(filter.Bounds(src.Bounds()))
	filter.Draw(dst, src)
	return dst
}

// cleanupResampledAlpha elimina exclusivamente residuos subvisibles generados
// por el lobulo negativo de Lanczos. No binariza ni cuantiza el borde.
func cleanupResampledAlpha(img *image.NRGBA) *image.NRGBA {
	if img == nil {
		return img
	}

	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.NRGBAAt(x, y)
			if p.A < 3 {
				continue
			}
			p.R = signatureInkR
			p.G = signatureInkG
			p.B = signatureInkB
			out.SetNRGBA(x, y, p)
		}
	}
	return out
}

func signatureInkPixel(alpha uint8) color.NRGBA {
	return color.NRGBA{
		R: signatureInkR,
		G: signatureInkG,
		B: signatureInkB,
		A: alpha,
	}
}

func closeSmallAlphaGaps(img *image.NRGBA, radius int) *image.NRGBA {
	if img == nil || radius <= 0 {
		return img
	}

	return erodeAlpha(dilateAlpha(img, radius), radius)
}

func dilateAlpha(img *image.NRGBA, radius int) *image.NRGBA {
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			maxAlpha := uint8(0)
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx := x + dx
					ny := y + dy
					if nx < bounds.Min.X || nx >= bounds.Max.X ||
						ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}
					alpha := img.NRGBAAt(nx, ny).A
					if alpha > maxAlpha {
						maxAlpha = alpha
					}
				}
			}
			if maxAlpha != 0 {
				out.SetNRGBA(x, y, signatureInkPixel(maxAlpha))
			}
		}
	}

	return out
}

func erodeAlpha(img *image.NRGBA, radius int) *image.NRGBA {
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			minAlpha := uint8(255)
			hasAlpha := false
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx := x + dx
					ny := y + dy
					if nx < bounds.Min.X || nx >= bounds.Max.X ||
						ny < bounds.Min.Y || ny >= bounds.Max.Y {
						minAlpha = 0
						continue
					}
					alpha := img.NRGBAAt(nx, ny).A
					if alpha != 0 {
						hasAlpha = true
					}
					if alpha < minAlpha {
						minAlpha = alpha
					}
				}
			}
			if hasAlpha && minAlpha != 0 {
				out.SetNRGBA(x, y, signatureInkPixel(minAlpha))
			}
		}
	}

	return out
}

func scaleToByte(value, low, high int) int {
	if high <= low {
		if value >= high {
			return 255
		}
		return 0
	}
	if value <= low {
		return 0
	}
	if value >= high {
		return 255
	}
	return (value - low) * 255 / (high - low)
}

func rgbLuma(r, g, b int) int {
	return clampInt((r*299+g*587+b*114)/1000, 0, 255)
}

// removeIsolatedWeakPixels elimina únicamente píxeles de baja opacidad que no
// tienen soporte suficiente alrededor. Un píxel fuerte se conserva siempre,
// incluso si pertenece a un trazo extremadamente fino.
func removeIsolatedWeakPixels(img *image.NRGBA) *image.NRGBA {
	if img == nil {
		return img
	}

	bounds := img.Bounds()
	output := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.NRGBAAt(x, y)
			if pixel.A == 0 {
				continue
			}

			// Los píxeles fuertes pertenecen al centro del trazo y nunca se quitan
			// en esta etapa.
			if int(pixel.A) >= isolatedPixelAlphaMax {
				output.SetNRGBA(x, y, pixel)
				continue
			}

			neighbors := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}

					nx := x + dx
					ny := y + dy
					if nx < bounds.Min.X || nx >= bounds.Max.X ||
						ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}

					if int(img.NRGBAAt(nx, ny).A) >= isolatedNeighborAlphaMin {
						neighbors++
					}
				}
			}

			if neighbors < isolatedPixelNeighborMin {
				continue
			}

			output.SetNRGBA(x, y, pixel)
		}
	}

	return output
}

func adaptiveColorAgnosticNoiseThreshold(width, height int) int {
	if width <= 0 || height <= 0 {
		return 3
	}

	// Se mantiene bajo a proposito: puntos y trazos cortos pueden ser partes
	// legitimas de una firma. La histeresis ya realiza la mayor parte de la
	// limpieza antes de esta etapa.
	return clampInt((width*height)/25000, 3, 12)
}

func adaptiveColorInkNoiseThreshold(width, height int) int {
	if width <= 0 || height <= 0 {
		return 18
	}

	return clampInt((width*height)/12000, 18, 40)
}

func keepPrimarySignatureBand(img *image.NRGBA) *image.NRGBA {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width == 0 || height == 0 {
		return image.NewNRGBA(bounds)
	}

	// Se toleran pequenos huecos verticales dentro de la misma firma.
	// Para una imagen de ~500 px de alto resulta ~12 px.
	gapTolerance := clampInt(height/40, 6, 18)

	bands := make([]inkBand, 0, 4)
	var current *inkBand
	lastOccupiedY := -1

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		rowPixels := 0
		rowMinX := bounds.Max.X
		rowMaxX := bounds.Min.X

		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.NRGBAAt(x, y).A == 0 {
				continue
			}

			rowPixels++
			if x < rowMinX {
				rowMinX = x
			}
			if x > rowMaxX {
				rowMaxX = x
			}
		}

		if rowPixels == 0 {
			continue
		}

		if current == nil || (lastOccupiedY >= 0 && y-lastOccupiedY > gapTolerance) {
			if current != nil {
				bands = append(bands, *current)
			}
			current = &inkBand{
				minY:   y,
				maxY:   y,
				minX:   rowMinX,
				maxX:   rowMaxX,
				pixels: rowPixels,
			}
		} else {
			current.maxY = y
			if rowMinX < current.minX {
				current.minX = rowMinX
			}
			if rowMaxX > current.maxX {
				current.maxX = rowMaxX
			}
			current.pixels += rowPixels
		}

		lastOccupiedY = y
	}

	if current != nil {
		bands = append(bands, *current)
	}

	if len(bands) == 0 {
		return image.NewNRGBA(bounds)
	}

	bestIndex := 0
	bestScore := bandScore(bands[0])
	for i := 1; i < len(bands); i++ {
		score := bandScore(bands[i])
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	best := bands[bestIndex]

	// Margen muy pequeno: ayuda a no cortar anti-aliasing en los bordes,
	// pero no une una linea de texto claramente separada debajo.
	verticalMargin := clampInt(height/200, 2, 6)
	horizontalMargin := clampInt(width/100, 2, 10)

	minY := maxInt(bounds.Min.Y, best.minY-verticalMargin)
	maxY := minInt(bounds.Max.Y-1, best.maxY+verticalMargin)
	minX := maxInt(bounds.Min.X, best.minX-horizontalMargin)
	maxX := minInt(bounds.Max.X-1, best.maxX+horizontalMargin)

	output := image.NewNRGBA(bounds)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			pixel := img.NRGBAAt(x, y)
			if pixel.A == 0 {
				continue
			}
			output.SetNRGBA(x, y, pixel)
		}
	}

	return output
}

func bandScore(band inkBand) int64 {
	width := band.maxX - band.minX + 1
	height := band.maxY - band.minY + 1
	if width <= 0 || height <= 0 || band.pixels <= 0 {
		return 0
	}

	// Favorece regiones con bastante tinta y gran extension espacial.
	// Una firma manuscrita suele ocupar mucha mas area que un texto impreso
	// aislado debajo, aun cuando ambos tengan el mismo color.
	return int64(band.pixels) * int64(width+height)
}

func removeSmallNoiseComponents(img *image.NRGBA, minPixels int) *image.NRGBA {
	if minPixels <= 1 {
		return img
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width == 0 || height == 0 {
		return image.NewNRGBA(bounds)
	}

	visited := make([]bool, width*height)
	output := image.NewNRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			index := pointIndex(bounds, x, y)
			if visited[index] || img.NRGBAAt(x, y).A == 0 {
				continue
			}

			component := floodComponent(img, visited, x, y)
			if len(component) < minPixels {
				continue
			}

			for _, point := range component {
				output.SetNRGBA(point.X, point.Y, img.NRGBAAt(point.X, point.Y))
			}
		}
	}

	return output
}

func countVisiblePixels(img *image.NRGBA) int {
	if img == nil {
		return 0
	}

	bounds := img.Bounds()
	count := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.NRGBAAt(x, y).A != 0 {
				count++
			}
		}
	}
	return count
}

func floodComponent(img *image.NRGBA, visited []bool, startX, startY int) []image.Point {
	bounds := img.Bounds()
	queue := make([]image.Point, 0, 256)
	queue = append(queue, image.Point{X: startX, Y: startY})

	component := make([]image.Point, 0, 256)
	visited[pointIndex(bounds, startX, startY)] = true

	directions := [...]image.Point{
		{X: -1, Y: -1},
		{X: 0, Y: -1},
		{X: 1, Y: -1},
		{X: -1, Y: 0},
		{X: 1, Y: 0},
		{X: -1, Y: 1},
		{X: 0, Y: 1},
		{X: 1, Y: 1},
	}

	for head := 0; head < len(queue); head++ {
		current := queue[head]
		component = append(component, current)

		for _, direction := range directions {
			x := current.X + direction.X
			y := current.Y + direction.Y
			if !image.Pt(x, y).In(bounds) {
				continue
			}

			index := pointIndex(bounds, x, y)
			if visited[index] || img.NRGBAAt(x, y).A == 0 {
				continue
			}

			visited[index] = true
			queue = append(queue, image.Point{X: x, Y: y})
		}
	}

	return component
}

func pointIndex(bounds image.Rectangle, x, y int) int {
	return (y-bounds.Min.Y)*bounds.Dx() + (x - bounds.Min.X)
}

func cropTransparent(img *image.NRGBA) *image.NRGBA {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.NRGBAAt(x, y).A == 0 {
				continue
			}

			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x >= maxX {
				maxX = x + 1
			}
			if y >= maxY {
				maxY = y + 1
			}
		}
	}

	if minX >= maxX || minY >= maxY {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}

	minX = maxInt(bounds.Min.X, minX-signatureCropPadding)
	minY = maxInt(bounds.Min.Y, minY-signatureCropPadding)
	maxX = minInt(bounds.Max.X, maxX+signatureCropPadding)
	maxY = minInt(bounds.Max.Y, maxY+signatureCropPadding)

	croppedBounds := image.Rect(0, 0, maxX-minX, maxY-minY)
	cropped := image.NewNRGBA(croppedBounds)
	draw.Draw(
		cropped,
		croppedBounds,
		img,
		image.Point{X: minX, Y: minY},
		draw.Src,
	)

	return cropped
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
