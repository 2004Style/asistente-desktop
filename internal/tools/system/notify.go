package system

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"rbot/internal/executor"
)

// NotifyTool envía notificaciones al escritorio Linux mediante notify-send.
type NotifyTool struct{}

func NewNotifyTool() *NotifyTool {
	return &NotifyTool{}
}

func (t *NotifyTool) Name() string { return "system.notify" }
func (t *NotifyTool) Description() string {
	return "Muestra una notificación emergente en el escritorio del sistema (utiliza notify-send)."
}
func (t *NotifyTool) Category() string  { return "system" }
func (t *NotifyTool) RiskLevel() string { return "low" }
func (t *NotifyTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Título principal de la notificación.",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Cuerpo o mensaje de texto de la notificación.",
			},
		},
		"required": []string{"title", "message"},
	}
}

func (t *NotifyTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	title, _ := args["title"].(string)
	message, _ := args["message"].(string)
	if title == "" || message == "" {
		return nil, fmt.Errorf("los argumentos 'title' y 'message' son obligatorios")
	}

	if _, err := exec.LookPath("notify-send"); err != nil {
		return nil, fmt.Errorf("el comando 'notify-send' no está instalado en el sistema")
	}

	cmd := exec.CommandContext(ctx, "notify-send", title, message)
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error al enviar la notificación: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       "Notificación de escritorio enviada.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
