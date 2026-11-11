package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	rolSecretariaGeneral = "secretaria_general"
	rolRector            = "rector"
	ordenSecretario      = 1
	ordenDecano          = 2
	ordenSecretariaGral  = 3
	ordenRector          = 4
)

func (s CeremoniaService) ListarFirmas(ctx context.Context, ceremoniaID int64, input *ListarCeremoniasInput) (*CeremoniaFirmasResult, error) {
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
	return construirCeremoniaFirmasResult(ctx, relacion, ordenSecretario)
}

func (s CeremoniaService) FirmarMasivamente(ctx context.Context, ceremoniaID int64, input *FirmaMasivaCeremoniaInput) (*FirmaMasivaCeremoniaResult, error) {
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
	firmas, err := construirCeremoniaFirmasResult(ctx, relacion, ordenSecretario)
	if err != nil {
		return nil, err
	}
	seleccion := map[int64]struct{}{}
	for _, id := range input.DocumentoDigitalIDs {
		if id > 0 {
			seleccion[id] = struct{}{}
		}
	}
	firmarTodos := len(seleccion) == 0
	result := &FirmaMasivaCeremoniaResult{CeremoniaID: ceremoniaID, Resultados: []FirmaMasivaCeremoniaItem{}}
	for _, estudiante := range firmas.Estudiantes {
		if !firmarTodos {
			if _, ok := seleccion[estudiante.DocumentoDigitalID]; !ok {
				continue
			}
		}
		result.Total++
		item := FirmaMasivaCeremoniaItem{DocumentoDigitalID: estudiante.DocumentoDigitalID}
		if !estudiante.PuedeFirmar {
			result.Omitidos++
			item.Error = estudiante.MotivoBloqueo
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma, err := FirmaDiplomaService{}.Firmar(ctx, estudiante.DocumentoDigitalID, &FirmaDiplomaInput{DocumentoIdentidadFirmante: input.DocumentoIdentidadSecretario})
		if err != nil {
			result.Errores++
			item.Error = err.Error()
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma.File = ""
		item.Firmado = true
		item.Result = firma
		result.Firmados++
		result.Resultados = append(result.Resultados, item)
	}
	return result, nil
}

func (s CeremoniaService) ListarFirmasDecano(ctx context.Context, ceremoniaID int64, input *ListarCeremoniasInput) (*CeremoniaFirmasResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_decano is required", ErrInvalidInput)
	}
	decano, err := s.decanoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, ceremoniaID, decano.FacultadID)
	if err != nil {
		return nil, err
	}
	return construirCeremoniaFirmasResult(ctx, relacion, ordenDecano)
}

func (s CeremoniaService) FirmarMasivamenteDecano(ctx context.Context, ceremoniaID int64, input *FirmaMasivaCeremoniaInput) (*FirmaMasivaCeremoniaResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	if input == nil || input.DocumentoIdentidadSecretario <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_decano is required", ErrInvalidInput)
	}
	decano, err := s.decanoAutorizado(ctx, input.DocumentoIdentidadSecretario)
	if err != nil {
		return nil, err
	}
	relacion, err := s.relacionCeremoniaAutorizada(ctx, ceremoniaID, decano.FacultadID)
	if err != nil {
		return nil, err
	}
	firmas, err := construirCeremoniaFirmasResult(ctx, relacion, ordenDecano)
	if err != nil {
		return nil, err
	}
	seleccion := map[int64]struct{}{}
	for _, id := range input.DocumentoDigitalIDs {
		if id > 0 {
			seleccion[id] = struct{}{}
		}
	}
	firmarTodos := len(seleccion) == 0
	result := &FirmaMasivaCeremoniaResult{CeremoniaID: ceremoniaID, Resultados: []FirmaMasivaCeremoniaItem{}}
	for _, estudiante := range firmas.Estudiantes {
		if !firmarTodos {
			if _, ok := seleccion[estudiante.DocumentoDigitalID]; !ok {
				continue
			}
		}
		result.Total++
		item := FirmaMasivaCeremoniaItem{DocumentoDigitalID: estudiante.DocumentoDigitalID}
		if !estudiante.PuedeFirmar {
			result.Omitidos++
			item.Error = estudiante.MotivoBloqueo
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma, err := FirmaDiplomaService{}.Firmar(ctx, estudiante.DocumentoDigitalID, &FirmaDiplomaInput{DocumentoIdentidadFirmante: input.DocumentoIdentidadSecretario})
		if err != nil {
			result.Errores++
			item.Error = err.Error()
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma.File = ""
		item.Firmado = true
		item.Result = firma
		result.Firmados++
		result.Resultados = append(result.Resultados, item)
	}
	return result, nil
}

func (s CeremoniaService) ListarSecretariaGeneral(ctx context.Context, input *ListarCeremoniasInput) (*TableroCeremoniasResult, error) {
	documento := documentoListarFirmante(input)
	if documento <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	if _, err := s.firmanteOrdenAutorizado(ctx, documento, rolSecretariaGeneral, ordenSecretariaGral); err != nil {
		return nil, err
	}
	return s.tableroFirmasGlobal(ctx, ordenSecretariaGral)
}

func (s CeremoniaService) ListarRector(ctx context.Context, input *ListarCeremoniasInput) (*TableroCeremoniasResult, error) {
	documento := documentoListarFirmante(input)
	if documento <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	if _, err := s.firmanteOrdenAutorizado(ctx, documento, rolRector, ordenRector); err != nil {
		return nil, err
	}
	return s.tableroFirmasGlobal(ctx, ordenRector)
}

func (s CeremoniaService) ListarFirmasSecretariaGeneral(ctx context.Context, ceremoniaID int64, input *ListarCeremoniasInput) (*CeremoniaFirmasResult, error) {
	return s.listarFirmasGlobal(ctx, ceremoniaID, input, rolSecretariaGeneral, ordenSecretariaGral)
}

func (s CeremoniaService) FirmarMasivamenteSecretariaGeneral(ctx context.Context, ceremoniaID int64, input *FirmaMasivaCeremoniaInput) (*FirmaMasivaCeremoniaResult, error) {
	return s.firmarMasivamenteGlobal(ctx, ceremoniaID, input, rolSecretariaGeneral, ordenSecretariaGral)
}

func (s CeremoniaService) ListarFirmasRector(ctx context.Context, ceremoniaID int64, input *ListarCeremoniasInput) (*CeremoniaFirmasResult, error) {
	return s.listarFirmasGlobal(ctx, ceremoniaID, input, rolRector, ordenRector)
}

func (s CeremoniaService) FirmarMasivamenteRector(ctx context.Context, ceremoniaID int64, input *FirmaMasivaCeremoniaInput) (*FirmaMasivaCeremoniaResult, error) {
	return s.firmarMasivamenteGlobal(ctx, ceremoniaID, input, rolRector, ordenRector)
}

func (s CeremoniaService) listarFirmasGlobal(ctx context.Context, ceremoniaID int64, input *ListarCeremoniasInput, rol string, orden int) (*CeremoniaFirmasResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	documento := documentoListarFirmante(input)
	if documento <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	if _, err := s.firmanteOrdenAutorizado(ctx, documento, rol, orden); err != nil {
		return nil, err
	}
	relaciones, err := s.relacionesCeremoniaGlobal(ctx, ceremoniaID)
	if err != nil {
		return nil, err
	}
	return construirCeremoniaFirmasResultRelaciones(ctx, relaciones, orden)
}

func (s CeremoniaService) firmarMasivamenteGlobal(ctx context.Context, ceremoniaID int64, input *FirmaMasivaCeremoniaInput, rol string, orden int) (*FirmaMasivaCeremoniaResult, error) {
	if ceremoniaID <= 0 {
		return nil, fmt.Errorf("%w: ceremonia id is required", ErrInvalidInput)
	}
	documento := documentoFirmaInput(input)
	if documento <= 0 {
		return nil, fmt.Errorf("%w: documento_identidad_firmante is required", ErrInvalidInput)
	}
	if _, err := s.firmanteOrdenAutorizado(ctx, documento, rol, orden); err != nil {
		return nil, err
	}
	relaciones, err := s.relacionesCeremoniaGlobal(ctx, ceremoniaID)
	if err != nil {
		return nil, err
	}
	firmas, err := construirCeremoniaFirmasResultRelaciones(ctx, relaciones, orden)
	if err != nil {
		return nil, err
	}
	return firmarEstudiantesCeremonia(ctx, ceremoniaID, documento, input.DocumentoDigitalIDs, firmas.Estudiantes)
}

func (s CeremoniaService) tableroFirmasGlobal(ctx context.Context, orden int) (*TableroCeremoniasResult, error) {
	relaciones, err := s.listarRelacionesActivas(ctx)
	if err != nil {
		return nil, err
	}
	porCeremonia := map[int64][]ceremoniaFacultadProgramaCRUDResponse{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo {
			continue
		}
		porCeremonia[relacion.CeremoniaGrado.ID] = append(porCeremonia[relacion.CeremoniaGrado.ID], relacion)
	}
	ids := make([]int64, 0, len(porCeremonia))
	for id := range porCeremonia {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	result := &TableroCeremoniasResult{Ceremonias: []CeremoniaResumen{}, AprobadosProgramas: []ProgramaAprobadoResumen{}, PendientesProgramas: []ProgramaAprobadoResumen{}, AsignadosProgramas: []ProgramaAprobadoResumen{}}
	for _, id := range ids {
		detalle, err := construirCeremoniaFirmasResultRelaciones(ctx, porCeremonia[id], orden)
		if err != nil {
			return nil, err
		}
		if detalle.Total == 0 {
			continue
		}
		resumen := detalle.Ceremonia
		resumen.CuposAsignados = detalle.Total
		resumen.CuposDisponibles = detalle.Pendientes
		resumen.PuedeEditar = false
		resumen.PuedeEliminar = false
		resumen.EstadoFlujo = "EN_FIRMA"
		result.Ceremonias = append(result.Ceremonias, resumen)
		result.TotalCeremonias++
		result.TotalCupos += resumen.CantidadEstudiantesProgramada
		result.CuposAsignados += detalle.Total
		result.CuposDisponibles += detalle.Pendientes
	}
	return result, nil
}

func (s CeremoniaService) relacionesCeremoniaGlobal(ctx context.Context, ceremoniaID int64) ([]ceremoniaFacultadProgramaCRUDResponse, error) {
	relaciones, err := s.listarRelacionesActivas(ctx)
	if err != nil {
		return nil, err
	}
	result := []ceremoniaFacultadProgramaCRUDResponse{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado != nil && relacion.CeremoniaGrado.ID == ceremoniaID && relacion.Activo && relacion.CeremoniaGrado.Activo {
			result = append(result, relacion)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: ceremonia no encontrada", ErrInvalidInput)
	}
	return result, nil
}

func construirCeremoniaFirmasResultRelaciones(ctx context.Context, relaciones []ceremoniaFacultadProgramaCRUDResponse, ordenFirma int) (*CeremoniaFirmasResult, error) {
	result := &CeremoniaFirmasResult{Estudiantes: []CeremoniaFirmaEstudianteResumen{}}
	vistos := map[int64]struct{}{}
	for _, relacion := range relaciones {
		relacion := relacion
		detalle, err := construirCeremoniaFirmasResult(ctx, &relacion, ordenFirma)
		if err != nil {
			return nil, err
		}
		if result.Ceremonia.ID == 0 {
			result.Ceremonia = detalle.Ceremonia
			result.Ceremonia.RelacionID = 0
			result.Ceremonia.FacultadID = 0
			result.Ceremonia.CantidadEstudiantesProgramada = 0
			result.Ceremonia.CuposAsignados = 0
			result.Ceremonia.CuposDisponibles = 0
		}
		result.Ceremonia.CantidadEstudiantesProgramada += detalle.Ceremonia.CantidadEstudiantesProgramada
		result.Ceremonia.CuposAsignados += detalle.Ceremonia.CuposAsignados
		for _, estudiante := range detalle.Estudiantes {
			if _, ok := vistos[estudiante.DocumentoDigitalID]; ok {
				continue
			}
			vistos[estudiante.DocumentoDigitalID] = struct{}{}
			result.Estudiantes = append(result.Estudiantes, estudiante)
			result.Total++
			if firmadoEtapa(estudiante, ordenFirma) {
				result.Firmados++
			} else {
				result.Pendientes++
			}
		}
	}
	result.Ceremonia.CuposDisponibles = result.Pendientes
	sort.Slice(result.Estudiantes, func(i, j int) bool {
		if result.Estudiantes[i].ProgramaAcademicoID == result.Estudiantes[j].ProgramaAcademicoID {
			return result.Estudiantes[i].CodigoEstudiante < result.Estudiantes[j].CodigoEstudiante
		}
		return result.Estudiantes[i].ProgramaAcademicoID < result.Estudiantes[j].ProgramaAcademicoID
	})
	return result, nil
}

func firmarEstudiantesCeremonia(ctx context.Context, ceremoniaID int64, documentoFirmante int64, ids []int64, estudiantes []CeremoniaFirmaEstudianteResumen) (*FirmaMasivaCeremoniaResult, error) {
	seleccion := map[int64]struct{}{}
	for _, id := range ids {
		if id > 0 {
			seleccion[id] = struct{}{}
		}
	}
	firmarTodos := len(seleccion) == 0
	result := &FirmaMasivaCeremoniaResult{CeremoniaID: ceremoniaID, Resultados: []FirmaMasivaCeremoniaItem{}}
	for _, estudiante := range estudiantes {
		if !firmarTodos {
			if _, ok := seleccion[estudiante.DocumentoDigitalID]; !ok {
				continue
			}
		}
		result.Total++
		item := FirmaMasivaCeremoniaItem{DocumentoDigitalID: estudiante.DocumentoDigitalID}
		if !estudiante.PuedeFirmar {
			result.Omitidos++
			item.Error = estudiante.MotivoBloqueo
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma, err := FirmaDiplomaService{}.Firmar(ctx, estudiante.DocumentoDigitalID, &FirmaDiplomaInput{DocumentoIdentidadFirmante: documentoFirmante})
		if err != nil {
			result.Errores++
			item.Error = err.Error()
			result.Resultados = append(result.Resultados, item)
			continue
		}
		firma.File = ""
		item.Firmado = true
		item.Result = firma
		result.Firmados++
		result.Resultados = append(result.Resultados, item)
	}
	result.Message = resumenErroresFirmaMasiva(result.Resultados)
	return result, nil
}

func resumenErroresFirmaMasiva(items []FirmaMasivaCeremoniaItem) string {
	errores := make([]string, 0)
	vistos := map[string]struct{}{}
	for _, item := range items {
		mensaje := strings.TrimSpace(item.Error)
		if mensaje == "" {
			continue
		}
		if _, ok := vistos[mensaje]; ok {
			continue
		}
		vistos[mensaje] = struct{}{}
		errores = append(errores, mensaje)
	}
	return strings.Join(errores, " ")
}

func construirCeremoniaFirmasResult(ctx context.Context, relacion *ceremoniaFacultadProgramaCRUDResponse, ordenFirma int) (*CeremoniaFirmasResult, error) {
	if relacion == nil || relacion.CeremoniaGrado == nil {
		return nil, fmt.Errorf("%w: ceremonia no encontrada", ErrInvalidInput)
	}
	asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
	if err != nil {
		return nil, err
	}
	cantidad := relacion.CeremoniaGrado.CantidadEstudiantesProgramada
	if relacion.CantidadEstudiantesProgramada != nil && *relacion.CantidadEstudiantesProgramada > 0 {
		cantidad = *relacion.CantidadEstudiantesProgramada
	}
	resumen := CeremoniaResumen{
		ID:                            relacion.CeremoniaGrado.ID,
		PeriodoID:                     relacion.CeremoniaGrado.PeriodoID,
		RelacionID:                    relacion.ID,
		FacultadID:                    relacion.FacultadID,
		FechaCeremonia:                relacion.CeremoniaGrado.FechaCeremonia,
		Lugar:                         relacion.CeremoniaGrado.Lugar,
		CantidadEstudiantesProgramada: cantidad,
		CuposAsignados:                len(asignaciones),
		CuposDisponibles:              cantidad - len(asignaciones),
		EstadoFlujo:                   "ASIGNADA",
		PuedeEditar:                   len(asignaciones) == 0,
		PuedeEliminar:                 len(asignaciones) == 0,
		Observacion:                   relacion.CeremoniaGrado.Observacion,
		Activo:                        relacion.Activo && relacion.CeremoniaGrado.Activo,
	}
	if resumen.CuposDisponibles < 0 {
		resumen.CuposDisponibles = 0
	}
	result := &CeremoniaFirmasResult{Ceremonia: resumen, Estudiantes: []CeremoniaFirmaEstudianteResumen{}}
	for _, asignacion := range asignaciones {
		if asignacion.DocumentoDigital == nil || !asignacion.Activo {
			continue
		}
		doc := asignacion.DocumentoDigital

		// Enriquecer datos del estudiante si faltan (asignaciones antiguas sin DatosDiploma)
		needsEnrich := doc.NombreEstudiante == "" || doc.NumeroDocumentoEstudiante == ""
		if needsEnrich && doc.CodigoEstudiante > 0 {
			estudiante := EstudianteAprobadoGrado{
				CodigoEstudiante: doc.CodigoEstudiante,
				IDFacultadOikos:  0,
				IDProyectoOikos:  0,
			}
			if doc.FacultadID != nil {
				estudiante.IDFacultadOikos = *doc.FacultadID
			}
			if doc.ProgramaAcademicoID != nil {
				estudiante.IDProyectoOikos = *doc.ProgramaAcademicoID
			}
			if enriquecido, err := enriquecerEstudianteAprobado(ctx, estudiante); err == nil {
				facultadID := enriquecido.IDFacultadOikos
				proyectoID := enriquecido.IDProyectoOikos
				doc = &documentoDigitalResponse{
					ID:                        doc.ID,
					CodigoEstudiante:          enriquecido.CodigoEstudiante,
					FacultadID:                &facultadID,
					ProgramaAcademicoID:       &proyectoID,
					NombreEstudiante:          enriquecido.DatosDiploma.Nombre,
					TipoDocumentoEstudiante:   enriquecido.DatosDiploma.TipoDocumento,
					NumeroDocumentoEstudiante: enriquecido.DatosDiploma.NumeroIdentificacion,
					TituloOtorgado:            enriquecido.DatosDiploma.Titulo,
					EstadoDocumentoID:         doc.EstadoDocumentoID,
					UUIDDocumento:             doc.UUIDDocumento,
				}
			}
		}

		item := CeremoniaFirmaEstudianteResumen{
			AsignacionID:              asignacion.ID,
			DocumentoDigitalID:        doc.ID,
			CodigoEstudiante:          doc.CodigoEstudiante,
			NombreEstudiante:          doc.NombreEstudiante,
			TipoDocumentoEstudiante:   doc.TipoDocumentoEstudiante,
			NumeroDocumentoEstudiante: doc.NumeroDocumentoEstudiante,
			TituloOtorgado:            doc.TituloOtorgado,
			EstadoDocumentoID:         doc.EstadoDocumentoID,
			PuedeFirmar:               true,
		}
		if doc.ProgramaAcademicoID != nil {
			item.ProgramaAcademicoID = *doc.ProgramaAcademicoID
			item.Programa = nombreDependenciaOikos(ctx, *doc.ProgramaAcademicoID)
			if item.Programa == "" {
				item.Programa = nombreProgramaFallback(*doc.ProgramaAcademicoID)
			}
		}
		firmas, err := listarFirmasDocumento(ctx, doc.ID)
		if err != nil {
			return nil, err
		}
		ordenesFirmadas := map[int]bool{}
		for _, firma := range firmas {
			orden, ok := ordenRolFirmante(firma.RolFirmanteID)
			if !ok {
				continue
			}
			ordenesFirmadas[orden] = true
			if orden == ordenSecretario {
				item.FirmaSecretario = true
			}
			if orden == ordenDecano {
				item.FirmaDecano = true
			}
			if orden == ordenSecretariaGral {
				item.FirmaSecretariaGeneral = true
			}
			if orden == ordenRector {
				item.FirmaRector = true
			}
		}
		for orden := ordenSecretario; orden < ordenFirma; orden++ {
			if !ordenesFirmadas[orden] {
				continue
			}
		}
		if !tieneFirmasPrevias(ordenesFirmadas, ordenFirma) {
			continue
		}
		if firmadoEtapa(item, ordenFirma) {
			item.PuedeFirmar = false
			item.MotivoBloqueo = fmt.Sprintf("el diploma ya tiene firma de orden %d", ordenFirma)
		}
		if item.PuedeFirmar && doc.UUIDDocumento != nil && *doc.UUIDDocumento != "" {
			item.PuedeFirmar = false
			item.MotivoBloqueo = "el diploma ya fue finalizado"
		}
		if firmadoEtapa(item, ordenFirma) {
			result.Firmados++
		} else {
			result.Pendientes++
		}
		result.Total++
		result.Estudiantes = append(result.Estudiantes, item)
	}
	sort.Slice(result.Estudiantes, func(i, j int) bool {
		if result.Estudiantes[i].ProgramaAcademicoID == result.Estudiantes[j].ProgramaAcademicoID {
			return result.Estudiantes[i].CodigoEstudiante < result.Estudiantes[j].CodigoEstudiante
		}
		return result.Estudiantes[i].ProgramaAcademicoID < result.Estudiantes[j].ProgramaAcademicoID
	})
	return result, nil
}

func tieneFirmasPrevias(firmas map[int]bool, ordenFirma int) bool {
	for orden := ordenSecretario; orden < ordenFirma; orden++ {
		if !firmas[orden] {
			return false
		}
	}
	return true
}

func firmadoEtapa(estudiante CeremoniaFirmaEstudianteResumen, ordenFirma int) bool {
	switch ordenFirma {
	case ordenSecretario:
		return estudiante.FirmaSecretario
	case ordenDecano:
		return estudiante.FirmaDecano
	case ordenSecretariaGral:
		return estudiante.FirmaSecretariaGeneral
	case ordenRector:
		return estudiante.FirmaRector
	default:
		return false
	}
}

func documentoListarFirmante(input *ListarCeremoniasInput) int64 {
	if input == nil {
		return 0
	}
	if input.DocumentoIdentidadFirmante > 0 {
		return input.DocumentoIdentidadFirmante
	}
	return input.DocumentoIdentidadSecretario
}

func documentoFirmaInput(input *FirmaMasivaCeremoniaInput) int64 {
	if input == nil {
		return 0
	}
	if input.DocumentoIdentidadFirmante > 0 {
		return input.DocumentoIdentidadFirmante
	}
	return input.DocumentoIdentidadSecretario
}
