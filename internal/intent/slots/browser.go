package slots

import (
	"regexp"
	"strings"
)

// ExtractBrowserSlots extrae slots relacionados con el navegador.
// Retorna (url, query)
func ExtractBrowserSlots(input string) (string, string) {
	inputLower := strings.ToLower(input)
	var urlVal, queryVal string

	// Buscar URLs: http(s)://... o www.... o dominios comunes
	urlRegex := regexp.MustCompile(`(https?://[^\s,]+|www\.[^\s,]+|[a-zA-Z0-9.-]+\.(com|org|net|io|edu|gov|es|co)(/[^\s,]*)*)`)
	if match := urlRegex.FindString(inputLower); match != "" {
		// Encontrar la coincidencia original (preservando mayúsculas)
		idx := strings.Index(inputLower, match)
		if idx != -1 {
			urlVal = input[idx : idx+len(match)]
		} else {
			urlVal = match
		}
	}

	// Buscar consultas de búsqueda.
	// Ejemplos: "busca en google documentación de go", "busca recetas de cocina", "busca cómo usar docker"
	searchTriggers := []string{"busca en google", "buscar en google", "busca", "buscar", "encuentra", "googlea"}
	for _, trigger := range searchTriggers {
		if idx := strings.Index(inputLower, trigger); idx != -1 {
			// Extraer la parte posterior al trigger, eliminando conectores como "de", "sobre", "en"
			queryPart := strings.TrimSpace(input[idx+len(trigger):])
			
			// Si empieza con conectores comunes, los quitamos
			queryPartLower := strings.ToLower(queryPart)
			for _, conn := range []string{"sobre ", "de ", "en ", "para ", "cómo "} {
				if strings.HasPrefix(queryPartLower, conn) {
					queryPart = queryPart[len(conn):]
					queryPartLower = queryPartLower[len(conn):]
				}
			}
			
			// Si la consulta no es una URL, la asignamos
			if queryVal == "" && queryPart != "" && !strings.Contains(strings.ToLower(queryPart), urlVal) {
				queryVal = strings.Trim(queryPart, " .,?!¡¿")
				break
			}
		}
	}

	return urlVal, queryVal
}
