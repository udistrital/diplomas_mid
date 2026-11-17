package services

import (
	"errors"
	"testing"
)

func TestValidarReversionFirma(t *testing.T) {
	firma := func(id, cargo, documento int64, activo bool) firmaDocumentoResponse {
		return firmaDocumentoResponse{ID: id, RolFirmanteID: cargo, DocumentoIdentidad: documento, Activo: activo}
	}
	for _, tc := range []struct {
		name   string
		orden  int
		firmas []firmaDocumentoResponse
		want   error
	}{
		{"propia sin posteriores", 1, []firmaDocumentoResponse{firma(1, 158, 123, true)}, nil},
		{"posterior inmediato", 1, []firmaDocumentoResponse{firma(1, 158, 123, true), firma(2, 270, 456, true)}, ErrInvalidState},
		{"posterior no inmediato", 1, []firmaDocumentoResponse{firma(1, 158, 123, true), firma(3, 106, 789, true)}, ErrInvalidState},
		{"tres posteriores", 1, []firmaDocumentoResponse{firma(1, 158, 123, true), firma(2, 270, 456, true), firma(3, 106, 789, true), firma(4, 100, 987, true)}, ErrInvalidState},
		{"secretaria general sin posteriores", 3, []firmaDocumentoResponse{firma(1, 158, 456, true), firma(3, 106, 123, true)}, nil},
		{"posterior inactiva", 1, []firmaDocumentoResponse{firma(1, 158, 123, true), firma(3, 106, 789, false)}, nil},
		{"otro titular", 1, []firmaDocumentoResponse{firma(1, 158, 456, true)}, ErrNoActiveRole},
		{"sin firma propia", 1, nil, ErrInvalidState},
		{"orden desconocido", 1, []firmaDocumentoResponse{firma(1, 158, 123, true), firma(5, 999, 789, true)}, ErrInvalidState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validarReversionFirma(tc.firmas, tc.orden, 123)
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
