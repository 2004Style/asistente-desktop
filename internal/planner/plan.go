package planner

import (
	"time"

	"github.com/google/uuid"
	"rbot/internal/intent"
)

type Plan struct {
	ID           string
	UserInput    string
	Intent       string
	Confidence   float64
	Steps        []*PlanStep
	RiskLevel    string
	NeedsConfirm bool
}

type PlanStep struct {
	ToolName string
	Args     map[string]interface{}
	Reason   string
	Timeout  time.Duration
	Rollback *PlanStep
}

// FromIntent crea un Plan base a partir de un IntentCandidate con un solo paso
func FromIntent(userInput string, candidate intent.IntentCandidate) *Plan {
	plan := &Plan{
		ID:           uuid.New().String(),
		UserInput:    userInput,
		Intent:       candidate.Intent,
		Confidence:   candidate.Confidence,
		RiskLevel:    candidate.RiskLevel,
		NeedsConfirm: false, // Se evaluará más adelante por PolicyEngine
		Steps:        []*PlanStep{},
	}

	// Por defecto asume que si hay herramientas, crea un paso para la primera (muy básico)
	// En el futuro, el Planner puede resolver la herramienta basado en el intent
	return plan
}
