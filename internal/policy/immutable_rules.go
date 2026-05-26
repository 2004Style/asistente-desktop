package policy

import (
	"strings"
)

// ImmutableBlockedCommands contiene comandos o patrones que JAMÁS se deben permitir.
var ImmutableBlockedCommands = []string{
	"rm -rf /",
	"rm -rf *",
	"dd if=",
	"mkfs.",
	"chmod -r 777",
	"chmod 777",
}

// IsImmutablyBlocked comprueba si una acción o comando viola reglas inmutables del sistema.
func IsImmutablyBlocked(toolName string, args map[string]interface{}) (bool, string) {
	// Verificar si se intenta ejecutar un comando de shell crítico/destructivo
	if toolName == "system.run_command" || toolName == "system.run_command_safe" {
		cmd, _ := args["command"].(string)
		if cmd == "" {
			cmd, _ = args["CommandLine"].(string)
		}
		if cmd != "" {
			cmdLower := strings.ToLower(cmd)
			for _, blocked := range ImmutableBlockedCommands {
				if strings.Contains(cmdLower, blocked) {
					return true, "comando de riesgo extremo bloqueado por política inmutable del sistema"
				}
			}
		}
	}
	return false, ""
}
