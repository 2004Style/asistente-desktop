package executor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"rbot/internal/planner"
)

// PlanResult representa el resultado acumulado de la ejecución de todos los pasos de un plan.
type PlanResult struct {
	Success bool          `json:"success"`
	Results []*ToolResult `json:"results"`
	Error   string        `json:"error,omitempty"`
}

// PolicyDecision describe el veredicto del motor de políticas de seguridad.
type PolicyDecision struct {
	Allowed         bool   `json:"allowed"`
	RequiresConfirm bool   `json:"requires_confirm"`
	RiskLevel       string `json:"risk_level"`
	Reason          string `json:"reason"`
}

// Policy es la interfaz que debe implementar el motor de políticas de seguridad.
type Policy interface {
	EvaluateTool(ctx context.Context, tool ToolHandler, args map[string]interface{}) PolicyDecision
}

// EventPublisher permite al ejecutor notificar al EventBus sobre el estado de la ejecución.
type EventPublisher interface {
	Publish(eventType string, payload map[string]interface{})
}

// Executor coordina la ejecución segura de planes paso a paso.
type Executor struct {
	Registry *Registry
	Policy   Policy
	Events   EventPublisher
	DB       *sql.DB
}

// NewExecutor inicializa un ejecutor con sus dependencias básicas.
func NewExecutor(reg *Registry, pol Policy, evs EventPublisher, database *sql.DB) *Executor {
	return &Executor{
		Registry: reg,
		Policy:   pol,
		Events:   evs,
		DB:       database,
	}
}

// ExecutePlan procesa cada paso del plan provisto, evaluando la seguridad y ejecutando con timeouts.
func (e *Executor) ExecutePlan(ctx context.Context, plan planner.Plan) (*PlanResult, error) {
	results := make([]*ToolResult, 0, len(plan.Steps))

	for _, step := range plan.Steps {
		tool, ok := e.Registry.Get(step.ToolName)
		if !ok {
			return nil, fmt.Errorf("herramienta no registrada en el sistema: %s", step.ToolName)
		}

		decision := e.Policy.EvaluateTool(ctx, tool, step.Args)
		if !decision.Allowed {
			e.logAction(plan.ID, plan.UserInput, step.ToolName, step.Args, decision.RiskLevel, "denied", decision.Reason, time.Now(), time.Now())
			return &PlanResult{
				Success: false,
				Results: results,
				Error:   decision.Reason,
			}, nil
		}

		timeout := time.Duration(step.TimeoutMs) * time.Millisecond
		if timeout <= 0 {
			timeout = 20 * time.Second
		}

		stepCtx, cancel := context.WithTimeout(ctx, timeout)

		if e.Events != nil {
			e.Events.Publish("tool.started", map[string]interface{}{
				"tool": step.ToolName,
			})
		}

		startedAt := time.Now()
		result, err := tool.Execute(stepCtx, step.Args)
		finishedAt := time.Now()
		cancel()

		if err != nil {
			result = &ToolResult{
				Success:    false,
				Error:      err.Error(),
				StartedAt:  startedAt,
				FinishedAt: finishedAt,
			}
		}

		if result.StartedAt.IsZero() {
			result.StartedAt = startedAt
		}
		if result.FinishedAt.IsZero() {
			result.FinishedAt = finishedAt
		}

		results = append(results, result)

		status := "success"
		var errStr string
		if !result.Success {
			status = "failed"
			errStr = result.Error
		}

		// Registrar la auditoría en la base de datos
		e.logAction(plan.ID, plan.UserInput, step.ToolName, step.Args, decision.RiskLevel, status, errStr, startedAt, finishedAt)

		if e.Events != nil {
			if result.Success {
				e.Events.Publish("tool.finished", map[string]interface{}{
					"tool":    step.ToolName,
					"success": true,
				})
			} else {
				e.Events.Publish("tool.failed", map[string]interface{}{
					"tool":    step.ToolName,
					"success": false,
					"error":   result.Error,
				})
			}
		}

		if !result.Success {
			return &PlanResult{
				Success: false,
				Results: results,
				Error:   result.Error,
			}, nil
		}
	}

	return &PlanResult{
		Success: true,
		Results: results,
	}, nil
}

func (e *Executor) logAction(planID, userInput, toolName string, args map[string]interface{}, risk, status, errorStr string, start, end time.Time) {
	if e.DB == nil {
		return
	}
	argsJSON, _ := json.Marshal(args)

	// Formato de fechas
	startStr := start.Format("2006-01-02 15:04:05")
	endStr := end.Format("2006-01-02 15:04:05")

	query := `INSERT INTO action_log (plan_id, user_input, tool_name, arguments_json, risk_level, status, error, started_at, finished_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := e.DB.Exec(query, planID, userInput, toolName, string(argsJSON), risk, status, errorStr, startStr, endStr)
	if err != nil {
		// Fallback para mantener compatibilidad antes de ejecutar la migración física de base de datos
		legacyQuery := `INSERT INTO action_log (user_input, tool_name, tool_source, arguments_json, result_json, success, error, required_confirmation, duration_ms) 
		                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		successVal := 0
		if status == "success" {
			successVal = 1
		}
		durationMs := end.Sub(start).Milliseconds()
		_, _ = e.DB.Exec(legacyQuery, userInput, toolName, "internal", string(argsJSON), errorStr, successVal, errorStr, 0, durationMs)
	}
}
