package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"rbot/internal/executor"
	"rbot/internal/ollama"
)

type RunCommandTool struct{}

func (t *RunCommandTool) Name() string {
	return "system.run_command"
}

func (t *RunCommandTool) Definition() ollama.Tool {
	return ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "system.run_command",
			Description: "Ejecuta un comando en la terminal de Linux y retorna el resultado (salida estándar).",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Comando bash a ejecutar.",
					},
				},
				Required: []string{"command"},
			},
		},
	}
}

func (t *RunCommandTool) Risk(args map[string]interface{}) string {
	cmd, _ := args["command"].(string)
	if strings.Contains(cmd, "rm -rf") || strings.Contains(cmd, "mkfs") || strings.Contains(cmd, "dd") || strings.Contains(cmd, "chmod -R 777") || strings.Contains(cmd, "curl") || strings.Contains(cmd, "sudo") {
		return "high"
	}
	return "medium"
}

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]interface{}) (executor.ToolResult, error) {
	cmdStr, ok := args["command"].(string)
	if !ok {
		return executor.ToolResult{}, fmt.Errorf("comando no proporcionado")
	}

	cmd := exec.CommandContext(ctx, "bash", "-lc", cmdStr)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return executor.ToolResult{Output: string(output)}, fmt.Errorf("comando cancelado por timeout")
	}

	if err != nil {
		return executor.ToolResult{Output: string(output)}, fmt.Errorf("comando falló: %w", err)
	}

	return executor.ToolResult{Output: string(output), Error: nil}, nil
}

// RegisterTools registra las herramientas del sistema en el registry
func RegisterTools(registry *executor.Registry) {
	registry.Register(&RunCommandTool{})
}
