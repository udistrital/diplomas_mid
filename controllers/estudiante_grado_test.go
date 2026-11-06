package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/udistrital/diplomas_mid/services"
)

func TestAprobadosPeriodo77Programa72PropagaAuthorization(t *testing.T) {
	const token = "Bearer local-test-aprobados"
	academicCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/periodo":
			fmt.Fprint(w, `{"Success":true,"Status":"200","Data":[{"Id":77,"Nombre":"2026-3","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true}]}`)
		case "/v1/solicitud-grado/paz-salvos/aprobados":
			if r.URL.Query().Get("periodo_id") != "77" {
				t.Error("period not forwarded")
			}
			fmt.Fprint(w, `{"Success":true,"Status":200,"Data":{"Solicitudes":[{"SolicitudId":2,"TerceroId":12,"CodigoEstudiante":"123","PeriodoId":77,"ProgramaAcademicoId":84,"DependenciaOikosId":72}],"Total":1}}`)
		case "/v2/dependencia_padre/":
			if r.URL.Query().Get("query") != "HijaId:72" {
				t.Error("academic ID used instead of Oikos")
			}
			fmt.Fprint(w, `[{"Activo":true,"HijaId":{"Id":72,"Activo":true},"PadreId":{"Id":14,"Nombre":"FACULTAD DE INGENIERIA","Activo":true}}]`)
		case "/v1/estudiantes/datos-diploma/123":
			academicCalls++
			if r.Header.Get("Authorization") != token {
				w.WriteHeader(401)
				fmt.Fprint(w, `{}`)
				return
			}
			fmt.Fprint(w, `{"nombre":"Estudiante prueba","titulo":"Título prueba"}`)
		default:
			t.Errorf("unexpected mock request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	for _, key := range []string{"ParametrosCrudURL", "PazYSalvosMidURL", "OikosAPIURL", "AcademicaCoreServiceURL"} {
		old, _ := beego.AppConfig.String(key)
		beego.AppConfig.Set(key, upstream.URL)
		k := key
		defer beego.AppConfig.Set(k, old)
	}
	app := beego.NewHttpSever()
	app.Router("/v1/diplomas/estudiantes-aprobados", &EstudianteGradoController{}, "get:AprobadosPorFacultad")
	req := httptest.NewRequest(http.MethodGet, "/v1/diplomas/estudiantes-aprobados?periodo_id=77&facultad_id=14&excluir_registrados=false", nil)
	req.Header.Set("Authorization", token)
	res := httptest.NewRecorder()
	app.Handlers.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("unexpected HTTP %d", res.Code)
	}
	var result services.EstudiantesAprobadosGradoResult
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Errores != 0 || academicCalls != 1 {
		t.Fatalf("auth/enrichment failed: %+v", result)
	}
	e := result.Facultades[0].Estudiantes[0]
	if e.IDProyectoOikos != 72 || e.ProgramaAcademicoMIDID != 84 || e.IDFacultadOikos != 14 || e.PeriodoID != 77 {
		t.Fatalf("bad mapping: %+v", e)
	}
}
