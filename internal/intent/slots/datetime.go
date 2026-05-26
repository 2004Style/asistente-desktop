package slots

import (
	"regexp"
	"strings"
)

// ExtractDateTimeSlots extrae slots de fecha/hora y duración.
// Retorna (datetime, duration)
func ExtractDateTimeSlots(input string) (string, string) {
	inputLower := strings.ToLower(input)
	var datetimeVal, durationVal string

	// Expresión para duraciones simples como "5 minutos", "10 segundos", "2 horas", "1 hora"
	durationRegex := regexp.MustCompile(`(\d+\s+(?:segundo|minuto|hora|día)s?)`)
	if match := durationRegex.FindString(inputLower); match != "" {
		durationVal = match
	}

	// Expresión para fechas/horas
	// "mañana", "hoy a las 5pm", "a las 15:30", "el 25 de mayo"
	datetimeRegexes := []*regexp.Regexp{
		regexp.MustCompile(`(mañana(?:\s+a\s+las\s+\d+(?::\d+)?(?:\s*(?:am|pm))?)?)`),
		regexp.MustCompile(`(hoy\s+a\s+las\s+\d+(?::\d+)?(?:\s*(?:am|pm))?)`),
		regexp.MustCompile(`(?:a\s+las\s+)(\d+(?::\d+)?(?:\s*(?:am|pm))?)`),
		regexp.MustCompile(`(el\s+\d+\s+de\s+[a-z]+)`),
	}

	for _, re := range datetimeRegexes {
		if match := re.FindString(inputLower); match != "" {
			// Si la coincidencia contiene un prefijo como "a las ", lo limpiamos o mantenemos según el caso
			if strings.HasPrefix(match, "a las ") {
				datetimeVal = match
			} else {
				datetimeVal = match
			}
			break
		}
	}

	// Si no se encuentra patrón de fecha/hora pero hay un "en 5 minutos", el "en 5 minutos" puede ser datetime
	if datetimeVal == "" && strings.Contains(inputLower, "en ") {
		inTimeRegex := regexp.MustCompile(`en\s+(\d+\s+(?:segundo|minuto|hora|día)s?)`)
		if matches := inTimeRegex.FindStringSubmatch(inputLower); len(matches) > 1 {
			datetimeVal = "en " + matches[1]
		}
	}

	return datetimeVal, durationVal
}
