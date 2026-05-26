package timeparser

import (
	"strings"
	"time"
)

// SpanishDays mapea nombres de días en español a su correspondiente time.Weekday.
var SpanishDays = map[string]time.Weekday{
	"lunes":     time.Monday,
	"martes":    time.Tuesday,
	"miercoles": time.Wednesday,
	"miércoles": time.Wednesday,
	"jueves":    time.Thursday,
	"viernes":   time.Friday,
	"sabado":    time.Saturday,
	"sábado":    time.Saturday,
	"domingo":   time.Sunday,
}

// NormalizeSpanish limpia y normaliza la frase para facilitar la coincidencia de patrones.
func NormalizeSpanish(input string) string {
	cleaned := strings.ToLower(strings.TrimSpace(input))
	// Reemplazar acentos comunes que puedan interferir en expresiones regulares
	cleaned = strings.ReplaceAll(cleaned, "á", "a")
	cleaned = strings.ReplaceAll(cleaned, "é", "e")
	cleaned = strings.ReplaceAll(cleaned, "í", "i")
	cleaned = strings.ReplaceAll(cleaned, "ó", "o")
	cleaned = strings.ReplaceAll(cleaned, "ú", "u")
	cleaned = strings.ReplaceAll(cleaned, "ü", "u")
	cleaned = strings.ReplaceAll(cleaned, "ñ", "n")
	
	// Quitar signos residuales
	cleaned = strings.TrimFunc(cleaned, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == '¿' || r == '¡'
	})
	return cleaned
}
