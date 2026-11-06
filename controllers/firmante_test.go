package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/udistrital/diplomas_mid/services"
)

func TestRolActivoRecuperaRubricaConAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status       int
		body         string
		expected     int
		hasSignature bool
	}{
		{"rúbrica guardada", 200, `[{"id":1,"documento_identidad":123,"enlace_firma":"mock-rubrica","activo":true}]`, 200, true},
		{"sin rúbrica", 200, `[]`, 200, false},
		{"error CRUD no es ausencia de rúbrica", 401, `{}`, 502, false},
		{"timeout de consulta rúbrica", 0, `{}`, 504, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookups := 0
			mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/supervisor_contrato":
					fmt.Fprint(w, `[{"Id":1,"Documento":123,"Nombre":"Secretario prueba","CargoId":{"Id":158,"Cargo":"SECRETARIO ACADEMICO"}}]`)
				case "/v1/firma_firmante/":
					lookups++
					if tc.status == 0 {
						<-r.Context().Done()
						return
					}
					if r.Header.Get("Authorization") != "Bearer test-rol" {
						w.WriteHeader(401)
						fmt.Fprint(w, `{}`)
						return
					}
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
				case "/v1/document/mock-rubrica":
					fmt.Fprint(w, `{"file":"cHJ1ZWJh"}`)
				default:
					t.Errorf("unexpected mock endpoint %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer mock.Close()
			for _, key := range []string{"AdministrativaAmazonAPIURL", "DiplomasCrudURL", "GestorDocumentalURL"} {
				old, _ := beego.AppConfig.String(key)
				beego.AppConfig.Set(key, mock.URL)
				k := key
				defer beego.AppConfig.Set(k, old)
			}
			app := beego.NewHttpSever()
			app.Router("/v1/firmantes/rol-activo", &FirmanteController{}, "get:RolActivo")
			req := httptest.NewRequest(http.MethodGet, "/v1/firmantes/rol-activo?documento_identidad=123", nil)
			req.Header.Set("Authorization", "Bearer test-rol")
			if tc.status == 0 {
				ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
				defer cancel()
				req = req.WithContext(ctx)
			}
			res := httptest.NewRecorder()
			app.Handlers.ServeHTTP(res, req)
			if res.Code != tc.expected {
				t.Fatalf("HTTP %d, expected %d", res.Code, tc.expected)
			}
			if res.Code == http.StatusGatewayTimeout {
				var body apiError
				if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Message != "Un servicio requerido no respondió a tiempo. Intente nuevamente." {
					t.Error("timeout no identificado")
				}
			}
			if lookups == 0 {
				t.Fatal("no consultó rúbrica")
			}
			if res.Code == 200 {
				var firmante services.FirmanteActivo
				if err := json.Unmarshal(res.Body.Bytes(), &firmante); err != nil {
					t.Fatal(err)
				}
				if (firmante.EnlaceFirma != "") != tc.hasSignature {
					t.Fatal("rúbrica guardada no reconocida")
				}
			}
		})
	}
}
