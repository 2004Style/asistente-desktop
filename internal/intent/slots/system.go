package slots

import (
	"regexp"
	"strings"
)

// ExtractSystemSlots extrae slots relacionados con el sistema y la memoria de usuario.
// Retorna (command, key, value, workspace)
func ExtractSystemSlots(input string) (string, string, string, string) {
	inputLower := strings.ToLower(input)
	var commandVal, keyVal, valueVal, workspaceVal string

	// 1. Extraer comando: "ejecuta go test", "ejecutar ls -la", "corre ./script.sh"
	cmdTriggers := []string{"ejecuta el comando", "ejecuta", "ejecutar", "corre el comando", "corre", "correr"}
	for _, trigger := range cmdTriggers {
		if idx := strings.Index(inputLower, trigger); idx != -1 {
			cmdPart := strings.TrimSpace(input[idx+len(trigger):])
			if cmdPart != "" {
				commandVal = cmdPart
				break
			}
		}
	}

	// 2. Extraer memoria de usuario (clave/valor): "recuerda que mi correo es test@test.com"
	// "recuerda que [clave] es [valor]"
	memoryRegex := regexp.MustCompile(`recuerda\s+que\s+(?:mi\s+)?([^\s]+)\s+es\s+(.+)`)
	if matches := memoryRegex.FindStringSubmatch(inputLower); len(matches) > 2 {
		keyVal = strings.TrimSpace(matches[1])
		valueVal = strings.Trim(matches[2], " .,?!¡¿")
		// Restaurar mayúsculas del valor original
		valIdx := strings.Index(inputLower, valueVal)
		if valIdx != -1 {
			valueVal = input[valIdx : valIdx+len(valueVal)]
		}
	}

	// 3. Extraer workspace: "cambia al área de trabajo 2", "pásate al workspace dev"
	workspaceRegex := regexp.MustCompile(`(?:workspace|área de trabajo|escritorio)\s+([^\s,]+)`)
	if matches := workspaceRegex.FindStringSubmatch(inputLower); len(matches) > 1 {
		workspaceVal = matches[1]
	}

	return commandVal, keyVal, valueVal, workspaceVal
}
