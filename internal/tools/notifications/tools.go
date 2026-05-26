package notifications

import (
	"context"
	"fmt"
	"time"

	"rbot/internal/executor"
)

type SendNotificationTool struct {
	manager *NotificationManager
}

func NewSendNotificationTool(mgr *NotificationManager) *SendNotificationTool {
	return &SendNotificationTool{manager: mgr}
}

func (t *SendNotificationTool) Name() string { return "notification.send" }
func (t *SendNotificationTool) Description() string {
	return "Envía una notificación al usuario a través de canales especificados (desktop, voice, hud, sound, all o default)."
}
func (t *SendNotificationTool) Category() string  { return "productivity" }
func (t *SendNotificationTool) RiskLevel() string { return "low" }
func (t *SendNotificationTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Canal de notificación: 'desktop', 'voice', 'hud', 'sound', 'all', o 'default'.",
				"enum":        []interface{}{"desktop", "voice", "hud", "sound", "all", "default"},
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Título de la notificación.",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Cuerpo del mensaje de la notificación.",
			},
			"urgent": map[string]interface{}{
				"type":        "boolean",
				"description": "Si es verdadero, ignora Quiet Hours.",
			},
		},
		"required": []string{"message"},
	}
}

func (t *SendNotificationTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	channel, _ := args["channel"].(string)
	if channel == "" {
		channel = "default"
	}
	title, _ := args["title"].(string)
	message, _ := args["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("el argumento 'message' es obligatorio")
	}
	urgent, _ := args["urgent"].(bool)

	err := t.manager.Send(ctx, channel, title, message, urgent)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Notificación enviada al canal %q.", channel),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
