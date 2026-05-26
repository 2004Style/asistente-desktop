package browser

import (
	"context"
	"fmt"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

// OpenURLTool abre un sitio web en el navegador por defecto del sistema.
type OpenURLTool struct{}

// NewOpenURLTool inicializa la herramienta.
func NewOpenURLTool() *OpenURLTool {
	return &OpenURLTool{}
}

func (t *OpenURLTool) Name() string { return "browser.open_url" }
func (t *OpenURLTool) Description() string {
	return "Abre un sitio web en el navegador por defecto del sistema (ej: youtube.com, whatsapp.com, google.com)."
}
func (t *OpenURLTool) Category() string  { return "browser" }
func (t *OpenURLTool) RiskLevel() string { return "low" }
func (t *OpenURLTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Dirección URL o dominio web exacto a abrir (ej: youtube.com, https://github.com).",
			},
		},
		"required": []string{"url"},
	}
}

func (t *OpenURLTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	targetURL, _ := args["url"].(string)
	if targetURL == "" {
		return nil, fmt.Errorf("el argumento 'url' es obligatorio")
	}

	err := desktop.OpenURL(targetURL)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("URL '%s' abierta correctamente.", targetURL),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
