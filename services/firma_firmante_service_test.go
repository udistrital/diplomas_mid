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
