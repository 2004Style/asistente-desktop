package system

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"rbot/internal/executor"
)

// RunCommandTool ejecuta un comando shell de bash (herramienta heredada).
type RunCommandTool struct{}

func NewRunCommandTool() *RunCommandTool {
	return &RunCommandTool{}
}

func (t *RunCommandTool) Name() string { return "system.run_command" }
func (t *RunCommandTool) Description() string {
	return "Ejecuta un comando en la terminal de Linux y retorna el resultado (salida estándar)."
}
func (t *RunCommandTool) Category() string  { return "system" }
func (t *RunCommandTool) RiskLevel() string { return "high" }
func (t *RunCommandTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Comando bash a ejecutar.",
			},
		},
		"required": []string{"command"},
	}
}

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	command, _ := args["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("el argumento 'command' es obligatorio")
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
