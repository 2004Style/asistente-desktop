package executor_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"rbot/internal/executor"
	"rbot/internal/planner"
)

type mockTool struct {
	delay time.Duration
}

func (t *mockTool) Name() string                     { return "mock.tool" }
func (t *mockTool) Description() string              { return "Mock Tool" }
func (t *mockTool) Category() string                 { return "mock" }
func (t *mockTool) RiskLevel() string                { return "low" }
func (t *mockTool) Schema() map[string]interface{}   { return nil }
func (t *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	select {
	case <-time.After(t.delay):
		return &executor.ToolResult{Success: true, Text: "success"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type mockPolicy struct{}

func (p *mockPolicy) EvaluateTool(ctx context.Context, tool executor.ToolHandler, args map[string]interface{}) executor.PolicyDecision {
	return executor.PolicyDecision{
		Allowed: true,
	}
}

func TestExecutorTimeout(t *testing.T) {
	reg := executor.NewRegistry()
	_ = reg.Register(&mockTool{delay: 1 * time.Second})

	pol := &mockPolicy{}
	exec := executor.NewExecutor(reg, pol, nil, nil)

	plan := planner.Plan{
		Steps: []planner.PlanStep{
			{
				ToolName:  "mock.tool",
				TimeoutMs: 50,
			},
		},
	}

	res, err := exec.ExecutePlan(context.Background(), plan)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if res.Success {
		t.Fatal("Expected plan to fail due to timeout, but it succeeded")
	}

	if len(res.Results) != 1 {
		t.Fatalf("Expected 1 tool result, got %d", len(res.Results))
	}

	if res.Results[0].Success {
		t.Fatal("Expected step to fail")
	}

	if !strings.Contains(res.Results[0].Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("Expected DeadlineExceeded error, got: %s", res.Results[0].Error)
	}
}
