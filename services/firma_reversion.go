package services

import "fmt"

// validarReversionFirma uses the same role ordering as signature registration.
// Only active signatures actually recorded on this document can block reversal.
func validarReversionFirma(firmas []firmaDocumentoResponse, orden int, documento int64) error {
	propia := false
	for _, firma := range firmas {
		if !firma.Activo {
			continue
		}
		firmaOrden, ok := ordenRolFirmante(firma.RolFirmanteID)
		if !ok {
			return fmt.Errorf("%w: no se puede verificar el orden de una firma registrada", ErrInvalidState)
		}
		if firmaOrden > orden {
			return fmt.Errorf("%w: no se puede revertir porque el diploma ya tiene una firma posterior", ErrInvalidState)
		}
		if firmaOrden == orden {
			if firma.DocumentoIdentidad != documento {
				return fmt.Errorf("%w: solo el titular puede revertir su firma", ErrNoActiveRole)
			}
			propia = true
		}
	}
	if !propia {
		return fmt.Errorf("%w: no existe una firma activa propia para revertir", ErrInvalidState)
	}
	return nil
}

func idsFirmasActivas(firmas []firmaDocumentoResponse) []int64 {
	ids := make([]int64, 0, len(firmas))
	for _, firma := range firmas {
		if firma.Activo {
			ids = append(ids, firma.ID)
		}
	}
	return ids
}

type revertirFirmaDocumentoRequest struct {
	DocumentoIdentidad int64   `json:"documento_identidad"`
	RolFirmanteID      int64   `json:"rol_firmante_id"`
	FirmasEsperadas    []int64 `json:"firmas_esperadas"`
	EstadoNuevoID      int64   `json:"estado_nuevo_id"`
}
