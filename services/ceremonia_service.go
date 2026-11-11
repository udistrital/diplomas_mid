package services

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/udistrital/utils_oas/v2/request"
)

type CeremoniaService struct{}

type programasAprobadosCacheEntry struct {
	programas []ProgramaAprobadoResumen
	total     int
	expira    time.Time
}

var programasAprobadosCache = struct {
	sync.RWMutex
	items map[[2]int64]programasAprobadosCacheEntry
}{items: map[[2]int64]programasAprobadosCacheEntry{}}

func (s CeremoniaService) Crear(ctx context.Context, input *CrearCeremoniaInput) (*CrearCeremoniaResult, error) {
	if err := validarCeremonia(input); err != nil {
		return nil, err
	}

	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}

	if _, err := obtenerPeriodoAcademico(ctx, input.PeriodoID); err != nil {
		return nil, err
	}

	fechaCeremonia, _ := time.Parse("2006-01-02", input.FechaCeremonia)
	ceremoniaPayload := ceremoniaGradoCRUDRequest{
		PeriodoID:                     input.PeriodoID,
		FechaCeremonia:                fechaCeremonia.Format(time.RFC3339),
		Lugar:                         strings.TrimSpace(input.Lugar),
		CantidadEstudiantesProgramada: input.CantidadEstudiantesProgramada,
		Observacion:                   strings.TrimSpace(input.Observacion),
		Activo:                        true,
	}

	var ceremonia ceremoniaGradoCRUDResponse
	status, err := request.PostWithContext(ctx, diplomasCrudURL()+"/ceremonia_grado/", ceremoniaPayload, &ceremonia)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_grado status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_grado returned status %d", ErrExternalService, status)
	}
	if ceremonia.ID <= 0 {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_grado response missing id", ErrExternalService)
	}

	if ceremonia.PeriodoID != input.PeriodoID {
		s.rollbackCeremonia(ctx, ceremonia.ID, nil)
		return nil, fmt.Errorf("%w: CRUD no persistió periodo_id; desplegar migración de ceremonias", ErrExternalService)
	}

	cantidad := input.CantidadEstudiantesProgramada
	relacionPayload := ceremoniaFacultadProgramaCRUDRequest{
		CeremoniaGrado:                ceremoniaRef{ID: ceremonia.ID},
		FacultadID:                    secretario.FacultadID,
		ProgramaAcademicoID:           0,
		CantidadEstudiantesProgramada: &cantidad,
		Activo:                        true,
	}
	var relacion ceremoniaFacultadProgramaCRUDResponse
	status, err = request.PostWithContext(ctx, diplomasCrudURL()+"/ceremonia_facultad_programa/", relacionPayload, &relacion)
	if err != nil || status < 200 || status >= 300 {
		s.rollbackCeremonia(ctx, ceremonia.ID, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa status %d: %v", ErrExternalService, status, err)
		}
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa returned status %d", ErrExternalService, status)
	}

	return &CrearCeremoniaResult{
		Ceremonia:           ceremonia,
		Facultad:            relacion,
		SecretarioAcademico: *secretario,
		FacultadesProgramas: []ceremoniaFacultadProgramaCRUDResponse{relacion},
	}, nil
}

func (s CeremoniaService) Listar(ctx context.Context, input *ListarCeremoniasInput) (*TableroCeremoniasResult, error) {
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario is required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	return s.listarPorFacultad(ctx, secretario.FacultadID, input.PeriodoID > 0, input.PeriodoID)
}

func (s CeremoniaService) ObtenerParametrizacionRegistro(ctx context.Context, documentoSecretario int64, vigencia int) (*ParametrizacionRegistroResponse, error) {
	if documentoSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario is required", ErrInvalidInput)
	}
	if vigencia <= 0 {
		return nil, fmt.Errorf("%w: vigencia is required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, documentoSecretario)
	if err != nil {
		return nil, err
	}
	return buscarParametrizacionRegistro(ctx, secretario.FacultadID, vigencia)
}

func (s CeremoniaService) GuardarParametrizacionRegistro(ctx context.Context, input *ParametrizacionRegistroInput) (*ParametrizacionRegistroResponse, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.DocumentoIdentidadSecretario <= 0 || input.Vigencia <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario and vigencia are required", ErrInvalidInput)
	}
	if input.LibroInicial <= 0 || input.FolioInicial <= 0 || input.RegistroInicial <= 0 ||
		input.ConsecutivoFacultadInicial <= 0 || input.FoliosPorLibro <= 0 || input.RegistrosPorFolio <= 0 {
		return nil, fmt.Errorf("%w: todos los valores de registro deben ser mayores a cero", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	payload := ParametrizacionRegistroResponse{
		FacultadID:                 secretario.FacultadID,
		Vigencia:                   input.Vigencia,
		LibroInicial:               input.LibroInicial,
		FolioInicial:               input.FolioInicial,
		RegistroInicial:            input.RegistroInicial,
		ConsecutivoFacultadInicial: input.ConsecutivoFacultadInicial,
		FoliosPorLibro:             input.FoliosPorLibro,
		RegistrosPorFolio:          input.RegistrosPorFolio,
		Activo:                     true,
	}
	actual, err := buscarParametrizacionRegistro(ctx, secretario.FacultadID, input.Vigencia)
	if err != nil {
		return nil, err
	}
	if actual != nil {
		payload.ID = actual.ID
		var result ParametrizacionRegistroResponse
		status, err := request.PutWithContext(ctx, fmt.Sprintf("%s/parametrizacion_registro_facultad/%d", diplomasCrudURL(), actual.ID), payload, &result)
		if err != nil {
			return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad put status %d: %v", ErrExternalService, status, err)
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad put returned status %d", ErrExternalService, status)
		}
		return &result, nil
	}
	var result ParametrizacionRegistroResponse
	status, err := request.PostWithContext(ctx, diplomasCrudURL()+"/parametrizacion_registro_facultad/", payload, &result)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad returned status %d", ErrExternalService, status)
	}
	return &result, nil
}

func buscarParametrizacionRegistro(ctx context.Context, facultadID int64, vigencia int) (*ParametrizacionRegistroResponse, error) {
	rawQuery := fmt.Sprintf("facultad_id:%d,vigencia:%d", facultadID, vigencia)
	endpoint := fmt.Sprintf("%s/parametrizacion_registro_facultad/?query=%s", diplomasCrudURL(), url.QueryEscape(rawQuery))
	var items []ParametrizacionRegistroResponse
	status, err := request.GetWithContext(ctx, endpoint, &items)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud parametrizacion_registro_facultad returned status %d", ErrExternalService, status)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (s CeremoniaService) ListarDecano(ctx context.Context, input *ListarCeremoniasInput) (*TableroCeremoniasResult, error) {
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_decano is required", ErrInvalidInput)
	}
	decano, err := s.decanoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	return s.listarPorFacultad(ctx, decano.FacultadID, false)
}

func (s CeremoniaService) listarPorFacultad(ctx context.Context, facultadID int64, incluirProgramas bool, periodoIDs ...int64) (*TableroCeremoniasResult, error) {
	var periodoID int64
	if len(periodoIDs) > 0 {
		periodoID = periodoIDs[0]
	}
	relaciones, err := s.listarRelacionesFacultad(ctx, facultadID)
	if err != nil {
		return nil, err
	}

	if periodoID > 0 {
		filtradas := make([]ceremoniaFacultadProgramaCRUDResponse, 0)
		for _, r := range relaciones {
			if r.CeremoniaGrado != nil && r.CeremoniaGrado.PeriodoID == periodoID {
				filtradas = append(filtradas, r)
			}
		}
		relaciones = filtradas
	}
	result := TableroCeremoniasResult{FacultadID: facultadID, Ceremonias: []CeremoniaResumen{}, AprobadosProgramas: []ProgramaAprobadoResumen{}, PendientesProgramas: []ProgramaAprobadoResumen{}, AsignadosProgramas: []ProgramaAprobadoResumen{}}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo {
			continue
		}
		cantidad := relacion.CeremoniaGrado.CantidadEstudiantesProgramada
		if relacion.CantidadEstudiantesProgramada != nil && *relacion.CantidadEstudiantesProgramada > 0 {
			cantidad = *relacion.CantidadEstudiantesProgramada
		}
		asignados, err := contarAsignadosRelacion(ctx, relacion.ID)
		if err != nil {
			return nil, err
		}
		disponibles := cantidad - asignados
		if disponibles < 0 {
			disponibles = 0
		}
		item := CeremoniaResumen{
			ID:                            relacion.CeremoniaGrado.ID,
			PeriodoID:                     relacion.CeremoniaGrado.PeriodoID,
			RelacionID:                    relacion.ID,
			FacultadID:                    relacion.FacultadID,
			FechaCeremonia:                relacion.CeremoniaGrado.FechaCeremonia,
			Lugar:                         relacion.CeremoniaGrado.Lugar,
			CantidadEstudiantesProgramada: cantidad,
			CuposAsignados:                asignados,
			CuposDisponibles:              disponibles,
			EstadoFlujo:                   "PARAMETRIZADA",
			PuedeEditar:                   true,
			PuedeEliminar:                 asignados == 0,
			Observacion:                   relacion.CeremoniaGrado.Observacion,
			Activo:                        true,
		}
		if asignados > 0 {
			item.EstadoFlujo = "ASIGNADA"
		}
		result.Ceremonias = append(result.Ceremonias, item)
		result.TotalCupos += cantidad
		result.CuposAsignados += asignados
	}
	result.TotalCeremonias = len(result.Ceremonias)
	result.CuposDisponibles = result.TotalCupos - result.CuposAsignados
	result.AsignadosProgramas, err = programasAsignadosResumen(ctx, relaciones)
	if err != nil {
		return nil, err
	}
	aplicarBloqueosCeremonias(result.Ceremonias, result.AsignadosProgramas)
	if incluirProgramas {
		result.AprobadosProgramas, result.AprobadosTotal, err = programasAprobadosResumen(ctx, facultadID, periodoID)
		if err != nil {
			return nil, err
		}
		result.PendientesProgramas = programasPendientes(result.AprobadosProgramas, result.AsignadosProgramas)
	}
	return &result, nil
}

func (s CeremoniaService) Actualizar(ctx context.Context, id int64, input *ActualizarCeremoniaInput) (*CeremoniaResumen, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil {
		return nil, fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	crearInput := &CrearCeremoniaInput{PeriodoID: input.PeriodoID, FechaCeremonia: input.FechaCeremonia, Lugar: input.Lugar, CantidadEstudiantesProgramada: input.CantidadEstudiantesProgramada, Observacion: input.Observacion, DocumentoIdentidadSecretario: input.DocumentoIdentidadSecretario}
	if err := validarCeremonia(crearInput); err != nil {
		return nil, err
	}
	if _, err := obtenerPeriodoAcademico(ctx, input.PeriodoID); err != nil {
		return nil, err
	}

	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, id, secretario.FacultadID)
	if err != nil {
		return nil, err
	}
	asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
	if err != nil {
		return nil, err
	}
	if relacion.CeremoniaGrado.PeriodoID != input.PeriodoID && len(asignaciones) > 0 {
		return nil, fmt.Errorf("%w: no se puede cambiar período con estudiantes asignados", ErrInvalidState)
	}

	flujo, err := estadoFlujoAsignaciones(ctx, asignaciones)
	if err != nil {
		return nil, err
	}
	if flujo.Bloqueado {
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, flujo.MotivoBloqueo)
	}

	fechaCeremonia, _ := time.Parse("2006-01-02", input.FechaCeremonia)
	payload := ceremoniaGradoCRUDRequest{PeriodoID: input.PeriodoID, FechaCeremonia: fechaCeremonia.Format(time.RFC3339), Lugar: strings.TrimSpace(input.Lugar), CantidadEstudiantesProgramada: input.CantidadEstudiantesProgramada, Observacion: strings.TrimSpace(input.Observacion), Activo: true}
	var ceremonia ceremoniaGradoCRUDResponse
	status, err := request.PutWithContext(ctx, fmt.Sprintf("%s/ceremonia_grado/%d", diplomasCrudURL(), id), payload, &ceremonia)
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			return nil, fmt.Errorf("%w: diplomas_crud ceremonia_grado put status %d: %v", ErrExternalService, status, err)
		}
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_grado put returned status %d", ErrExternalService, status)
	}
	if ceremonia.PeriodoID != input.PeriodoID {
		return nil, fmt.Errorf("%w: CRUD no persistió periodo_id de ceremonia", ErrExternalService)
	}

	cantidad := input.CantidadEstudiantesProgramada
	relPayload := ceremoniaFacultadProgramaCRUDRequest{CeremoniaGrado: ceremoniaRef{ID: id}, FacultadID: secretario.FacultadID, ProgramaAcademicoID: 0, CantidadEstudiantesProgramada: &cantidad, Activo: true}
	var relActualizada ceremoniaFacultadProgramaCRUDResponse
	status, err = request.PutWithContext(ctx, fmt.Sprintf("%s/ceremonia_facultad_programa/%d", diplomasCrudURL(), relacion.ID), relPayload, &relActualizada)
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa put status %d: %v", ErrExternalService, status, err)
		}
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa put returned status %d", ErrExternalService, status)
	}
	return &CeremoniaResumen{PeriodoID: ceremonia.PeriodoID, ID: ceremonia.ID, RelacionID: relacion.ID, FacultadID: secretario.FacultadID, FechaCeremonia: ceremonia.FechaCeremonia, Lugar: ceremonia.Lugar, CantidadEstudiantesProgramada: cantidad, CuposDisponibles: cantidad, Observacion: ceremonia.Observacion, Activo: true}, nil
}

func (s CeremoniaService) Eliminar(ctx context.Context, id int64, input *EliminarCeremoniaInput) error {
	if id <= 0 {
		return fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return fmt.Errorf("%w: documento_identidad_secretario is required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, id, secretario.FacultadID)
	if err != nil {
		return err
	}
	asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
	if err != nil {
		return err
	}
	if len(asignaciones) > 0 {
		return fmt.Errorf("%w: no se puede eliminar una ceremonia con estudiantes asignados", ErrInvalidState)
	}
	status, err := request.DeleteWithContext(ctx, fmt.Sprintf("%s/ceremonia_facultad_programa/%d", diplomasCrudURL(), relacion.ID), nil)
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			return fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa delete status %d: %v", ErrExternalService, status, err)
		}
		return fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa delete returned status %d", ErrExternalService, status)
	}
	status, err = request.DeleteWithContext(ctx, fmt.Sprintf("%s/ceremonia_grado/%d", diplomasCrudURL(), id), nil)
	if err != nil || status < 200 || status >= 300 {
		if err != nil {
			return fmt.Errorf("%w: diplomas_crud ceremonia_grado delete status %d: %v", ErrExternalService, status, err)
		}
		return fmt.Errorf("%w: diplomas_crud ceremonia_grado delete returned status %d", ErrExternalService, status)
	}
	return nil
}

func (s CeremoniaService) AsignarEstudiantes(ctx context.Context, ceremoniaID int64, input *AsignarEstudiantesCeremoniaInput) (*AsignarEstudiantesCeremoniaResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario is required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, ceremoniaID, secretario.FacultadID)
	if err != nil {
		return nil, err
	}
	if err := validarPeriodoAsignacion(relacion.CeremoniaGrado, input.PeriodoID); err != nil {
		return nil, err
	}

	programas := programasSeleccionados(input.Programas)
	if len(programas) == 0 {
		return nil, fmt.Errorf("%w: debe seleccionar al menos un programa", ErrInvalidInput)
	}
	capacidad := 0
	if relacion.CantidadEstudiantesProgramada != nil {
		capacidad = *relacion.CantidadEstudiantesProgramada
	}
	asignadosActuales, err := contarAsignadosRelacion(ctx, relacion.ID)
	if err != nil {
		return nil, err
	}
	disponibles := capacidad - asignadosActuales
	if disponibles <= 0 {
		return nil, fmt.Errorf("%w: la ceremonia no tiene cupos disponibles", ErrInvalidState)
	}

	tipoDocumentoDigitalID, err := resolverParametroID(ctx, 0, codigoTipoDocumentoDigitalDefault)
	if err != nil {
		return nil, err
	}
	estadoDocumentoID, err := resolverParametroID(ctx, 0, codigoEstadoDocumentoDefault)
	if err != nil {
		return nil, err
	}
	estudiantes, err := consultarEstudiantesAprobadosPorFacultadLigero(ctx, secretario.FacultadID, input.PeriodoID)
	if err != nil {
		return nil, err
	}

	estudiantesPorPrograma := map[int64][]EstudianteAprobadoGrado{}
	for _, estudiante := range estudiantes {
		if _, ok := programas[estudiante.IDProyectoOikos]; ok {
			estudiantesPorPrograma[estudiante.IDProyectoOikos] = append(estudiantesPorPrograma[estudiante.IDProyectoOikos], estudiante)
		}
	}
	totalSolicitado := 0
	for programaID := range programas {
		if yaAsignado, err := programaAsignadoEnFacultad(ctx, secretario.FacultadID, programaID, input.PeriodoID); err != nil {
			return nil, err
		} else if yaAsignado {
			return nil, fmt.Errorf("%w: el programa %d ya esta asignado a una ceremonia", ErrInvalidState, programaID)
		}
		cantidad := len(estudiantesPorPrograma[programaID])
		if cantidad == 0 {
			return nil, fmt.Errorf("%w: el programa %d no tiene estudiantes pendientes", ErrInvalidInput, programaID)
		}
		for _, estudiante := range estudiantesPorPrograma[programaID] {
			if estudiante.PeriodoID != input.PeriodoID {
				return nil, fmt.Errorf("%w: estudiante pertenece a otro período", ErrInvalidState)
			}
			yaAsignado, err := estudianteAsignadoEnPeriodo(ctx, estudiante.CodigoEstudiante, input.PeriodoID)
			if err != nil {
				return nil, err
			}
			if yaAsignado {
				return nil, fmt.Errorf("%w: estudiante %d ya está asignado en período %d", ErrInvalidState, estudiante.CodigoEstudiante, input.PeriodoID)
			}
		}

		totalSolicitado += cantidad
	}
	if totalSolicitado > disponibles {
		return nil, fmt.Errorf("%w: los programas seleccionados requieren %d cupos y la ceremonia tiene %d disponibles", ErrInvalidState, totalSolicitado, disponibles)
	}

	result := &AsignarEstudiantesCeremoniaResult{CeremoniaID: ceremoniaID}
	ids := make([]int64, 0, len(programas))
	for id := range programas {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, programaID := range ids {
		for idx, estudiante := range estudiantesPorPrograma[programaID] {
			if estudiante.PeriodoID != relacion.CeremoniaGrado.PeriodoID {
				result.Errores++
				continue
			}

			estudianteEnriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante)
			if err == nil {
				estudiante = estudianteEnriquecido
				estudiantesPorPrograma[programaID][idx] = estudiante
			}
			documentoPayload := estudiante.DocumentoDigitalRequest(tipoDocumentoDigitalID, estadoDocumentoID)
			documento, err := buscarDocumentoDigitalExistente(ctx, documentoPayload)
			if err != nil {
				result.Errores++
				continue
			}
			if documento == nil {
				documento, err = crearDocumentoDigital(ctx, documentoPayload)
				if err != nil {
					result.Errores++
					continue
				}
			}
			if documento.PeriodoID == nil || *documento.PeriodoID != input.PeriodoID {
				result.Errores++
				continue
			}

			fechaCeremonia, err := time.Parse(time.RFC3339, relacion.CeremoniaGrado.FechaCeremonia)
			if err != nil {
				return nil, fmt.Errorf("%w: fecha de ceremonia inválida para reservar registro", ErrInvalidState)
			}
			if err := asegurarRegistroDiplomaEnFecha(ctx, documento, fechaCeremonia); err != nil {
				return nil, err
			}

			creado, err := crearAsignacionCeremoniaEstudiante(ctx, relacion.ID, documento.ID)
			if err != nil {
				result.Errores++
				continue
			}
			if !creado {
				result.Omitidos++
				continue
			}
			result.Asignados++
		}
	}
	return result, nil
}

func (s CeremoniaService) QuitarPrograma(ctx context.Context, ceremoniaID int64, input *QuitarProgramaCeremoniaInput) (*AsignarEstudiantesCeremoniaResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadSecretario <= 0 || input.ProgramaAcademicoID <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario and programa_academico_id are required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, ceremoniaID, secretario.FacultadID)
	if err != nil {
		return nil, err
	}
	asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
	if err != nil {
		return nil, err
	}
	programaAsignaciones := filtrarAsignacionesPrograma(asignaciones, input.ProgramaAcademicoID)
	flujo, err := estadoFlujoAsignaciones(ctx, programaAsignaciones)
	if err != nil {
		return nil, err
	}
	if flujo.Bloqueado {
		return nil, fmt.Errorf("%w: %s", ErrInvalidState, flujo.MotivoBloqueo)
	}
	result := &AsignarEstudiantesCeremoniaResult{CeremoniaID: ceremoniaID}
	for _, asignacion := range programaAsignaciones {
		status, err := request.DeleteWithContext(ctx, fmt.Sprintf("%s/ceremonia_estudiante/%d", diplomasCrudURL(), asignacion.ID), nil)
		if err != nil || status < 200 || status >= 300 {
			result.Errores++
			continue
		}
		result.Asignados++
	}
	return result, nil
}

func (s CeremoniaService) EstudiantesPrograma(ctx context.Context, input *EstudiantesProgramaCeremoniaInput) (*EstudiantesProgramaCeremoniaResult, error) {
	if input == nil || input.DocumentoIdentidadSecretario <= 0 || input.ProgramaAcademicoID <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_secretario and programa_academico_id are required", ErrInvalidInput)
	}
	secretario, err := s.secretarioAcademicoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	estudiantes, err := consultarEstudiantesAprobadosPorFacultadLigero(ctx, secretario.FacultadID, input.PeriodoID)
	if err != nil {
		return nil, err
	}
	filtrados := make([]EstudianteAprobadoGrado, 0)
	for _, estudiante := range estudiantes {
		if estudiante.IDProyectoOikos != input.ProgramaAcademicoID {
			continue
		}
		enriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante)
		if err != nil {
			// La vista de asignación no debe quedar bloqueada porque un estudiante
			// no tenga todavía datos completos para diploma. Se conserva el código
			// para permitir asignar el programa completo y revisar el caso después.
			filtrados = append(filtrados, estudiante)
			continue
		}
		filtrados = append(filtrados, enriquecido)
	}
	return &EstudiantesProgramaCeremoniaResult{ProgramaAcademicoID: input.ProgramaAcademicoID, Nombre: nombreDependenciaOikos(ctx, input.ProgramaAcademicoID), Total: len(filtrados), Estudiantes: filtrados}, nil
}

func programasSeleccionados(items []AsignarProgramaCeremoniaItem) map[int64]struct{} {
	result := map[int64]struct{}{}
	for _, item := range items {
		if item.ProgramaAcademicoID > 0 {
			result[item.ProgramaAcademicoID] = struct{}{}
		}
	}
	return result
}

func contarAsignadosRelacion(ctx context.Context, relacionID int64) (int, error) {
	endpoint := fmt.Sprintf("%s/ceremonia_estudiante/?query=%s&limit=-1", diplomasCrudURL(), url.QueryEscape(fmt.Sprintf("ceremonia_facultad_programa_id:%d,activo:true", relacionID)))
	var asignados []ceremoniaEstudianteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &asignados)
	if err != nil {
		return 0, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return 0, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante returned status %d", ErrExternalService, status)
	}
	return len(asignados), nil
}

func listarAsignacionesRelacion(ctx context.Context, relacionID int64) ([]ceremoniaEstudianteCRUDResponse, error) {
	endpoint := fmt.Sprintf("%s/ceremonia_estudiante/?query=%s&limit=-1", diplomasCrudURL(), url.QueryEscape(fmt.Sprintf("ceremonia_facultad_programa_id:%d,activo:true", relacionID)))
	var asignados []ceremoniaEstudianteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &asignados)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante returned status %d", ErrExternalService, status)
	}
	return asignados, nil
}

func programaAsignadoEnFacultad(ctx context.Context, facultadID int64, programaID int64, periodoID int64) (bool, error) {
	relaciones, err := (CeremoniaService{}).listarRelacionesFacultad(ctx, facultadID)
	if err != nil {
		return false, err
	}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || relacion.CeremoniaGrado.PeriodoID != periodoID {
			continue
		}
		asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
		if err != nil {
			return false, err
		}
		for _, asignacion := range asignaciones {
			if asignacion.DocumentoDigital != nil && asignacion.DocumentoDigital.ProgramaAcademicoID != nil && *asignacion.DocumentoDigital.ProgramaAcademicoID == programaID {
				return true, nil
			}
		}
	}
	return false, nil
}

func crearAsignacionCeremoniaEstudiante(ctx context.Context, relacionID int64, documentoID int64) (bool, error) {
	// La pareja es única incluso después del retiro lógico: consultar también inactivos.
	query := fmt.Sprintf("ceremonia_facultad_programa_id:%d,documento_digital_id:%d", relacionID, documentoID)
	endpoint := fmt.Sprintf("%s/ceremonia_estudiante/?query=%s&limit=1", diplomasCrudURL(), url.QueryEscape(query))
	var existentes []ceremoniaEstudianteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &existentes)
	if err != nil || status < 200 || status >= 300 {
		return false, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante buscar status %d: %v", ErrExternalService, status, err)
	}
	if len(existentes) > 0 && existentes[0].Activo {
		return false, nil
	}
	payload := ceremoniaEstudianteCRUDRequest{CeremoniaFacultadPrograma: ceremoniaFacultadProgramaRef{ID: relacionID}, DocumentoDigital: documentoDigitalRef{ID: documentoID}, Activo: true}
	var response ceremoniaEstudianteCRUDResponse
	if len(existentes) > 0 {
		existente := existentes[0]
		if existente.ID <= 0 {
			return false, fmt.Errorf("%w: asignación existente sin identificador", ErrExternalService)
		}
		payload.FechaCreacion = existente.FechaCreacion
		status, err = request.PutWithContext(ctx, fmt.Sprintf("%s/ceremonia_estudiante/%d", diplomasCrudURL(), existente.ID), payload, &response)
	} else {
		status, err = request.PostWithContext(ctx, diplomasCrudURL()+"/ceremonia_estudiante/", payload, &response)
	}
	if err != nil || status < 200 || status >= 300 {
		return false, fmt.Errorf("%w: diplomas_crud ceremonia_estudiante guardar status %d: %v", ErrExternalService, status, err)
	}
	if response.ID <= 0 {
		return false, fmt.Errorf("%w: asignación guardada sin identificador", ErrExternalService)
	}
	return true, nil
}

func (s CeremoniaService) secretarioAcademicoAutorizado(ctx context.Context, documento int64) (*FirmanteActivo, error) {
	secretaria, err := consultarSecretariaFacultadOikos(ctx, documento)
	if err != nil {
		return nil, err
	}
	firmante, err := firmanteDesdeSecretaria(secretaria)
	if err != nil {
		return nil, fmt.Errorf("%w: no fue posible determinar la facultad del secretario academico", ErrInvalidState)
	}
	return firmante, nil
}

func (s CeremoniaService) decanoAutorizado(ctx context.Context, documento int64) (*FirmanteActivo, error) {
	firmante, err := FirmanteService{}.ConsultarRolActivo(ctx, documento, time.Now())
	if err != nil {
		return nil, err
	}
	if firmante == nil || firmante.Rol != "decano" || firmante.Orden != 2 {
		return nil, ErrNoActiveRole
	}
	facultadID, err := consultarFacultadOikosArgo(ctx, firmante.Dependencia)
	if err != nil {
		return nil, fmt.Errorf("%w: no fue posible determinar la facultad del decano", ErrInvalidState)
	}
	firmante.FacultadID = facultadID
	return firmante, nil
}

func (s CeremoniaService) firmanteOrdenAutorizado(ctx context.Context, documento int64, rol string, orden int) (*FirmanteActivo, error) {
	firmante, err := FirmanteService{}.ConsultarRolActivo(ctx, documento, time.Now())
	if err != nil {
		return nil, err
	}
	if firmante == nil || firmante.Rol != rol || firmante.Orden != orden {
		return nil, ErrNoActiveRole
	}
	return firmante, nil
}

func (s CeremoniaService) listarRelacionesFacultad(ctx context.Context, facultadID int64) ([]ceremoniaFacultadProgramaCRUDResponse, error) {
	endpoint := fmt.Sprintf("%s/ceremonia_facultad_programa/?query=%s&limit=-1", diplomasCrudURL(), url.QueryEscape(fmt.Sprintf("facultad_id:%d,activo:true", facultadID)))
	var relaciones []ceremoniaFacultadProgramaCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &relaciones)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa returned status %d", ErrExternalService, status)
	}
	return relaciones, nil
}

func (s CeremoniaService) listarRelacionesActivas(ctx context.Context) ([]ceremoniaFacultadProgramaCRUDResponse, error) {
	endpoint := fmt.Sprintf("%s/ceremonia_facultad_programa/?query=%s&limit=-1", diplomasCrudURL(), url.QueryEscape("activo:true"))
	var relaciones []ceremoniaFacultadProgramaCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &relaciones)
	if err != nil {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa status %d: %v", ErrExternalService, status, err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: diplomas_crud ceremonia_facultad_programa returned status %d", ErrExternalService, status)
	}
	return relaciones, nil
}

func (s CeremoniaService) relacionCeremoniaAutorizada(ctx context.Context, ceremoniaID int64, facultadID int64) (*ceremoniaFacultadProgramaCRUDResponse, error) {
	relaciones, err := s.listarRelacionesFacultad(ctx, facultadID)
	if err != nil {
		return nil, err
	}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado != nil && relacion.CeremoniaGrado.ID == ceremoniaID {
			return &relacion, nil
		}
	}
	return nil, fmt.Errorf("%w: ceremonia no pertenece a la facultad del firmante", ErrNoActiveRole)
}

func consultarEstudiantesAprobadosPorFacultadLigero(ctx context.Context, facultadID int64, periodoID int64) ([]EstudianteAprobadoGrado, error) {
	estudiantes, err := consultarAprobadosMID(ctx, periodoID)
	if err != nil {
		return nil, err
	}
	return filtrarEstudiantesAprobados(estudiantes, ListarAprobadosPorFacultadInput{FacultadID: facultadID}), nil
}

func programasAprobadosResumen(ctx context.Context, facultadID, periodoID int64) ([]ProgramaAprobadoResumen, int, error) {
	key := [2]int64{periodoID, facultadID}
	programasAprobadosCache.RLock()
	entry, ok := programasAprobadosCache.items[key]
	programasAprobadosCache.RUnlock()
	if ok && time.Now().Before(entry.expira) {
		return entry.programas, entry.total, nil
	}

	estudiantes, err := consultarEstudiantesAprobadosPorFacultadLigero(ctx, facultadID, periodoID)
	if err != nil {
		return nil, 0, err
	}
	conteo := map[int64]int{}
	total := 0
	for _, estudiante := range estudiantes {
		if facultadID > 0 && estudiante.IDFacultadOikos != facultadID {
			continue
		}
		conteo[estudiante.IDProyectoOikos]++
		total++
	}
	ids := make([]int64, 0, len(conteo))
	for id := range conteo {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	programas := make([]ProgramaAprobadoResumen, 0, len(ids))
	for _, id := range ids {
		nombre := nombreDependenciaOikos(ctx, id)
		if nombre == "" {
			nombre = nombreProgramaFallback(id)
		}
		programas = append(programas, ProgramaAprobadoResumen{PeriodoID: periodoID, ProgramaAcademicoID: id, Nombre: nombre, Cantidad: conteo[id]})
	}
	programasAprobadosCache.Lock()
	programasAprobadosCache.items[key] = programasAprobadosCacheEntry{programas: programas, total: total, expira: time.Now().Add(5 * time.Minute)}
	programasAprobadosCache.Unlock()
	return programas, total, nil
}

type estadoFlujoPrograma struct {
	Estado        string
	Bloqueado     bool
	Finalizado    bool
	MotivoBloqueo string
}

func estadoFlujoAsignaciones(ctx context.Context, asignaciones []ceremoniaEstudianteCRUDResponse) (*estadoFlujoPrograma, error) {
	result := &estadoFlujoPrograma{Estado: "ASIGNADO"}
	if len(asignaciones) == 0 {
		result.Estado = "PARAMETRIZADA"
		return result, nil
	}
	pendienteID, _ := resolverParametroID(ctx, 0, codigoEstadoDocumentoDefault)
	firmadoID, _ := resolverParametroID(ctx, 0, "FIRMADO")
	firmados := 0
	for _, asignacion := range asignaciones {
		if asignacion.DocumentoDigital == nil {
			continue
		}
		diploma, err := buscarDiplomaDigitalExistente(ctx, asignacion.DocumentoDigital.ID)
		if err != nil {
			return nil, err
		}
		if firmadoID > 0 && asignacion.DocumentoDigital.EstadoDocumentoID == firmadoID {
			firmados++
			result.Bloqueado = true
			continue
		}
		if diploma != nil || (pendienteID > 0 && asignacion.DocumentoDigital.EstadoDocumentoID != pendienteID) {
			result.Bloqueado = true
		}
	}
	if result.Bloqueado {
		result.Estado = "EN_GENERACION"
		result.MotivoBloqueo = "ya existen diplomas generados o en proceso de firma"
	}
	if firmados == len(asignaciones) && len(asignaciones) > 0 {
		result.Estado = "FINALIZADO"
		result.Finalizado = true
		result.MotivoBloqueo = "los diplomas del programa ya fueron firmados"
	}
	return result, nil
}

func filtrarAsignacionesPrograma(asignaciones []ceremoniaEstudianteCRUDResponse, programaID int64) []ceremoniaEstudianteCRUDResponse {
	filtradas := make([]ceremoniaEstudianteCRUDResponse, 0)
	for _, asignacion := range asignaciones {
		if asignacion.DocumentoDigital == nil || asignacion.DocumentoDigital.ProgramaAcademicoID == nil || *asignacion.DocumentoDigital.ProgramaAcademicoID != programaID {
			continue
		}
		filtradas = append(filtradas, asignacion)
	}
	return filtradas
}

func aplicarBloqueosCeremonias(ceremonias []CeremoniaResumen, programas []ProgramaAprobadoResumen) {
	bloqueadas := map[int64]string{}
	for _, programa := range programas {
		if !programa.PuedeReasignar {
			bloqueadas[programa.CeremoniaID] = programa.MotivoBloqueo
		}
	}
	for i := range ceremonias {
		if motivo, ok := bloqueadas[ceremonias[i].ID]; ok {
			ceremonias[i].EstadoFlujo = "EN_GENERACION"
			ceremonias[i].PuedeEditar = false
			ceremonias[i].PuedeEliminar = false
			ceremonias[i].MotivoBloqueo = motivo
		}
	}
}

func programasAsignadosResumen(ctx context.Context, relaciones []ceremoniaFacultadProgramaCRUDResponse) ([]ProgramaAprobadoResumen, error) {
	porPrograma := map[[2]int64]*ProgramaAprobadoResumen{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo {
			continue
		}
		asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
		if err != nil {
			return nil, err
		}
		for _, asignacion := range asignaciones {
			if asignacion.DocumentoDigital == nil || asignacion.DocumentoDigital.ProgramaAcademicoID == nil {
				continue
			}
			programaID := *asignacion.DocumentoDigital.ProgramaAcademicoID
			key := [2]int64{relacion.CeremoniaGrado.ID, programaID}
			item := porPrograma[key]
			if item == nil {
				item = &ProgramaAprobadoResumen{PeriodoID: relacion.CeremoniaGrado.PeriodoID, ProgramaAcademicoID: programaID, Nombre: nombreDependenciaOikos(ctx, programaID), CeremoniaID: relacion.CeremoniaGrado.ID, RelacionID: relacion.ID, Lugar: relacion.CeremoniaGrado.Lugar, FechaCeremonia: relacion.CeremoniaGrado.FechaCeremonia}
				if item.Nombre == "" {
					item.Nombre = nombreProgramaFallback(programaID)
				}
				porPrograma[key] = item
			}
			item.Cantidad++
		}
	}
	for _, item := range porPrograma {
		asignaciones, err := listarAsignacionesRelacion(ctx, item.RelacionID)
		if err != nil {
			return nil, err
		}
		flujo, err := estadoFlujoAsignaciones(ctx, filtrarAsignacionesPrograma(asignaciones, item.ProgramaAcademicoID))
		if err != nil {
			return nil, err
		}
		item.EstadoFlujo = flujo.Estado
		item.PuedeReasignar = !flujo.Bloqueado
		item.MotivoBloqueo = flujo.MotivoBloqueo
	}
	result := make([]ProgramaAprobadoResumen, 0, len(porPrograma))
	for _, item := range porPrograma {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CeremoniaID != result[j].CeremoniaID {
			return result[i].CeremoniaID < result[j].CeremoniaID
		}
		return result[i].ProgramaAcademicoID < result[j].ProgramaAcademicoID
	})
	return result, nil
}

func programasPendientes(aprobados []ProgramaAprobadoResumen, asignados []ProgramaAprobadoResumen) []ProgramaAprobadoResumen {
	asignadosMap := map[[2]int64]struct{}{}
	for _, item := range asignados {
		asignadosMap[[2]int64{item.PeriodoID, item.ProgramaAcademicoID}] = struct{}{}
	}
	pendientes := make([]ProgramaAprobadoResumen, 0, len(aprobados))
	for _, item := range aprobados {
		if _, ok := asignadosMap[[2]int64{item.PeriodoID, item.ProgramaAcademicoID}]; ok {
			continue
		}
		pendientes = append(pendientes, item)
	}
	return pendientes
}

func (s CeremoniaService) rollbackCeremonia(ctx context.Context, ceremoniaID int64, relaciones []ceremoniaFacultadProgramaCRUDResponse) {
	for _, relacion := range relaciones {
		if relacion.ID > 0 {
			_, _ = request.DeleteWithContext(ctx, fmt.Sprintf("%s/ceremonia_facultad_programa/%d", diplomasCrudURL(), relacion.ID), nil)
		}
	}
	_, _ = request.DeleteWithContext(ctx, fmt.Sprintf("%s/ceremonia_grado/%d", diplomasCrudURL(), ceremoniaID), nil)
}

func validarCeremonia(input *CrearCeremoniaInput) error {
	if input == nil {
		return fmt.Errorf("%w: request body is required", ErrInvalidInput)
	}
	if input.PeriodoID <= 0 {
		return fmt.Errorf("%w: periodo_id es obligatorio para configurar ceremonia", ErrInvalidInput)
	}

	if input.DocumentoIdentidadSecretario <= 0 {
		return fmt.Errorf("%w: documento_identidad_secretario is required", ErrInvalidInput)
	}
	if strings.TrimSpace(input.FechaCeremonia) == "" {
		return fmt.Errorf("%w: fecha_ceremonia is required", ErrInvalidInput)
	}
	if _, err := time.Parse("2006-01-02", input.FechaCeremonia); err != nil {
		return fmt.Errorf("%w: fecha_ceremonia must use YYYY-MM-DD", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Lugar) == "" {
		return fmt.Errorf("%w: lugar is required", ErrInvalidInput)
	}
	if input.CantidadEstudiantesProgramada <= 0 {
		return fmt.Errorf("%w: cantidad_estudiantes_programada must be greater than zero", ErrInvalidInput)
	}
	return nil
}

func catalogoEndpoint(base string, path string, query string) string {
	if query == "" {
		return base + path
	}
	return fmt.Sprintf("%s%s?query=%s", base, path, url.QueryEscape(query))
}

func validarPeriodoAsignacion(ceremonia *ceremoniaGradoCRUDResponse, periodoID int64) error {
	if ceremonia == nil || ceremonia.PeriodoID <= 0 {
		return fmt.Errorf("%w: configure el período de la ceremonia antes de asignar", ErrInvalidState)
	}
	if periodoID <= 0 || periodoID != ceremonia.PeriodoID {
		return fmt.Errorf("%w: período solicitado no coincide con ceremonia", ErrInvalidInput)
	}
	return nil
}

func estudianteAsignadoEnPeriodo(ctx context.Context, codigo, periodoID int64) (bool, error) {
	endpoint := diplomasCrudURL() + "/ceremonia_estudiante/?query=" + url.QueryEscape(fmt.Sprintf("codigo_estudiante:%d,periodo_id:%d,activo:true", codigo, periodoID)) + "&limit=-1"
	var asignaciones []ceremoniaEstudianteCRUDResponse
	status, err := request.GetWithContext(ctx, endpoint, &asignaciones)
	if err != nil || status < 200 || status >= 300 {
		return false, fmt.Errorf("%w: verificar estudiante/período asignado status %d: %v", ErrExternalService, status, err)
	}
	return len(asignaciones) > 0, nil
}
