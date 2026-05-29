package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"rbot/internal/executor"
)

// RunCommandSafeTool ejecuta un comando shell de bash de forma controlada y con confirmaciones.
type RunCommandSafeTool struct{}

func NewRunCommandSafeTool() *RunCommandSafeTool {
	return &RunCommandSafeTool{}
}

func (t *RunCommandSafeTool) Name() string { return "system.run_command_safe" }
func (t *RunCommandSafeTool) Description() string {
	return "Ejecuta un comando de consola en Bash de manera segura y devuelve la salida combinada de stdout y stderr."
}
func (t *RunCommandSafeTool) Category() string  { return "system" }
func (t *RunCommandSafeTool) RiskLevel() string { return "high" }
func (t *RunCommandSafeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Comando de shell completo a ejecutar (ej: go test ./..., git status).",
			},
		},
		"required": []string{"command"},
	}
}

func (t *RunCommandSafeTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	command, _ := args["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("el argumento 'command' es obligatorio")
	}

	// Validar comando xdg-open para evitar asociaciones MIME erróneas que abran AnyDesk u otras apps locales
	if strings.Contains(command, "xdg-open") {
		if !strings.Contains(command, "http://") && !strings.Contains(command, "https://") {
			return &executor.ToolResult{
				Success:    false,
				Error:      "Bloqueado por política de seguridad: xdg-open solo puede abrir enlaces web válidos (http:// o https://) para evitar la apertura accidental de aplicaciones locales como AnyDesk.",
				StartedAt:  started,
				FinishedAt: time.Now(),
			}, nil
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	outStr := string(output)

	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      fmt.Sprintf("Comando falló con error: %v\nSalida:\n%s", err, outStr),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	if outStr == "" {
		outStr = "(comando ejecutado sin salida en consola)"
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       outStr,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
