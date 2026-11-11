package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNombreArchivoDiploma(t *testing.T) {
	preview := &DiplomaPreviewResult{
		DocumentoDigital: documentoDigitalResponse{CodigoEstudiante: 20242781013},
		DiplomaDigital:   &diplomaDigitalResponse{FechaGrado: time.Date(2029, 11, 26, 0, 0, 0, 0, time.UTC)},
	}

	if got := nombreArchivoDiploma(preview); got != "20291125_Diploma_20242781013.pdf" {
		t.Fatalf("nombreArchivoDiploma() = %q", got)
	}
}

func TestNombreArchivoDiplomaUsaMismaFechaBogotaDelDocumento(t *testing.T) {
	fechaUTC := time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC)
	preview := &DiplomaPreviewResult{
		DocumentoDigital: documentoDigitalResponse{CodigoEstudiante: 20181016035},
		DiplomaDigital:   &diplomaDigitalResponse{FechaGrado: fechaUTC},
	}

	if fechaTexto := fechaCeremoniaBogota(fechaUTC); fechaTexto != "En la ciudad de Bogota a los 8 dias del mes de octubre de 2026" {
		t.Fatalf("fechaCeremoniaBogota() = %q", fechaTexto)
	}
	if got := nombreArchivoDiploma(preview); got != "20261008_Diploma_20181016035.pdf" {
		t.Fatalf("nombreArchivoDiploma() = %q", got)
	}
}

func TestReservaTempranaReutilizaRegistro(t *testing.T) {
	for _, existente := range []bool{true, false} {
		t.Run(map[bool]string{true: "ya reservado", false: "reserva pendiente"}[existente], func(t *testing.T) {
			reservas := 0
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/diploma_digital/":
					if existente {
						w.Write([]byte(`[{"id":600,"registro_grado":{"id":10,"numero_registro":42}}]`))
					} else {
						w.Write([]byte(`[{"id":600}]`))
					}
				case r.Method == http.MethodPost && r.URL.Path == "/v1/diploma_digital/600/registro":
					reservas++
					w.Write([]byte(`{"id":600,"registro_grado":{"id":10,"numero_registro":42}}`))
				default:
					t.Errorf("operación inesperada %s %s", r.Method, r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer mock.Close()
			t.Setenv("DIPLOMAS_CRUD_URL", mock.URL)
			err := asegurarRegistroDiplomaEnFecha(t.Context(), &documentoDigitalResponse{ID: 500}, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if (existente && reservas != 0) || (!existente && reservas != 1) {
				t.Fatalf("reservas inesperadas: %d", reservas)
			}
		})
	}
}
