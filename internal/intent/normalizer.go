package intent

import (
	"strings"
)

// CleanCommand limpia puntuación y remueve letras remanentes (como 'o' en 'Ronaldo') para extraer el comando de voz limpio.
func CleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	// Remover signos de puntuación iniciales/finales
	cmd = strings.TrimFunc(cmd, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':' || r == '-' || r == ')' || r == '(' || r == '\'' || r == '"'
	})
	cmd = strings.TrimSpace(cmd)

	// Manejar el caso de "Ronaldo" cuando la wake word es "ronald"
	// Si el comando restante empieza con "o " o "o," o es exactamente "o"
	if cmd == "o" {
		cmd = ""
	} else if strings.HasPrefix(cmd, "o ") {
		cmd = strings.TrimPrefix(cmd, "o ")
	} else if strings.HasPrefix(cmd, "o,") {
		cmd = strings.TrimPrefix(cmd, "o,")
	}

	// Limpiar puntuación residual de nuevo
	cmd = strings.TrimFunc(cmd, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':' || r == '-' || r == ')' || r == '(' || r == '\'' || r == '"'
	})
	return strings.TrimSpace(cmd)
}

// IsWhisperHallucination detecta y descarta alucinaciones típicas de Whisper.
func IsWhisperHallucination(text string) bool {
	t := strings.ToLower(strings.Trim(text, " .,?!¡¿"))
	if t == "" || t == "blank_audio" || t == "gracias" || t == "gracias por ver" ||
		t == "subtítulos" || t == "subtítulos por" || t == "subtitles" || t == "subtitles by" ||
		t == "thank you" || t == "thank you for watching" || t == "amara" || t == "amara.org" ||
		t == "y" || t == "sí" || t == "si" || t == "hola" || t == "adiós" || t == "bye" {
		return true
	}
	// Si contiene frases específicas de subtítulos de Whisper
	if strings.Contains(t, "subtítulos por") || strings.Contains(t, "subtitles by") || strings.Contains(t, "amara.org") {
		return true
	}
	return false
}

// SplitMultiIntent separa una oración del usuario en intenciones múltiples solo si se detectan verbos de acción.
func SplitMultiIntent(input string) []string {
	// Lista de verbos de acción fuertes para dividir tareas.
	// Se buscarán conector + verbo, ej: " y abre", " luego busca"
	actionVerbs := []string{
		"abre", "cierra", "busca", "lee", "crea", "elimina", "pon", "reproduce", "copia", "mueve", "ejecuta", "compila",
	}

	var parts []string
	inputLower := strings.ToLower(input)

	// Un separador simple que busca ", [verbo]" o " y [verbo]" o " luego [verbo]"
	connectors := []string{", ", " y ", " luego ", " después "}

	lastCut := 0
	for i := 0; i < len(inputLower); i++ {
		// Intentar buscar conectores
		for _, conn := range connectors {
			if strings.HasPrefix(inputLower[i:], conn) {
				// Revisar si justo después del conector viene un verbo de acción
				afterConnIdx := i + len(conn)
				for _, verb := range actionVerbs {
					if strings.HasPrefix(inputLower[afterConnIdx:], verb+" ") || inputLower[afterConnIdx:] == verb {
						// Separar
						parts = append(parts, strings.TrimSpace(input[lastCut:i]))
						lastCut = afterConnIdx // el inicio del nuevo comando es el verbo
						i = afterConnIdx - 1
						break
					}
				}
			}
		}
	}

	if lastCut < len(input) {
		parts = append(parts, strings.TrimSpace(input[lastCut:]))
	}

	if len(parts) == 0 {
		return []string{input}
	}
	return parts
}
