package services

import (
	"context"
	"sort"
)

// tableroFirmasFacultad mantiene intacto el tablero de gestión (sin la opción
// resumen_firmas). No consulta aprobados, perfiles, programas, PDF ni S3.
func (s CeremoniaService) tableroFirmasFacultad(ctx context.Context, facultadID int64, orden int, periodoID int64) (*TableroCeremoniasResult, error) {
	relaciones, err := s.listarRelacionesFacultad(ctx, facultadID)
	if err != nil {
		return nil, err
	}
	filtradas := make([]ceremoniaFacultadProgramaCRUDResponse, 0, len(relaciones))
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo || relacion.FacultadID != facultadID {
			continue
		}
		if periodoID > 0 && relacion.CeremoniaGrado.PeriodoID != periodoID {
			continue
		}
		filtradas = append(filtradas, relacion)
	}
	return construirTableroResumenFirmas(ctx, filtradas, facultadID, orden)
}

// El CRUD actual no ofrece consultas de firmas por lotes. Se consulta una vez
// cada relación y cada documento distinto, con caché limitada a esta petición.
// No se carga el detalle enriquecido de todas las ceremonias.
func construirTableroResumenFirmas(ctx context.Context, relaciones []ceremoniaFacultadProgramaCRUDResponse, facultadID int64, orden int) (*TableroCeremoniasResult, error) {
	porCeremonia := map[int64]*CeremoniaResumen{}
	vistos := map[int64]map[int64]bool{}
	firmasPorDocumento := map[int64]map[int]bool{}
	relacionesVistas := map[int64]bool{}
	for _, relacion := range relaciones {
		if relacion.CeremoniaGrado == nil || !relacion.Activo || !relacion.CeremoniaGrado.Activo || relacionesVistas[relacion.ID] {
			continue
		}
		relacionesVistas[relacion.ID] = true
		ceremonia := relacion.CeremoniaGrado
		resumen := porCeremonia[ceremonia.ID]
		if resumen == nil {
			resumen = &CeremoniaResumen{
				ID: ceremonia.ID, PeriodoID: ceremonia.PeriodoID, RelacionID: relacion.ID,
				FacultadID: facultadID, FechaCeremonia: ceremonia.FechaCeremonia,
				Lugar: ceremonia.Lugar, Observacion: ceremonia.Observacion, Activo: true,
				ResumenFirmas: &ResumenFirmasCeremonia{Orden: orden},
			}
			porCeremonia[ceremonia.ID] = resumen
			vistos[ceremonia.ID] = map[int64]bool{}
		}
		cantidad := ceremonia.CantidadEstudiantesProgramada
		if relacion.CantidadEstudiantesProgramada != nil && *relacion.CantidadEstudiantesProgramada > 0 {
			cantidad = *relacion.CantidadEstudiantesProgramada
		}
		resumen.CantidadEstudiantesProgramada += cantidad
		asignaciones, err := listarAsignacionesRelacion(ctx, relacion.ID)
		if err != nil {
			return nil, err
		}
		for _, asignacion := range asignaciones {
			doc := asignacion.DocumentoDigital
			if !asignacion.Activo || doc == nil || doc.ID <= 0 || vistos[ceremonia.ID][doc.ID] {
				continue
			}
			vistos[ceremonia.ID][doc.ID] = true
			resumen.CuposAsignados++
			ordenes, ok := firmasPorDocumento[doc.ID]
			if !ok {
				firmas, err := listarFirmasDocumento(ctx, doc.ID)
				if err != nil {
					return nil, err
				}
				ordenes = map[int]bool{}
				for _, firma := range firmas {
					if !firma.Activo {
						continue
					}
					if etapa, valida := ordenRolFirmante(firma.RolFirmanteID); valida {
						ordenes[etapa] = true
					}
				}
				firmasPorDocumento[doc.ID] = ordenes
			}
			if !tieneFirmasPrevias(ordenes, orden) {
				continue
			}
			resumen.ResumenFirmas.Total++
			if ordenes[orden] {
				resumen.ResumenFirmas.Firmados++
			} else {
				resumen.ResumenFirmas.Pendientes++
			}
		}
	}
	result := &TableroCeremoniasResult{FacultadID: facultadID, Ceremonias: []CeremoniaResumen{}, AprobadosProgramas: []ProgramaAprobadoResumen{}, PendientesProgramas: []ProgramaAprobadoResumen{}, AsignadosProgramas: []ProgramaAprobadoResumen{}}
	for _, resumen := range porCeremonia {
		// Una ceremonia sin diplomas habilitados no está completada para este actor.
		if resumen.ResumenFirmas.Total == 0 {
			continue
		}
		resumen.CuposDisponibles = resumen.CantidadEstudiantesProgramada - resumen.CuposAsignados
		if resumen.CuposDisponibles < 0 {
			resumen.CuposDisponibles = 0
		}
		resumen.EstadoFlujo = "EN_FIRMA"
		result.Ceremonias = append(result.Ceremonias, *resumen)
		result.TotalCeremonias++
		result.TotalCupos += resumen.CantidadEstudiantesProgramada
		result.CuposAsignados += resumen.CuposAsignados
		result.CuposDisponibles += resumen.CuposDisponibles
	}
	sort.Slice(result.Ceremonias, func(i, j int) bool { return result.Ceremonias[i].ID < result.Ceremonias[j].ID })
	return result, nil
}
