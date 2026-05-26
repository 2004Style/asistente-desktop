package intent

import (
	"strings"
)

// SplitMultiIntent separa una oración del usuario en intenciones múltiples
// si y solo si los conectores ("y", "luego", "después", comas) van seguidos de un verbo de acción.
func SplitMultiIntent(input string) []string {
	// Lista expandida de verbos de acción
	actionVerbs := []string{
		"abre", "cierra", "busca", "lee", "crea", "elimina", "pon", "reproduce",
		"copia", "mueve", "ejecuta", "compila", "presiona", "escribe", "notifica", "recuérdame",
	}

	var parts []string
	inputLower := strings.ToLower(input)

	// Conectores que pueden separar intenciones
	connectors := []string{", ", " y ", " luego ", " después "}

	lastCut := 0
	for i := 0; i < len(inputLower); i++ {
		for _, conn := range connectors {
			if strings.HasPrefix(inputLower[i:], conn) {
				afterConnIdx := i + len(conn)
				for _, verb := range actionVerbs {
					// Comprobar si el verbo viene seguido de un espacio o finaliza el input
					if strings.HasPrefix(inputLower[afterConnIdx:], verb+" ") || inputLower[afterConnIdx:] == verb {
						leftPart := strings.TrimSpace(input[lastCut:i])
						// Limpiar conectores residuales al final de la parte izquierda
						for {
							trimmed := false
							for _, suffix := range []string{" y", " luego", " después", ","} {
								if strings.HasSuffix(strings.ToLower(leftPart), suffix) {
									leftPart = leftPart[:len(leftPart)-len(suffix)]
									leftPart = strings.TrimSpace(leftPart)
									trimmed = true
								}
							}
							if !trimmed {
								break
							}
						}
						if leftPart != "" {
							parts = append(parts, leftPart)
						}
						lastCut = afterConnIdx
						i = afterConnIdx - 1
						break
					}

				}
			}
		}
	}

	if lastCut < len(input) {
		rightPart := strings.TrimSpace(input[lastCut:])
		if rightPart != "" {
			parts = append(parts, rightPart)
		}
	}

	if len(parts) == 0 {
		return []string{strings.TrimSpace(input)}
	}
	return parts
}
