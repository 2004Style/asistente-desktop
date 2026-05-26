package runtime

import "time"

// ToolResult contiene el resultado estructurado de la ejecución de una herramienta
type ToolResult struct {
	Success    bool                   `json:"success"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Text       string                 `json:"text,omitempty"`
	Error      string                 `json:"error,omitempty"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
}

// AgentResponse es la respuesta integral y estructurada generada por el orquestador
type AgentResponse struct {
	Text        string       `json:"text"`
	Events      []Event      `json:"events,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}
