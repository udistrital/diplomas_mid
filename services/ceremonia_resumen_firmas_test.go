package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestResumenFirmasPorEtapaSinEnriquecimiento(t *testing.T) {
	for _, etapa := range []struct{ orden, total, firmados, pendientes int }{
		{1, 5, 4, 1}, {2, 4, 3, 1}, {3, 3, 2, 1}, {4, 2, 1, 1},
	} {
		t.Run(strconv.Itoa(etapa.orden), func(t *testing.T) {
			solicitudesFirmas := map[int64]int{}
			roles := []int64{123, 124, 125, 126}
			// Usar los IDs de rol reales, sin acoplar las pruebas a su numeración.
			for id, rol := range rolesFirmaPermitidos {
				if rol.Orden >= 1 && rol.Orden <= 4 {
					roles[rol.Orden-1] = int64(id)
				}
			}
			ceremonia := &ceremoniaGradoCRUDResponse{ID: 10, PeriodoID: 77, CantidadEstudiantesProgramada: 10, Activo: true}
			relaciones := []ceremoniaFacultadProgramaCRUDResponse{
				{ID: 1, CeremoniaGrado: ceremonia, FacultadID: 14, Activo: true},
				{ID: 2, CeremoniaGrado: ceremonia, FacultadID: 14, Activo: true},
				{ID: 3, CeremoniaGrado: &ceremoniaGradoCRUDResponse{ID: 20, PeriodoID: 77, Activo: true}, FacultadID: 14, Activo: true},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				query := r.URL.Query().Get("query")
				switch r.URL.Path {
				case "/v1/ceremonia_estudiante/":
					var items []ceremoniaEstudianteCRUDResponse
					switch query {
					case "ceremonia_facultad_programa_id:1,activo:true":
						for id := int64(1); id <= 5; id++ {
							items = append(items, ceremoniaEstudianteCRUDResponse{ID: id, Activo: true, DocumentoDigital: &documentoDigitalResponse{ID: id}})
						}
						// Duplicada e inactiva: no inflan el resumen ni disparan otra consulta.
						items = append(items, items[0], ceremoniaEstudianteCRUDResponse{ID: 99, Activo: false, DocumentoDigital: &documentoDigitalResponse{ID: 99}})
					case "ceremonia_facultad_programa_id:2,activo:true":
						items = []ceremoniaEstudianteCRUDResponse{{ID: 6, Activo: true, DocumentoDigital: &documentoDigitalResponse{ID: 1}}}
					case "ceremonia_facultad_programa_id:3,activo:true":
						items = []ceremoniaEstudianteCRUDResponse{{ID: 7, Activo: true, DocumentoDigital: &documentoDigitalResponse{ID: 6}}}
					default:
						t.Errorf("consulta de asignaciones inesperada: %s", query)
					}
					_ = json.NewEncoder(w).Encode(items)
				case "/v1/firma_documento/":
					partes := strings.Split(strings.TrimPrefix(query, "documento_digital_id:"), ",")
					id, _ := strconv.ParseInt(partes[0], 10, 64)
					solicitudesFirmas[id]++
					maxOrden := int(id) - 1
					if maxOrden > 4 {
						maxOrden = 4
					}
					firmas := []firmaDocumentoResponse{}
					for orden := 1; orden <= maxOrden; orden++ {
						firmas = append(firmas, firmaDocumentoResponse{RolFirmanteID: roles[orden-1], Activo: true})
					}
					firmas = append(firmas, firmaDocumentoResponse{RolFirmanteID: roles[3], Activo: false})
					_ = json.NewEncoder(w).Encode(firmas)
				default:
					t.Errorf("el resumen no debe consultar detalle enriquecido: %s", r.URL.Path)
					http.Error(w, "inesperado", 500)
				}
			}))
			defer server.Close()
			t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
			result, err := construirTableroResumenFirmas(t.Context(), relaciones, 14, etapa.orden)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Ceremonias) != 2 {
				t.Fatalf("ceremonias: %+v", result.Ceremonias)
			}
			resumen := result.Ceremonias[0].ResumenFirmas
			if resumen == nil || resumen.Orden != etapa.orden || resumen.Total != etapa.total || resumen.Firmados != etapa.firmados || resumen.Pendientes != etapa.pendientes {
				t.Fatalf("resumen incorrecto: %+v", resumen)
			}
			if result.Ceremonias[0].CuposAsignados != 5 || result.Ceremonias[0].CuposDisponibles != 15 {
				t.Fatalf("se confundieron cupos con pendientes: %+v", result.Ceremonias[0])
			}
			completada := result.Ceremonias[1].ResumenFirmas
			if completada.Total != 1 || completada.Firmados != 1 || completada.Pendientes != 0 {
				t.Fatalf("se perdió ceremonia completada: %+v", completada)
			}
			for id := int64(1); id <= 6; id++ {
				if solicitudesFirmas[id] != 1 {
					t.Errorf("documento %d consultado %d veces", id, solicitudesFirmas[id])
				}
			}
			if solicitudesFirmas[99] != 0 {
				t.Error("se consultó asignación inactiva")
			}
		})
	}
}

func TestResumenFirmasErrorNoSePresentaComoCompletada(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/ceremonia_estudiante/" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"id":1,"activo":true,"documento_digital":{"id":1}}]`))
			return
		}
		http.Error(w, "servicio no disponible", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	result, err := construirTableroResumenFirmas(t.Context(), []ceremoniaFacultadProgramaCRUDResponse{{ID: 1, Activo: true, CeremoniaGrado: &ceremoniaGradoCRUDResponse{ID: 10, Activo: true}}}, 14, 1)
	if result != nil || !errors.Is(err, ErrExternalService) {
		t.Fatalf("no debe devolver conteos parciales: %+v, %v", result, err)
	}
}

func TestResumenFirmasFiltraFacultadPeriodoYPrevias(t *testing.T) {
	consultasAsignaciones := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ceremonia_facultad_programa/":
			if !strings.Contains(r.URL.Query().Get("query"), "facultad_id:14") {
				t.Error("consulta fuera de facultad")
			}
			w.Write([]byte(`[{"id":1,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":10,"periodo_id":77,"activo":true}},{"id":2,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":20,"periodo_id":79,"activo":true}},{"id":3,"activo":true,"facultad_id":15,"ceremonia_grado":{"id":30,"periodo_id":77,"activo":true}},{"id":4,"activo":false,"facultad_id":14,"ceremonia_grado":{"id":40,"periodo_id":77,"activo":true}}]`))
		case "/v1/ceremonia_estudiante/":
			consultasAsignaciones++
			if r.URL.Query().Get("query") != "ceremonia_facultad_programa_id:1,activo:true" {
				t.Error("se consultó otro período o facultad")
			}
			w.Write([]byte(`[{"id":1,"activo":true,"documento_digital":{"id":1}}]`))
		case "/v1/firma_documento/":
			w.Write([]byte(`[]`))
		default:
			t.Errorf("request inesperada: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	result, err := (CeremoniaService{}).tableroFirmasFacultad(t.Context(), 14, ordenDecano, 77)
	if err != nil {
		t.Fatal(err)
	}
	if consultasAsignaciones != 1 {
		t.Fatalf("consultas: %d", consultasAsignaciones)
	}
	if len(result.Ceremonias) != 0 {
		t.Fatal("sin firma de secretario no debe contarse completada para decano")
	}
}

func TestListarSecretarioResumenNoConsultaAprobados(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/facultad_secretaria/1023026828":
			w.Write([]byte(secretariaFacultadJSON()))
		case "/facultad_oikos_gedep/33":
			w.Write([]byte(homologacionGedepJSON()))
		case "/v1/ceremonia_facultad_programa/":
			w.Write([]byte(`[{"id":1,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":10,"periodo_id":77,"activo":true}}]`))
		case "/v1/ceremonia_estudiante/":
			w.Write([]byte(`[{"id":1,"activo":true,"documento_digital":{"id":1}}]`))
		case "/v1/firma_documento/":
			w.Write([]byte(`[]`))
		default:
			t.Errorf("resumen no debe consultar aprobados ni enriquecer estudiantes: %s", r.URL.Path)
			http.Error(w, "inesperado", 500)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	t.Setenv("ACADEMICA_PRUEBAS_API_URL", server.URL)
	t.Setenv("HOMOLOGACION_DEPENDENCIAS_API_URL", server.URL)
	result, err := (CeremoniaService{}).Listar(t.Context(), &ListarCeremoniasInput{PeriodoID: 77, DocumentoIdentidadSecretario: 1023026828, IncluirResumenFirmas: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Ceremonias) != 1 || result.Ceremonias[0].ResumenFirmas == nil || result.Ceremonias[0].ResumenFirmas.Orden != 1 || result.Ceremonias[0].ResumenFirmas.Pendientes != 1 {
		t.Fatalf("resumen no incluido: %+v", result)
	}
}

func TestTablerosGlobalesIncluyenResumenExplicito(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ceremonia_facultad_programa/":
			w.Write([]byte(`[{"id":1,"activo":true,"facultad_id":14,"ceremonia_grado":{"id":10,"periodo_id":77,"activo":true}}]`))
		case "/v1/ceremonia_estudiante/":
			w.Write([]byte(`[{"id":1,"activo":true,"documento_digital":{"id":1,"nombre_estudiante":"Estudiante","numero_documento_estudiante":"123"}}]`))
		case "/v1/firma_documento/":
			w.Write([]byte(`[{"rol_firmante_id":158,"activo":true},{"rol_firmante_id":270,"activo":true},{"rol_firmante_id":106,"activo":true},{"rol_firmante_id":100,"activo":true}]`))
		default:
			t.Errorf("consulta inesperada: %s", r.URL.Path)
			http.Error(w, "inesperado", 500)
		}
	}))
	defer server.Close()
	t.Setenv("DIPLOMAS_CRUD_URL", server.URL)
	for _, orden := range []int{ordenSecretariaGral, ordenRector} {
		result, err := (CeremoniaService{}).tableroFirmasGlobal(t.Context(), orden)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Ceremonias) != 1 {
			t.Fatalf("ceremonia completada ausente: %+v", result)
		}
		resumen := result.Ceremonias[0].ResumenFirmas
		if resumen == nil || resumen.Orden != orden || resumen.Total != 1 || resumen.Firmados != 1 || resumen.Pendientes != 0 {
			t.Fatalf("resumen global incorrecto: %+v", resumen)
		}
	}
}
