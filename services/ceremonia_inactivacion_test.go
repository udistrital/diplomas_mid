package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestInactivarCeremoniaRevisaTodasLasRelaciones(t *testing.T) {
	for _, tc := range []struct {
		nombre                                      string
		ocupada, inactiva, conflicto, falloConsulta bool
	}{
		{nombre: "sin asignados"},
		{nombre: "asignados en otra facultad", ocupada: true},
		{nombre: "asignados en relacion historica inactiva", ocupada: true, inactiva: true},
		{nombre: "asignacion concurrente rechazada por CRUD", conflicto: true},
		{nombre: "error consultando asignaciones", falloConsulta: true},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			escrituras := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					escrituras = append(escrituras, r.URL.Path)
					if tc.conflicto {
						http.Error(w, "ceremonia con estudiantes asignados", http.StatusConflict)
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				switch r.URL.Path {
				case "/facultad_secretaria/1023026828":
					w.Write([]byte(secretariaFacultadJSON()))
				case "/facultad_oikos_gedep/33":
					w.Write([]byte(homologacionGedepJSON()))
				case "/v1/ceremonia_facultad_programa/":
					if strings.Contains(r.URL.Query().Get("query"), "CeremoniaGrado__Id:99") {
						activa := "true"
						if tc.inactiva {
							activa = "false"
						}
						w.Write([]byte(`[{"id":100,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":99,"activo":true}},{"id":101,"facultad_id":15,"activo":` + activa + `,"ceremonia_grado":{"id":99,"activo":true}}]`))
					} else {
						w.Write([]byte(`[{"id":100,"facultad_id":14,"activo":true,"ceremonia_grado":{"id":99,"activo":true}}]`))
					}
				case "/v1/ceremonia_estudiante/":
					if tc.falloConsulta {
						http.Error(w, "error", http.StatusInternalServerError)
						return
					}
					if strings.Contains(r.URL.Query().Get("query"), "ceremonia_facultad_programa_id:101") && tc.ocupada {
						w.Write([]byte(`[{"id":1,"activo":true}]`))
					} else {
						w.Write([]byte(`[]`))
					}
				default:
					t.Errorf("request inesperada: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
			t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
			t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)
			err := (CeremoniaService{}).Eliminar(t.Context(), 99, &EliminarCeremoniaInput{DocumentoIdentidadSecretario: 1023026828})
			switch {
			case tc.ocupada:
				if !errors.Is(err, ErrInvalidState) || len(escrituras) != 0 {
					t.Fatalf("no debe escribir: err=%v escrituras=%v", err, escrituras)
				}
			case tc.conflicto:
				if !errors.Is(err, ErrInvalidState) || !reflect.DeepEqual(escrituras, []string{"/v1/ceremonia_grado/99"}) {
					t.Fatalf("conflicto no propagado: err=%v escrituras=%v", err, escrituras)
				}
			case tc.falloConsulta:
				if !errors.Is(err, ErrExternalService) || len(escrituras) != 0 {
					t.Fatalf("consulta fallida no debe escribir: err=%v escrituras=%v", err, escrituras)
				}
			default:
				if err != nil || !reflect.DeepEqual(escrituras, []string{"/v1/ceremonia_grado/99", "/v1/ceremonia_facultad_programa/100"}) {
					t.Fatalf("orden incorrecto: err=%v escrituras=%v", err, escrituras)
				}
			}
		})
	}
}
