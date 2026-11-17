package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReversionDirectaValidaFirmasAntesDeEscribir(t *testing.T) {
	for _, tc := range []struct {
		name, firmas             string
		crudStatus, want, writes int
	}{
		{"propia", `[{"id":1,"rol_firmante_id":158,"documento_identidad":123,"activo":true}]`, 200, 200, 1},
		{"posterior", `[{"id":1,"rol_firmante_id":158,"documento_identidad":123,"activo":true},{"id":2,"rol_firmante_id":270,"documento_identidad":456,"activo":true}]`, 200, 409, 0},
		{"posterior no inmediato", `[{"id":1,"rol_firmante_id":158,"documento_identidad":123,"activo":true},{"id":3,"rol_firmante_id":106,"documento_identidad":789,"activo":true}]`, 200, 409, 0},
		{"otro titular", `[{"id":1,"rol_firmante_id":158,"documento_identidad":456,"activo":true}]`, 200, 403, 0},
		{"cambio concurrente", `[{"id":1,"rol_firmante_id":158,"documento_identidad":123,"activo":true}]`, 409, 409, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					writes++
					if r.Method != http.MethodPost || r.URL.Path != "/v1/documento_digital/40/firma/revertir" {
						t.Errorf("unexpected write %s %s", r.Method, r.URL.Path)
					}
					if r.Header.Get("Authorization") != "Bearer test-reversion" {
						t.Error("authorization not propagated")
					}
					var body struct {
						Documento int64   `json:"documento_identidad"`
						Firmas    []int64 `json:"firmas_esperadas"`
						Estado    int64   `json:"estado_nuevo_id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Documento != 123 || len(body.Firmas) != 1 || body.Firmas[0] != 1 || body.Estado != 9 {
						t.Error("invalid reversal payload")
					}
					w.WriteHeader(tc.crudStatus)
					fmt.Fprint(w, `{"anuladas":1}`)
					return
				}
				switch r.URL.Path {
				case "/v1/supervisor_contrato":
					fmt.Fprint(w, `[{"Documento":123,"Nombre":"Firmante","CargoId":{"Id":158}}]`)
				case "/v1/firma_firmante/":
					fmt.Fprint(w, `[]`)
				case "/v1/documento_digital/40":
					fmt.Fprint(w, `{"id":40,"estado_documento_id":9}`)
				case "/v1/firma_documento/":
					fmt.Fprint(w, tc.firmas)
				case "/v1/parametro":
					fmt.Fprint(w, `[{"Id":9,"CodigoAbreviacion":"GENERADO","Activo":true}]`)
				default:
					t.Errorf("unexpected request %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			for _, key := range []string{"AdministrativaAmazonAPIURL", "DiplomasCrudURL", "ParametrosCrudURL"} {
				old, _ := beego.AppConfig.String(key)
				beego.AppConfig.Set(key, upstream.URL)
				defer beego.AppConfig.Set(key, old)
			}
			req := httptest.NewRequest(http.MethodDelete, "/diplomas/40/firma", bytes.NewBufferString(`{"documento_identidad_firmante":123}`))
			req.Header.Set("Authorization", "Bearer test-reversion")
			rec := httptest.NewRecorder()
			app := beego.NewHttpSever()
			app.Router("/diplomas/:id/firma", &FirmaDiplomaController{}, "delete:Anular")
			app.Handlers.ServeHTTP(rec, req)
			if rec.Code != tc.want || writes != tc.writes {
				t.Fatalf("status=%d writes=%d body=%s", rec.Code, writes, rec.Body.String())
			}
			if tc.want == 409 && !strings.Contains(rec.Body.String(), "posterior") {
				t.Fatalf("unclear conflict message: %s", rec.Body.String())
			}
		})
	}
}
