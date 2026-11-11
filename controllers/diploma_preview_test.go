package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

// Detenerse antes del render evita dependencias PDF y escrituras reales:
// documento y diploma deben recibir la autorización de la vista previa.
func TestPreviewPropagaAuthorizationAlCRUD(t *testing.T) {
	llamadas := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamadas++
		if r.Header.Get("Authorization") != "Bearer test-preview" {
			t.Error("CRUD recibió petición sin autorización")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/documento_digital/2":
			fmt.Fprint(w, `{"id":2,"periodo_id":77,"activo":true}`)
		case "/v1/diploma_digital/":
			// Fallo controlado posterior a recuperar el documento.
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			t.Errorf("endpoint inesperado: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mock.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", mock.URL)
	app := beego.NewHttpSever()
	app.Router("/v1/documento_digital/:id/diploma/preview", &DiplomaPreviewController{}, "get:Generar")
	req := httptest.NewRequest(http.MethodGet, "/v1/documento_digital/2/diploma/preview", nil)
	req.Header.Set("Authorization", "Bearer test-preview")
	res := httptest.NewRecorder()
	app.Handlers.ServeHTTP(res, req)
	if llamadas != 2 || res.Code != http.StatusBadGateway {
		t.Fatalf("llamadas=%d HTTP=%d: no alcanzó consulta de diploma autorizada", llamadas, res.Code)
	}
}

func TestPrepararPreviewRechazaDocumentoDeOtraFacultad(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/facultad_secretaria/123":
			fmt.Fprint(w, `{"facultades":{"secretaria":[{"EMP_NRO_IDEN":"123","SEC_DEP_COD":"33","DEP_NOMBRE":"FACULTAD","SEC_NOMBRE":"Secretario","SEC_APELLIDO":"Prueba"}]}}`)
		case "/facultad_oikos_gedep/33":
			fmt.Fprint(w, `{"homologacion":{"id_oikos":"14","id_gedep":"33"}}`)
		case "/v1/documento_digital/2":
			fmt.Fprint(w, `{"id":2,"facultad_id":99,"activo":true}`)
		default:
			t.Errorf("no debe reservar ni modificar registros: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer mock.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", mock.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", mock.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", mock.URL)
	app := beego.NewHttpSever()
	app.Router("/v1/documento_digital/:id/diploma/preview", &DiplomaPreviewController{}, "post:Preparar")
	req := httptest.NewRequest(http.MethodPost, "/v1/documento_digital/2/diploma/preview", strings.NewReader(`{"documento_identidad_secretario":123}`))
	req.Header.Set("Authorization", "Bearer test-preview")
	res := httptest.NewRecorder()
	app.Handlers.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("HTTP %d: %s", res.Code, res.Body.String())
	}
}
