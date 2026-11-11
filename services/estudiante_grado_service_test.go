package services

import (
	"encoding/json"
	"testing"
)

func TestAgruparEstudiantesAprobadosPorFacultad(t *testing.T) {
	t.Parallel()

	result := agruparEstudiantesAprobadosPorFacultad([]EstudianteAprobadoGrado{
		{ID: 1, CodigoEstudiante: 10000000001, IDFacultadOikos: 14},
		{ID: 2, CodigoEstudiante: 10000000002, IDFacultadOikos: 14},
		{ID: 3, CodigoEstudiante: 10000000003, IDFacultadOikos: 2},
	})

	if result.Total != 3 {
		t.Fatalf("expected total 3, got %d", result.Total)
	}
	if len(result.Facultades) != 2 {
		t.Fatalf("expected 2 facultades, got %d", len(result.Facultades))
	}
	if result.Facultades[0].FacultadID != 2 || result.Facultades[0].Total != 1 {
		t.Fatalf("unexpected first group: %#v", result.Facultades[0])
	}
	if result.Facultades[1].FacultadID != 14 || result.Facultades[1].Total != 2 {
		t.Fatalf("unexpected second group: %#v", result.Facultades[1])
	}
}

func TestEstudianteAprobadoGradoDocumentoDigitalRequest(t *testing.T) {
	t.Parallel()

	estudiante := EstudianteAprobadoGrado{
		CodigoEstudiante: 10000000001,
		IDProyectoOikos:  73,
		AnioInsGrado:     2025,
		PerInsGrado:      3,
		PeriodoID:        77,
	}

	result := estudiante.DocumentoDigitalRequest(38, 1)

	if result.CodigoEstudiante != 10000000001 {
		t.Fatalf("expected codigo_estudiante, got %d", result.CodigoEstudiante)
	}
	if result.TipoDocumentoDigitalID != 38 {
		t.Fatalf("expected tipo_documento_digital_id, got %d", result.TipoDocumentoDigitalID)
	}
	if result.ProgramaAcademicoID != 73 {
		t.Fatalf("expected programa_academico_id from IdProyectoOikos, got %d", result.ProgramaAcademicoID)
	}
	if result.Vigencia != 2025 {
		t.Fatalf("expected vigencia from AnioInsGrado, got %d", result.Vigencia)
	}
	if result.PeriodoID != 77 {
		t.Fatalf("expected real periodo_id, got %d", result.PeriodoID)
	}
	if !result.Activo {
		t.Fatal("expected activo true")
	}
}

func TestFiltrarEstudiantesAprobadosPorCodigo(t *testing.T) {
	t.Parallel()

	estudiantes := []EstudianteAprobadoGrado{
		{ID: 1, CodigoEstudiante: 10000000001},
		{ID: 2, CodigoEstudiante: 10000000004},
		{ID: 3, CodigoEstudiante: 10000000003},
	}

	result := filtrarEstudiantesAprobadosPorCodigo(estudiantes, 10000000004)

	if len(result) != 1 {
		t.Fatalf("expected one estudiante, got %d", len(result))
	}
	if result[0].CodigoEstudiante != 10000000004 {
		t.Fatalf("expected codigo 10000000004, got %d", result[0].CodigoEstudiante)
	}
}

func TestParseParametroResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "wrapped data",
			raw:  `{"Data":[{"Id":7311,"CodigoAbreviacion":"DIP_NORMAL","Activo":true}]}`,
		},
		{
			name: "direct array",
			raw:  `[{"id":7313,"codigo_abreviacion":"PEND_GEN","activo":true}]`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseParametroResponse(json.RawMessage(test.raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != 1 {
				t.Fatalf("expected one parametro, got %d", len(result))
			}
			if result[0].ID == 0 || result[0].CodigoAbreviacion == "" || !result[0].Activo {
				t.Fatalf("unexpected parametro: %#v", result[0])
			}
		})
	}
}
