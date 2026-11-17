package services

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestNormalizarFirmaPNG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for x := 5; x < 15; x++ {
		src.SetNRGBA(x, 5, color.NRGBA{R: 25, G: 25, B: 25, A: 255})
	}

	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatalf("encode input png: %v", err)
	}

	outputBase64, err := normalizarFirmaPNG(base64.StdEncoding.EncodeToString(input.Bytes()))
	if err != nil {
		t.Fatalf("normalizarFirmaPNG returned error: %v", err)
	}

	outputBytes, err := base64.StdEncoding.DecodeString(outputBase64)
	if err != nil {
		t.Fatalf("output is not base64: %v", err)
	}
	outputImage, err := png.Decode(bytes.NewReader(outputBytes))
	if err != nil {
		t.Fatalf("output is not png: %v", err)
	}

	if outputImage.Bounds().Dx() == 0 || outputImage.Bounds().Dy() == 0 {
		t.Fatal("output image is empty")
	}
}

func TestBuildDocumentoFirmaPayloadPreservaDatos(t *testing.T) {
	firmante := &FirmanteActivo{DocumentoIdentidad: 123, Nombre: "Firmante", Cargo: "Decano", CargoID: 270, Rol: "decano"}
	for _, tc := range []struct{ nombre, expected string }{
		{"", "firma_diploma.png"}, {"rubrica", "rubrica.png"}, {"rubrica.png", "rubrica.png"}, {"rubrica.jpg", "rubrica.jpg.png"},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			input := &SubirFirmaFirmanteInput{IdTipoDocumento: 42, Nombre: tc.nombre, Descripcion: "Descripción original", Metadatos: map[string]interface{}{"custom": "valor"}}
			payload := buildDocumentoFirmaPayload(input, firmante, "normalized-base64")
			if payload.Nombre != tc.expected || payload.IdTipoDocumento != 42 || payload.Descripcion != input.Descripcion || payload.File != "normalized-base64" {
				t.Fatalf("unexpected payload: name=%s type=%d", payload.Nombre, payload.IdTipoDocumento)
			}
			if payload.Metadatos["custom"] != "valor" || payload.Metadatos["firmante"] != firmante.Nombre || payload.Metadatos["cargo"] != firmante.Cargo || payload.Metadatos["cargo_id"] != firmante.CargoID {
				t.Fatal("metadata changed")
			}
			if len(input.Metadatos) != 1 || input.Nombre != tc.nombre {
				t.Fatal("input mutated")
			}
		})
	}
}
