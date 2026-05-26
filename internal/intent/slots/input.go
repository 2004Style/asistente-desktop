package slots

import (
	"regexp"
	"strings"
)

// ExtractInputSlots extrae slots relacionados con simulación de teclado y mouse.
// Retorna (key, keys, text, button)
func ExtractInputSlots(input string) (string, string, string, string) {
	inputLower := strings.ToLower(input)
	var keyVal, keysVal, textVal, buttonVal string

	// 1. Extraer botón del mouse: "clic izquierdo", "click derecho", "clic central"
	if strings.Contains(inputLower, "izquierdo") || strings.Contains(inputLower, "izq") {
		buttonVal = "left"
	} else if strings.Contains(inputLower, "derecho") || strings.Contains(inputLower, "der") {
		buttonVal = "right"
	} else if strings.Contains(inputLower, "central") || strings.Contains(inputLower, "medio") {
		buttonVal = "middle"
	} else if strings.Contains(inputLower, "clic") || strings.Contains(inputLower, "click") {
		buttonVal = "left" // Default
	}

	// 2. Extraer teclas o combinaciones de teclas (ej. "ctrl+c", "alt+tab", "enter")
	// Buscamos patrones comunes de teclas combinadas con '+'
	comboRegex := regexp.MustCompile(`([a-z0-9]+(?:\+[a-z0-9]+)+)`)
	if match := comboRegex.FindString(inputLower); match != "" {
		keysVal = match
	}

	// Teclas individuales comunes
	commonKeys := []string{
		"enter", "intro", "escape", "esc", "space", "espacio", "tab", "tabulador",
		"backspace", "retroceso", "delete", "suprimir", "up", "arriba", "down", "abajo",
		"left", "izquierda", "right", "derecha", "f1", "f2", "f3", "f4", "f5", "f6",
		"f7", "f8", "f9", "f10", "f11", "f12",
	}

	if keysVal == "" {
		words := strings.Fields(inputLower)
		for _, w := range words {
			wClean := strings.Trim(w, ".,!?;:")
			for _, k := range commonKeys {
				if wClean == k {
					keyVal = k
					keysVal = k
					break
				}
			}
			if keyVal != "" {
				break
			}
		}
	}

	// 3. Extraer texto para escribir: "escribe hola mundo"
	writeTriggers := []string{"escribe el texto", "escribe texto", "escribe", "escribir"}
	for _, trigger := range writeTriggers {
		if idx := strings.Index(inputLower, trigger); idx != -1 {
			txt := strings.TrimSpace(input[idx+len(trigger):])
			// Si empieza con comillas o conectores, limpiarlos
			if strings.HasPrefix(txt, "\"") && strings.HasSuffix(txt, "\"") {
				txt = strings.Trim(txt, "\"")
			} else if strings.HasPrefix(txt, "'") && strings.HasSuffix(txt, "'") {
				txt = strings.Trim(txt, "'")
			} else if strings.HasPrefix(txt, "que dice ") {
				txt = txt[len("que dice "):]
			}
			if txt != "" {
				textVal = txt
				break
			}
		}
	}

	return keyVal, keysVal, textVal, buttonVal
}
