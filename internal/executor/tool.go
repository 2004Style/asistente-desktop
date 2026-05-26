package executor

import (
	"context"
	"time"
)

// ToolResult contiene la salida estructurada e información temporal de la ejecución de una herramienta.
type ToolResult struct {
	Success    bool                   `json:"success"`
	Text       string                 `json:"text,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
}

// ToolHandler define la interfaz universal para todas las herramientas del sistema (internas y adaptadores MCP).
type ToolHandler interface {
	Name() string
	Description() string
	Category() string
	RiskLevel() string
	Schema() map[string]interface{}
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}
