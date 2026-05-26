package planner

import (
	"github.com/google/uuid"
	"rbot/internal/intent"
)

// Plan representa una serie de pasos ordenados a ser ejecutados por el motor de RBot.
type Plan struct {
	ID           string                 `json:"id"`
	UserInput    string                 `json:"user_input"`
	Intent       string                 `json:"intent"`
	Confidence   float64                `json:"confidence"`
	RiskLevel    string                 `json:"risk_level"`
	NeedsConfirm bool                   `json:"needs_confirm"`
	Steps        []PlanStep             `json:"steps"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PlanStep representa un paso único de ejecución llamando a una herramienta específica.
type PlanStep struct {
	ID        string                 `json:"id"`
	ToolName  string                 `json:"tool_name"`
	Args      map[string]interface{} `json:"args"`
	TimeoutMs int                    `json:"timeout_ms"`
	Reason    string                 `json:"reason,omitempty"`
	DependsOn []string               `json:"depends_on,omitempty"`
}

// FromIntent crea un Plan base a partir de un IntentCandidate con un paso inicial
func FromIntent(userInput string, candidate intent.IntentCandidate) Plan {
	return Plan{
		ID:           uuid.New().String(),
		UserInput:    userInput,
		Intent:       candidate.Intent,
		Confidence:   candidate.Confidence,
		RiskLevel:    candidate.RiskLevel,
		NeedsConfirm: false,
		Steps:        []PlanStep{},
		Metadata:     make(map[string]interface{}),
	}
}
