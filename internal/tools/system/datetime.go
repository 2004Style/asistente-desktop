package system

import (
	"context"
	"fmt"
	"time"

	"rbot/internal/executor"
)

type DateTimeTool struct{}

func NewDateTimeTool() *DateTimeTool {
	return &DateTimeTool{}
}

func (t *DateTimeTool) Name() string { return "system.datetime" }
func (t *DateTimeTool) Description() string {
	return "Obtiene la fecha y hora actual en tiempo real."
}
func (t *DateTimeTool) Category() string  { return "system" }
func (t *DateTimeTool) RiskLevel() string { return "low" }
func (t *DateTimeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *DateTimeTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Fecha y hora actual: %s", nowStr),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
