package browser

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

// SearchTool realiza búsquedas de texto en Internet mediante Google.
type SearchTool struct{}

// NewSearchTool inicializa la herramienta de búsqueda.
func NewSearchTool() *SearchTool {
	return &SearchTool{}
}

func (t *SearchTool) Name() string { return "browser.search" }
func (t *SearchTool) Description() string {
	return "Realiza una búsqueda de texto en Internet utilizando Google en el navegador por defecto."
}
func (t *SearchTool) Category() string  { return "browser" }
func (t *SearchTool) RiskLevel() string { return "low" }
func (t *SearchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Texto o términos de búsqueda a consultar.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("el argumento 'query' es obligatorio")
	}

	targetURL := fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(query))
	err := desktop.OpenURL(targetURL)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Buscando '%s' en Internet.", query),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
