package executor

import (
	"context"
	"fmt"
	"time"

	"rbot/internal/planner"
)

type Executor struct {
	registry *Registry
}

func NewExecutor(registry *Registry) *Executor {
	return &Executor{registry: registry}
}

func (e *Executor) ExecutePlan(ctx context.Context, plan *planner.Plan) ([]ToolResult, error) {
	var results []ToolResult

	for _, step := range plan.Steps {
		stepCtx := ctx
		var cancel context.CancelFunc

		if step.Timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		} else {
			// Timeout por defecto para evitar bloqueos
			stepCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		}

		res, err := e.registry.Execute(stepCtx, step.ToolName, step.Args)
		cancel()

		if err != nil {
			results = append(results, ToolResult{Error: err})
			return results, fmt.Errorf("fallo en paso %s: %w", step.ToolName, err)
		}

		results = append(results, res)
	}

	return results, nil
}
