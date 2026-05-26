package intent

import (
	"strings"
)

// Normalize limpia y estandariza la entrada del usuario eliminando wake words y corrigiendo errores ortográficos.
func Normalize(input string, wakeWords []string) string {
	inputLower := strings.ToLower(strings.TrimSpace(input))

	// 1. Remover wake words si existen al inicio
	for _, ww := range wakeWords {
		wwLower := strings.ToLower(ww)
		if wwLower != "" && (strings.HasPrefix(inputLower, wwLower+" ") || inputLower == wwLower) {
			inputLower = strings.TrimPrefix(inputLower, wwLower)
			inputLower = strings.TrimSpace(inputLower)
			break
		}
	}

	// 2. Corregir puntuación residual
	inputLower = strings.TrimFunc(inputLower, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == '¿' || r == '¡' || r == ';' || r == ':' || r == '-' || r == ')' || r == '(' || r == '\'' || r == '"'
	})
	inputLower = strings.TrimSpace(inputLower)

	// Manejar el caso de "Ronaldo" si la wake word era "ronald"
	if inputLower == "o" {
		inputLower = ""
	} else if strings.HasPrefix(inputLower, "o ") {
		inputLower = strings.TrimPrefix(inputLower, "o ")
	} else if strings.HasPrefix(inputLower, "o,") {
		inputLower = strings.TrimPrefix(inputLower, "o,")
	}

	// 3. Normalizaciones fonéticas/ Whisper / Sinónimos
	inputLower = " " + inputLower + " "

	replacements := map[string]string{
		"yutub":         "youtube",
		"yutú":          "youtube",
		"bisual":        "vscode",
		"gugol":         "google",
		"gugel":         "google",
		"convertsistem": "convertsystems",
		"doker":         "docker",
		"karpeta":       "carpeta",
		"prueva":        "prueba",
		"arxivo":        "archivo",
		"sirra":         "cierra",
		"sierra":        "cierra",
		"habre":         "abre",
		"habré":         "abre",
		"bray":          "brave",
		"colcame":       "colócame",
		"preciona":      "presiona",
		"ele":           "l",
	}

	for k, v := range replacements {
		inputLower = strings.ReplaceAll(inputLower, " "+k+" ", " "+v+" ")
		inputLower = strings.ReplaceAll(inputLower, " "+k+",", " "+v+",")
		inputLower = strings.ReplaceAll(inputLower, " "+k+".", " "+v+".")
		inputLower = strings.ReplaceAll(inputLower, " "+k+"?", " "+v+"?")
		inputLower = strings.ReplaceAll(inputLower, " "+k+"!", " "+v+"!")
	}

	return strings.TrimSpace(inputLower)
}

// CleanCommand limpia puntuación y remueve letras remanentes (como 'o' en 'Ronaldo') para extraer el comando de voz limpio.
func CleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimFunc(cmd, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':' || r == '-' || r == ')' || r == '(' || r == '\'' || r == '"'
	})
	cmd = strings.TrimSpace(cmd)

	if cmd == "o" {
		cmd = ""
	} else if strings.HasPrefix(cmd, "o ") {
		cmd = strings.TrimPrefix(cmd, "o ")
	} else if strings.HasPrefix(cmd, "o,") {
		cmd = strings.TrimPrefix(cmd, "o,")
	}

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
	if strings.Contains(t, "subtítulos por") || strings.Contains(t, "subtitles by") || strings.Contains(t, "amara.org") {
		return true
	}
	return false
}
