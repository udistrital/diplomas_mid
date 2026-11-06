package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

func mockAprobados(t *testing.T, total int, mutate func(*http.Request, map[string]interface{})) (*httptest.Server, *int) {
	t.Helper()
	oikosCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/parametro":
			codigo := strings.TrimPrefix(r.URL.Query().Get("query"), "CodigoAbreviacion:")
			json.NewEncoder(w).Encode(map[string]interface{}{"Data": []interface{}{map[string]interface{}{"Id": 38, "CodigoAbreviacion": codigo, "Activo": true}}})
		case "/v1/periodo":
			fmt.Fprint(w, `{"Success":true,"Status":"200","Data":[{"Id":75,"Nombre":"duplicado","Year":2026,"Ciclo":"1","CodigoAbreviacion":"PA","Activo":false},{"Id":77,"Nombre":"duplicado","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true},{"Id":79,"Nombre":"2027-1","Year":2027,"Ciclo":"1","CodigoAbreviacion":"PA","Activo":true},{"Id":80,"Year":2028,"Ciclo":"1","CodigoAbreviacion":"OTRO"}]}`)
		case "/v1/solicitud-grado/paz-salvos/aprobados":
			if r.URL.Query().Get("limit") != "20" || r.URL.Query().Get("periodo_id") == "" {
				t.Error("missing pagination/period")
			}
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			periodo, _ := strconv.Atoi(r.URL.Query().Get("periodo_id"))
			items := []interface{}{}
			for i := offset; i < offset+20 && i < total; i++ {
				items = append(items, map[string]interface{}{"SolicitudId": i + 1, "TerceroId": 86511 + i, "CodigoEstudiante": strconv.Itoa(2025000 + i), "PeriodoId": periodo, "ProgramaAcademicoId": 26, "DependenciaOikosId": 275})
			}
			response := map[string]interface{}{"Success": true, "Status": 200, "Data": map[string]interface{}{"Solicitudes": items, "Total": total}}
			if mutate != nil {
				mutate(r, response)
			}
			json.NewEncoder(w).Encode(response)
		case "/v1/estudiantes/datos-diploma/2025000":
			fmt.Fprint(w, `{"nombre":"Estudiante prueba","numero_identificacion":"123","titulo":"Ingeniería","tipo_documento":"CC","municipio_expedicion":"Bogotá"}`)
		case "/v1/dependencia/275":
			fmt.Fprint(w, `{"Id":275,"Nombre":"Programa"}`)
		case "/v2/dependencia_padre/":
			oikosCalls++
			fmt.Fprint(w, `[{"Id":208,"Activo":true,"HijaId":{"Id":275,"Activo":true},"PadreId":{"Id":14,"Nombre":"FACULTAD DE INGENIERIA","Activo":true}}]`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	for _, key := range []string{"ParametrosCrudURL", "PazYSalvosMidURL", "OikosAPIURL", "AcademicaCoreServiceURL"} {
		old, _ := beego.AppConfig.String(key)
		beego.AppConfig.Set(key, server.URL)
		k := key
		t.Cleanup(func() { beego.AppConfig.Set(k, old) })
	}
	t.Cleanup(server.Close)
	return server, &oikosCalls
}

func TestPeriodosYMappingMID(t *testing.T) {
	_, calls := mockAprobados(t, 23, nil)
	periods, err := ListarPeriodosAcademicos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 3 || periods[0].ID != 79 || periods[1].ID != 77 || periods[2].Activo {
		t.Fatalf("wrong periods: %+v", periods)
	}
	estudiantes, err := consultarAprobadosMID(context.Background(), 77)
	if err != nil {
		t.Fatal(err)
	}
	if len(estudiantes) != 23 || *calls != 1 {
		t.Fatalf("pagination/dedup: students=%d oikos=%d", len(estudiantes), *calls)
	}
	e := estudiantes[0]
	doc := e.DocumentoDigitalRequest(1, 2)
	if e.ProgramaAcademicoMIDID != 26 || e.PerInsGrado != 3 || doc.PeriodoID != 77 || doc.Vigencia != 2026 || doc.ProgramaAcademicoID != 275 || doc.FacultadID != 14 {
		t.Fatalf("mapping: %+v %+v", e, doc)
	}
	if _, err := obtenerPeriodoAcademico(context.Background(), 999); err == nil {
		t.Fatal("missing period accepted")
	}
}

func TestMIDRechazaPaginasInvalidas(t *testing.T) {
	cases := []string{"failure", "empty", "duplicate", "total", "period", "code", "status", "data", "missing_total"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			mockAprobados(t, 23, func(r *http.Request, res map[string]interface{}) {
				d := res["Data"].(map[string]interface{})
				page := d["Solicitudes"].([]interface{})
				switch name {
				case "failure":
					res["Success"] = false
				case "status":
					res["Status"] = 500
				case "missing_total":
					delete(d, "Total")
				case "data":
					res["Data"] = nil
				case "empty":
					d["Solicitudes"] = []interface{}{}
				case "duplicate":
					page[1] = page[0]
				case "total":
					if r.URL.Query().Get("offset") != "0" {
						d["Total"] = 24
					}
				case "period":
					page[0].(map[string]interface{})["PeriodoId"] = 79
				case "code":
					page[0].(map[string]interface{})["CodigoEstudiante"] = "001234"
				}
			})
			if result, err := consultarAprobadosMID(context.Background(), 77); err == nil || result != nil {
				t.Fatalf("invalid page accepted: %+v %v", result, err)
			}
		})
	}
}

func TestMIDVacioCancelacionYPeriodoObligatorio(t *testing.T) {
	mockAprobados(t, 0, nil)
	result, err := consultarAprobadosMID(context.Background(), 75)
	if err != nil || len(result) != 0 {
		t.Fatalf("empty historic: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := consultarAprobadosMID(ctx, 77); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err := consultarAprobadosMID(context.Background(), 0); err == nil {
		t.Fatal("period not required")
	}
}

func TestCacheAprobadosPorPeriodo(t *testing.T) {
	mockAprobados(t, 1, nil)
	programasAprobadosCache.Lock()
	programasAprobadosCache.items = map[[2]int64]programasAprobadosCacheEntry{}
	programasAprobadosCache.Unlock()
	// Seed dependency names to avoid unrelated name endpoint.
	for _, periodo := range []int64{77, 75} {
		if _, _, err := programasAprobadosResumen(context.Background(), 14, periodo); err != nil {
			t.Fatal(err)
		}
	}
	programasAprobadosCache.RLock()
	defer programasAprobadosCache.RUnlock()
	if len(programasAprobadosCache.items) != 2 {
		t.Fatal("cache crossed periods")
	}
}

func TestOikosRechazaRelacionesInvalidas(t *testing.T) {
	for _, body := range []string{
		`[]`,
		`[{"Activo":false,"HijaId":{"Id":275,"Activo":true},"PadreId":{"Id":14,"Nombre":"FACULTAD A","Activo":true}}]`,
		`[{"Activo":true,"HijaId":{"Id":26,"Activo":true},"PadreId":{"Id":14,"Nombre":"FACULTAD A","Activo":true}}]`,
		`[{"Activo":true,"HijaId":{"Id":275,"Activo":true},"PadreId":{"Id":14,"Nombre":"FACULTAD A","Activo":true}},{"Activo":true,"HijaId":{"Id":275,"Activo":true},"PadreId":{"Id":15,"Nombre":"FACULTAD B","Activo":true}}]`,
		`[{"Activo":true,"HijaId":{"Id":275,"Activo":true},"PadreId":{"Id":275,"Nombre":"Programa","Activo":true}}]`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			old, _ := beego.AppConfig.String("OikosAPIURL")
			beego.AppConfig.Set("OikosAPIURL", server.URL)
			defer beego.AppConfig.Set("OikosAPIURL", old)
			if id, err := resolverFacultadOikos(context.Background(), 275, map[int64]bool{}); err == nil || id != 0 {
				t.Fatalf("invalid faculty: %d %v", id, err)
			}
		})
	}
}

func TestMIDHTTPError(t *testing.T) {
	mockAprobados(t, 1, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	beego.AppConfig.Set("PazYSalvosMidURL", server.URL)
	if result, err := consultarAprobadosMID(context.Background(), 77); err == nil || result != nil {
		t.Fatal("HTTP error accepted")
	}
}

func TestFlujoListadoDryRunYDocumentosMID(t *testing.T) {
	mockAprobados(t, 1, nil)
	posts := 0
	registrado := false
	crud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/documento_digital/" {
			t.Errorf("unexpected CRUD %s", r.URL)
			http.NotFound(w, r)
			return
		}
		if r.Method == "GET" {
			q := r.URL.Query().Get("query")
			if !strings.Contains(q, "periodo_id:77") || !strings.Contains(q, "programa_academico_id:275") {
				t.Errorf("wrong persisted semantics: %s", q)
			}
			if registrado {
				fmt.Fprint(w, `[{"id":10}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
			return
		}
		posts++
		var payload crearDocumentoDigitalRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.PeriodoID != 77 || payload.Vigencia != 2026 || payload.FacultadID != 14 || payload.ProgramaAcademicoID != 275 {
			t.Errorf("bad persisted payload: %+v", payload)
		}
		registrado = true
		fmt.Fprint(w, `{"id":10}`)
	}))
	defer crud.Close()
	old, _ := beego.AppConfig.String("DiplomasCrudURL")
	beego.AppConfig.Set("DiplomasCrudURL", crud.URL)
	defer beego.AppConfig.Set("DiplomasCrudURL", old)
	service := EstudianteGradoService{}
	listado, err := service.ListarAprobadosPorFacultad(context.Background(), &ListarAprobadosPorFacultadInput{PeriodoID: 77, FacultadID: 14, ProgramaAcademicoID: 275, ExcluirRegistrados: true})
	if err != nil || listado.Total != 1 || listado.Errores != 0 {
		t.Fatalf("list: %+v %v", listado, err)
	}
	input := &CrearDocumentosAprobadosInput{PeriodoID: 77, DryRun: true}
	dry, err := service.CrearDocumentosAprobados(context.Background(), input)
	if err != nil || !dry.DryRun || dry.Total != 1 || posts != 0 {
		t.Fatalf("dryrun: %+v %v posts %d", dry, err, posts)
	}
	input.DryRun = false
	created, err := service.CrearDocumentosAprobados(context.Background(), input)
	if err != nil || created.Creados != 1 || posts != 1 {
		t.Fatalf("creation: %+v %v posts %d", created, err, posts)
	}
	listado, err = service.ListarAprobadosPorFacultad(context.Background(), &ListarAprobadosPorFacultadInput{PeriodoID: 77, ExcluirRegistrados: true})
	if err != nil || listado.Total != 0 {
		t.Fatalf("exclusion: %+v %v", listado, err)
	}
}
