package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

func TestSubirFirmaPropagaAuthorizationAlCRUD(t *testing.T) {
	// Synthetic image and token; all external calls terminate at this local mock.
	img := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for x := 5; x < 15; x++ {
		img.SetNRGBA(x, 5, color.NRGBA{A: 255})
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}

	for _, token := range []string{"Bearer local-test-one", "Bearer local-test-two"} {
		t.Run(token, func(t *testing.T) {
			registered := 0
			stored := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path == "/v1/supervisor_contrato":
					fmt.Fprint(w, `[{"Id":1,"Documento":123,"Nombre":"Firmante de prueba","CargoId":{"Id":158,"Cargo":"SECRETARIO ACADEMICO"}}]`)
				case r.URL.Path == "/v1/firma_firmante/" && r.Method == http.MethodGet:
					fmt.Fprint(w, `[]`)
				case r.URL.Path == "/v1/gestor_documental" && r.Method == http.MethodPost:
					stored++
					if r.Header.Get("Authorization") != token {
						t.Error("gestor documental no recibió Authorization entrante")
					}
					fmt.Fprint(w, `[{"Enlace":"mock-document-id"}]`)
				case r.URL.Path == "/v1/firma_firmante/" && r.Method == http.MethodPost:
					if r.Header.Get("Authorization") != token {
						w.WriteHeader(http.StatusUnauthorized)
						fmt.Fprint(w, `{"message":"missing authorization"}`)
						return
					}
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["enlace_firma"] != "mock-document-id" || body["documento_identidad"] != float64(123) {
						t.Error("payload changed")
					}
					registered++
					fmt.Fprint(w, `{"id":1,"documento_identidad":123,"enlace_firma":"mock-document-id","activo":true}`)
				default:
					t.Errorf("unexpected mock request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			for _, key := range []string{"AdministrativaAmazonAPIURL", "DiplomasCrudURL", "DocumentosCrudURL"} {
				old, _ := beego.AppConfig.String(key)
				beego.AppConfig.Set(key, upstream.URL)
				k := key
				defer beego.AppConfig.Set(k, old)
			}
			payload, _ := json.Marshal(map[string]interface{}{"documento_identidad_firmante": 123, "file": base64.StdEncoding.EncodeToString(pngData.Bytes())})
			req := httptest.NewRequest(http.MethodPost, "/v1/firmantes/firma", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", token)
			res := httptest.NewRecorder()
			app := beego.NewHttpSever()
			app.Router("/v1/firmantes/firma", &FirmaFirmanteController{}, "post:SubirFirma")
			app.Handlers.ServeHTTP(res, req)
			if res.Code != http.StatusCreated {
				t.Fatalf("upload status %d, expected 201: %s", res.Code, res.Body.String())
			}
			if stored != 1 || registered != 1 {
				t.Fatalf("stored=%d registered=%d", stored, registered)
			}
		})
	}
}
