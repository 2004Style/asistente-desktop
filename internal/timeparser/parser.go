package timeparser

import (
	"time"
)

// ParsedTime contiene el resultado estructurado de analizar una expresión temporal natural.
type ParsedTime struct {
	StartAt        time.Time // Fecha/hora resuelta en tiempo local
	RecurrenceRule string    // Regla de recurrencia (formato RRULE o vacío)
	Confidence     float64   // Confianza del análisis (0.0 a 1.0)
	Ambiguous      bool      // true si falta información y se usó un valor por defecto
	Reason         string    // Motivo por el cual es ambiguo (si aplica)
}

// timeNow es una variable para poder mockear el tiempo actual en los tests.
var timeNow = time.Now
