package planner

import (
	"fmt"
	"github.com/google/uuid"
	"rbot/internal/intent"
)

// BuildPlan analiza las partes divididas e intenciones para generar un Plan secuencial ordenado.
func BuildPlan(userInput string, parts []string, router *intent.Router) Plan {
	plan := Plan{
		ID:           uuid.New().String(),
		UserInput:    userInput,
		NeedsConfirm: false,
		Steps:        []PlanStep{},
		Metadata:     make(map[string]interface{}),
	}

	maxRisk := "low"

	for i, part := range parts {
		candidates := router.Match(part)
		if len(candidates) == 0 {
			continue
		}
		top := candidates[0]

		// Actualizar el riesgo global al máximo encontrado
		if riskPriority(top.RiskLevel) > riskPriority(maxRisk) {
			maxRisk = top.RiskLevel
		}

		if i == 0 {
			plan.Intent = top.Intent
			plan.Confidence = top.Confidence
		}

		stepID := fmt.Sprintf("step_%d", i+1)

		// Convertir slots map[string]interface{} a argumentos del paso
		args := make(map[string]interface{})
		for k, v := range top.Slots {
			args[k] = v
		}

		step := PlanStep{
			ID:        stepID,
			ToolName:  top.ToolName,
			Args:      args,
			TimeoutMs: 20000, // Timeout por defecto de 20s
			Reason:    top.Reason,
			DependsOn: []string{},
		}

		// Encadenamiento secuencial: cada paso depende del anterior
		if i > 0 {
			step.DependsOn = append(step.DependsOn, fmt.Sprintf("step_%d", i))
		}

		plan.Steps = append(plan.Steps, step)
	}

	plan.RiskLevel = maxRisk

	// Heurística de confirmación: planes con riesgo high requieren confirmación.
	// Nota: Los de riesgo "critical" se bloquean directamente en el daemon.
	if maxRisk == "high" {
		plan.NeedsConfirm = true
	}

	return plan
}

func riskPriority(risk string) int {
	switch risk {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}
