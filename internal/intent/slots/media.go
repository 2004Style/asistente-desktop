package slots

import (
	"strings"
)

// ExtractMediaSlots extrae slots relacionados con reproducción de medios.
// Retorna (query)
func ExtractMediaSlots(input string) string {
	inputLower := strings.ToLower(input)

	triggers := []string{
		"reproduce música de",
		"reproducir música de",
		"reproduce la canción",
		"reproducir la canción",
		"reproduce a",
		"reproducir a",
		"reproduce",
		"reproducir",
		"pon música de",
		"poner música de",
		"pon la canción",
		"poner la canción",
		"pon algo de",
		"poner algo de",
		"pon a",
		"poner a",
		"pon",
		"poner",
	}

	for _, trigger := range triggers {
		if idx := strings.Index(inputLower, trigger); idx != -1 {
			queryPart := strings.TrimSpace(input[idx+len(trigger):])
			// Eliminar conectores iniciales innecesarios
			qpLower := strings.ToLower(queryPart)
			for _, conn := range []string{"de ", "a ", "la ", "el ", "los ", "las "} {
				if strings.HasPrefix(qpLower, conn) {
					queryPart = queryPart[len(conn):]
					qpLower = qpLower[len(conn):]
				}
			}
			if queryPart != "" {
				return strings.Trim(queryPart, " .,?!¡¿")
			}
		}
	}

	return ""
}
