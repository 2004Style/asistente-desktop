package input

import "strings"

// letterNames mapea nombres fonéticos en español a letras.
var letterNames = map[string]string{
	"ele":   "l",
	"ce":    "c",
	"uve":   "v",
	"ese":   "s",
	"erre":  "r",
	"te":    "t",
	"eme":   "m",
	"ene":   "n",
	"efe":   "f",
	"ge":    "g",
	"zeta":  "z",
	"ache":  "h",
	"de":    "d",
	"pe":    "p",
	"ka":    "k",
	"a":     "a",
	"be":    "b",
	"i":     "i",
	"o":     "o",
	"u":     "u",
	"ye":    "y",
	"equis": "x",
	"jota":  "j",
	"cu":    "q",
	"uve doble": "w",
}

// modifierNames normaliza nombres de modificadores.
var modifierNames = map[string]string{
	"control": "ctrl",
	"ctrl":    "ctrl",
	"alt":     "alt",
	"super":   "super",
	"windows": "super",
	"win":     "super",
	"shift":   "shift",
	"mayus":   "shift",
	"mayúscula": "shift",
}

// voicePhraseMap mapea frases de voz directamente a combinaciones de teclas.
var voicePhraseMap = map[string][]string{
	"selecciona todo":       {"ctrl", "a"},
	"seleccionar todo":      {"ctrl", "a"},
	"copia":                 {"ctrl", "c"},
	"copiar":                {"ctrl", "c"},
	"pega":                  {"ctrl", "v"},
	"pegar":                 {"ctrl", "v"},
	"guarda":                {"ctrl", "s"},
	"guardar":               {"ctrl", "s"},
	"deshace":               {"ctrl", "z"},
	"deshacer":              {"ctrl", "z"},
	"rehace":                {"ctrl", "y"},
	"rehacer":               {"ctrl", "y"},
	"nueva pestaña":         {"ctrl", "t"},
	"nueva ventana":         {"ctrl", "n"},
	"cerrar pestaña":        {"ctrl", "w"},
	"cerrar ventana":        {"alt", "F4"},
	"buscar":                {"ctrl", "f"},
	"imprimir":              {"ctrl", "p"},
	"abrir":                 {"ctrl", "o"},
	"deshacer selección":    {"Escape"},
}

// singleKeyMap mapea nombres de teclas especiales.
var singleKeyMap = map[string]string{
	"escape":      "Escape",
	"esc":         "Escape",
	"enter":       "Return",
	"intro":       "Return",
	"retroceso":   "BackSpace",
	"backspace":   "BackSpace",
	"tabulador":   "Tab",
	"tab":         "Tab",
	"arriba":      "Up",
	"abajo":       "Down",
	"izquierda":   "Left",
	"derecha":     "Right",
	"inicio":      "Home",
	"fin":         "End",
	"avanzar":     "Page_Down",
	"retroceder":  "Page_Up",
	"suprimir":    "Delete",
	"delete":      "Delete",
	"insertar":    "Insert",
	"espacio":     "space",
	"f1":          "F1",
	"f2":          "F2",
	"f3":          "F3",
	"f4":          "F4",
	"f5":          "F5",
	"f6":          "F6",
	"f7":          "F7",
	"f8":          "F8",
	"f9":          "F9",
	"f10":         "F10",
	"f11":         "F11",
	"f12":         "F12",
}

// NormalizeKeys convierte una frase de voz a un slice de teclas normalizadas.
// Ejemplos:
//   "control ele"   -> ["ctrl", "l"]
//   "alt tab"       -> ["alt", "Tab"]
//   "selecciona todo" -> ["ctrl", "a"]
func NormalizeKeys(input string) []string {
	normalized := strings.ToLower(strings.TrimSpace(input))

	// 1. Revisar frases directas
	if keys, ok := voicePhraseMap[normalized]; ok {
		return keys
	}

	// 2. Revisar si es una tecla especial sola
	if key, ok := singleKeyMap[normalized]; ok {
		return []string{key}
	}

	// 3. Parsear formato "mod+key" o "mod-key"
	if strings.ContainsAny(normalized, "+-") {
		sep := "+"
		if strings.Contains(normalized, "-") && !strings.Contains(normalized, "+") {
			sep = "-"
		}
		parts := strings.Split(normalized, sep)
		return resolveKeyParts(parts)
	}

	// 4. Parsear palabras separadas por espacios
	parts := strings.Fields(normalized)
	if len(parts) > 1 {
		return resolveKeyParts(parts)
	}

	// 5. Tecla sola
	return []string{NormalizeSingleKey(normalized)}
}

// resolveKeyParts convierte un slice de partes a teclas normalizadas.
func resolveKeyParts(parts []string) []string {
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Modificador
		if mod, ok := modifierNames[part]; ok {
			result = append(result, mod)
			continue
		}
		// Nombre de letra en español
		if letter, ok := letterNames[part]; ok {
			result = append(result, letter)
			continue
		}
		// Tecla especial
		if key, ok := singleKeyMap[part]; ok {
			result = append(result, key)
			continue
		}
		// Usar tal cual
		result = append(result, part)
	}
	return result
}

// NormalizeSingleKey normaliza una tecla individual.
func NormalizeSingleKey(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))

	if key, ok := singleKeyMap[normalized]; ok {
		return key
	}
	if letter, ok := letterNames[normalized]; ok {
		return letter
	}
	// Si es una letra o dígito, devolverlo directamente
	return normalized
}
