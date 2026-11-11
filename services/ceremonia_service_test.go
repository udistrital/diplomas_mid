package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ceremoniaInputValido() *CrearCeremoniaInput {
	return &CrearCeremoniaInput{
		PeriodoID:                     77,
		FechaCeremonia:                "2026-11-20",
		Lugar:                         "Auditorio Mayor",
		CantidadEstudiantesProgramada: 65,
		Observacion:                   "Ceremonia ordinaria",
		DocumentoIdentidadSecretario:  1023026828,
	}
}

func secretariaFacultadJSON() string {
	return `{"facultades":{"secretaria":[{"EMP_NRO_IDEN":"1023026828","SEC_DEP_COD":"33","DEP_NOMBRE":"FACULTAD DE INGENIERIA","SEC_NOMBRE":"ORLANDO","SEC_APELLIDO":"RIOS LEON"}]}}`
}

func homologacionGedepJSON() string {
	return `{"homologacion":{"id_oikos":"14","id_gedep":"33"}}`
}

func TestCrearCeremoniaExitosa(t *testing.T) {
	var ceremoniaPosts, relacionPosts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/periodo":
			w.Write([]byte(`{"Success":true,"Status":"200","Data":[{"Id":77,"Nombre":"2026-3","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_secretaria/1023026828":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(secretariaFacultadJSON()))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_oikos_gedep/33":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(homologacionGedepJSON()))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ceremonia_grado/":
			ceremoniaPosts++
			var payload ceremoniaGradoCRUDRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload.PeriodoID != 77 {
				t.Error("periodo_id no persistido en creación")
			}

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":99,"periodo_id":77,"lugar":"Auditorio Mayor","cantidad_estudiantes_programada":65,"activo":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ceremonia_facultad_programa/":
			relacionPosts++
			var payload ceremoniaFacultadProgramaCRUDRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("invalid relation payload: %v", err)
			}
			if payload.FacultadID != 14 || payload.ProgramaAcademicoID != 0 || payload.CantidadEstudiantesProgramada == nil || *payload.CantidadEstudiantesProgramada != 65 {
				t.Fatalf("unexpected relation payload: %+v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": int64(100), "facultad_id": int64(14), "programa_academico_id": int64(0), "activo": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("PARAMETROS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)

	result, err := CeremoniaService{}.Crear(t.Context(), ceremoniaInputValido())
	if err != nil {
		t.Fatalf("Crear returned error: %v", err)
	}
	if result.Ceremonia.ID != 99 || result.Facultad.FacultadID != 14 || result.SecretarioAcademico.Rol != "secretario_academico" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if ceremoniaPosts != 1 || relacionPosts != 1 {
		t.Fatalf("unexpected posts ceremonia=%d relaciones=%d", ceremoniaPosts, relacionPosts)
	}
}

func TestCrearCeremoniaPayloadInvalido(t *testing.T) {
	_, err := CeremoniaService{}.Crear(t.Context(), nil)
	if err == nil || !strings.Contains(err.Error(), "request body") {
		t.Fatalf("expected invalid payload error, got %v", err)
	}
}

func TestCrearCeremoniaSinDocumentoSecretario(t *testing.T) {
	input := ceremoniaInputValido()
	input.DocumentoIdentidadSecretario = 0
	_, err := CeremoniaService{}.Crear(t.Context(), input)
	if err == nil || !strings.Contains(err.Error(), "documento_identidad_secretario") {
		t.Fatalf("expected documento error, got %v", err)
	}
}

func TestCrearCeremoniaRechazaNoSecretario(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/facultad_secretaria/1023026828" {
			_, _ = w.Write([]byte(`{"facultades":{"secretaria":[]}}`))
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)

	_, err := CeremoniaService{}.Crear(t.Context(), ceremoniaInputValido())
	if err == nil || !strings.Contains(err.Error(), "secretaria de facultad") {
		t.Fatalf("expected secretary error, got %v", err)
	}
}

func TestCrearCeremoniaFalloCrearCeremoniaEnCRUD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/periodo":
			w.Write([]byte(`{"Success":true,"Status":"200","Data":[{"Id":77,"Nombre":"2026-3","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_secretaria/1023026828":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(secretariaFacultadJSON()))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_oikos_gedep/33":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(homologacionGedepJSON()))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ceremonia_grado/":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("PARAMETROS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)

	_, err := CeremoniaService{}.Crear(t.Context(), ceremoniaInputValido())
	if err == nil || !strings.Contains(err.Error(), "ceremonia_grado") {
		t.Fatalf("expected CRUD error, got %v", err)
	}
}

func TestCrearCeremoniaFalloRelacionEjecutaRollback(t *testing.T) {
	var deleteCeremonia bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/periodo":
			w.Write([]byte(`{"Success":true,"Status":"200","Data":[{"Id":77,"Nombre":"2026-3","Year":2026,"Ciclo":"3","CodigoAbreviacion":"PA","Activo":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_secretaria/1023026828":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(secretariaFacultadJSON()))
		case r.Method == http.MethodGet && r.URL.Path == "/facultad_oikos_gedep/33":
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected Accept application/json")
			}
			_, _ = w.Write([]byte(homologacionGedepJSON()))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ceremonia_grado/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":99,"periodo_id":77,"activo":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/ceremonia_facultad_programa/":
			w.WriteHeader(http.StatusInternalServerError)
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/ceremonia_grado/99":
			deleteCeremonia = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("PARAMETROS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)

	_, err := CeremoniaService{}.Crear(t.Context(), ceremoniaInputValido())
	if err == nil || !strings.Contains(err.Error(), "ceremonia_facultad_programa") {
		t.Fatalf("expected relation error, got %v", err)
	}
	if !deleteCeremonia {
		t.Fatal("expected rollback delete ceremonia")
	}
}

func TestListarCeremoniasPersistidasSinPeriodo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/facultad_secretaria/1023026828":
			w.Write([]byte(secretariaFacultadJSON()))
		case "/facultad_oikos_gedep/33":
			w.Write([]byte(homologacionGedepJSON()))
		case "/v1/ceremonia_facultad_programa/":
			w.Write([]byte(`[{"id":1,"facultad_id":14,"activo":true,"cantidad_estudiantes_programada":5,"ceremonia_grado":{"id":1,"activo":true,"lugar":"Prueba","cantidad_estudiantes_programada":5,"fecha_ceremonia":"2026-10-27T00:00:00Z"}}]`))
		case "/v1/ceremonia_estudiante/":
			w.Write([]byte(`[]`))
		default:
			t.Errorf("No debe consultar aprobados ni períodos: %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("PARAMETROS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)
	result, err := (CeremoniaService{}).Listar(t.Context(), &ListarCeremoniasInput{DocumentoIdentidadSecretario: 1023026828})
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCeremonias != 1 || result.Ceremonias[0].ID != 1 || result.TotalCupos != 5 {
		t.Fatalf("No devolvió ceremonia persistida: %+v", result)
	}
	if len(result.AprobadosProgramas) != 0 {
		t.Fatal("Consultó aprobados sin período")
	}
}

func TestAsignacionRechazaPeriodoAjenoAntesDeConsultarAprobados(t *testing.T) {
	for _, tc := range []struct {
		nombre                string
		solicitado, ceremonia int64
	}{
		{"otro período", 79, 77}, {"sin período solicitado", 0, 77}, {"ceremonia histórica sin configurar", 77, 0},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/facultad_secretaria/1023026828":
					w.Write([]byte(secretariaFacultadJSON()))
				case "/facultad_oikos_gedep/33":
					w.Write([]byte(homologacionGedepJSON()))
				case "/v1/ceremonia_facultad_programa/":
					json.NewEncoder(w).Encode([]any{map[string]any{"id": 1, "activo": true, "facultad_id": 14, "ceremonia_grado": map[string]any{"id": 99, "periodo_id": tc.ceremonia, "activo": true}}})
				default:
					t.Errorf("No debe consultar ni escribir con período incompatible: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 500)
				}
			}))
			defer server.Close()
			t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
			t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
			t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)
			_, err := (CeremoniaService{}).AsignarEstudiantes(t.Context(), 99, &AsignarEstudiantesCeremoniaInput{PeriodoID: tc.solicitado, DocumentoIdentidadSecretario: 1023026828, Programas: []AsignarProgramaCeremoniaItem{{ProgramaAcademicoID: 72}}})
			if err == nil {
				t.Fatal("aceptó asignación cruzada")
			}
		})
	}
}

func TestPendientesNoCruzaProgramaEntrePeriodos(t *testing.T) {
	aprobados := []ProgramaAprobadoResumen{{PeriodoID: 77, ProgramaAcademicoID: 72, Cantidad: 1}, {PeriodoID: 79, ProgramaAcademicoID: 72, Cantidad: 2}}
	asignados := []ProgramaAprobadoResumen{{PeriodoID: 79, ProgramaAcademicoID: 72, Cantidad: 2}}
	pendientes := programasPendientes(aprobados, asignados)
	if len(pendientes) != 1 || pendientes[0].PeriodoID != 77 {
		t.Fatalf("períodos mezclados: %+v", pendientes)
	}
}

func TestCeremoniaRequierePeriodoExplicito(t *testing.T) {
	input := ceremoniaInputValido()
	input.PeriodoID = 0
	if err := validarCeremonia(input); err == nil {
		t.Fatal("aceptó ceremonia sin período")
	}
}

func TestAsignarSoloEstudiantesDelPeriodoDeCeremonia(t *testing.T) {
	mockAprobados(t, 1, nil)
	writes := 0
	registroReservado := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/facultad_secretaria/1023026828":
			w.Write([]byte(secretariaFacultadJSON()))
		case "/facultad_oikos_gedep/33":
			w.Write([]byte(homologacionGedepJSON()))
		case "/v1/ceremonia_facultad_programa/":
			w.Write([]byte(`[{"id":1,"activo":true,"facultad_id":14,"cantidad_estudiantes_programada":5,"ceremonia_grado":{"id":99,"periodo_id":77,"fecha_ceremonia":"2026-11-20T00:00:00Z","activo":true}}]`))
		case "/v1/ceremonia_estudiante/":
			if r.Method == http.MethodGet {
				w.Write([]byte(`[]`))
				return
			}
			var body ceremoniaEstudianteCRUDRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.CeremoniaFacultadPrograma.ID != 1 || body.DocumentoDigital.ID != 500 {
				t.Error("wrong assignment identity")
			}
			if !registroReservado {
				t.Error("asignación creada antes de reservar registro")
			}
			writes++
			w.Write([]byte(`{"id":1}`))
		case "/v1/documento_digital/":
			if r.Method == http.MethodGet {
				w.Write([]byte(`[]`))
				return
			}
			var body crearDocumentoDigitalRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.PeriodoID != 77 || body.ProgramaAcademicoID != 275 || body.FacultadID != 14 {
				t.Errorf("mixed period/program: %+v", body)
			}
			w.Write([]byte(`{"id":500,"periodo_id":77,"facultad_id":14,"nombre_estudiante":"Estudiante Prueba","tipo_documento_estudiante":"CC","numero_documento_estudiante":"123","titulo_otorgado":"Ingeniero"}`))
		case "/v1/diploma_digital/":
			w.Write([]byte(`[]`))
		case "/v1/parametrizacion_registro_facultad/":
			w.Write([]byte(`[{"id":1,"facultad_id":14,"vigencia":2026,"activo":true}]`))
		case "/v1/documento_digital/500/diploma":
			var payload crearDiplomaDigitalRequest
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.FechaGrado.Format("2006-01-02") != "2026-11-20" {
				t.Error("no utilizó fecha de ceremonia")
			}
			w.Write([]byte(`{"id":600}`))
		case "/v1/diploma_digital/600/registro":
			registroReservado = true
			w.Write([]byte(`{"id":600,"registro_grado":{"id":1,"numero_registro":1}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)
	result, err := (CeremoniaService{}).AsignarEstudiantes(t.Context(), 99, &AsignarEstudiantesCeremoniaInput{PeriodoID: 77, DocumentoIdentidadSecretario: 1023026828, Programas: []AsignarProgramaCeremoniaItem{{ProgramaAcademicoID: 275}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Asignados != 1 || result.Errores != 0 || writes != 1 {
		t.Fatalf("unexpected result: %+v writes %d", result, writes)
	}
}

func TestTableroFiltraCeremoniasPorPeriodoConfigurado(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ceremonia_facultad_programa/":
			w.Write([]byte(`[{"id":1,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":1,"periodo_id":77,"activo":true,"cantidad_estudiantes_programada":5}},{"id":2,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":2,"periodo_id":79,"activo":true,"cantidad_estudiantes_programada":10}}]`))
		case "/v1/ceremonia_estudiante/":
			if !strings.Contains(r.URL.Query().Get("query"), "ceremonia_facultad_programa_id:1") {
				t.Error("consultó asignaciones de otro período")
			}
			w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	result, err := (CeremoniaService{}).listarPorFacultad(t.Context(), 14, false, 77)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCeremonias != 1 || result.Ceremonias[0].PeriodoID != 77 || result.TotalCupos != 5 {
		t.Fatalf("mezcló ceremonias de otros períodos: %+v", result)
	}
}

// Reasignar después de retirar debe reutilizar la pareja única, sin insertar duplicados.
func TestCrearAsignacionReutilizaRegistroRetirado(t *testing.T) {
	for _, tc := range []struct {
		nombre     string
		existentes string
		metodo     string
		status     int
		creado     bool
		falla      bool
	}{
		{"nueva", "[]", http.MethodPost, 201, true, false},
		{"activa", `[{"id":7,"activo":true}]`, "", 200, false, false},
		{"retirada", `[{"id":7,"activo":false,"fecha_creacion":"2026-10-28T15:13:28Z"}]`, http.MethodPut, 200, true, false},
		{"reactivacion_rechazada", `[{"id":7,"activo":false}]`, http.MethodPut, 409, false, true},
	} {
		t.Run(tc.nombre, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					if strings.Contains(r.URL.Query().Get("query"), "activo:") {
						t.Error("la búsqueda debe incluir asignaciones retiradas")
					}
					w.Write([]byte(tc.existentes))
					return
				}
				writes++
				if r.Method != tc.metodo {
					t.Errorf("método %s, esperado %s", r.Method, tc.metodo)
				}
				if r.Method == http.MethodPut && r.URL.Path != "/v1/ceremonia_estudiante/7" {
					t.Errorf("no reutilizó el registro: %s", r.URL.Path)
				}
				var payload ceremoniaEstudianteCRUDRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if !payload.Activo || payload.DocumentoDigital.ID != 2 || payload.CeremoniaFacultadPrograma.ID != 3 {
					t.Errorf("payload inválido: %+v", payload)
				}
				if tc.nombre == "retirada" && payload.FechaCreacion != "2026-10-28T15:13:28Z" {
					t.Error("se perdió fecha de creación original")
				}
				w.WriteHeader(tc.status)
				w.Write([]byte(`{"id":7,"activo":true}`))
			}))
			defer server.Close()
			t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
			creado, err := crearAsignacionCeremoniaEstudiante(t.Context(), 3, 2)
			if creado != tc.creado || (err != nil) != tc.falla {
				t.Fatalf("creado=%v error=%v", creado, err)
			}
			if (tc.metodo == "" && writes != 0) || (tc.metodo != "" && writes != 1) {
				t.Fatalf("escrituras inesperadas: %d", writes)
			}
		})
	}
}
