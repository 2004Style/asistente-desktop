package executor_test

import (
	"context"
	"rbot/internal/executor"
	"rbot/internal/ollama"
	"rbot/internal/planner"
	"testing"
	"time"
)

type mockTool struct {
	delay time.Duration
}

func (t *mockTool) Name() string { return "mock.tool" }
func (t *mockTool) Definition() ollama.Tool { return ollama.Tool{} }
func (t *mockTool) Risk(args map[string]interface{}) string { return "low" }
func (t *mockTool) Execute(ctx context.Context, args map[string]interface{}) (executor.ToolResult, error) {
	select {
	case <-time.After(t.delay):
		return executor.ToolResult{Output: "success"}, nil
	case <-ctx.Done():
		return executor.ToolResult{}, ctx.Err()
	}
}

func TestExecutorTimeout(t *testing.T) {
	reg := executor.NewRegistry()
	reg.Register(&mockTool{delay: 2 * time.Second})

	exec := executor.NewExecutor(reg)

	plan := &planner.Plan{
		Steps: []*planner.PlanStep{
			{
				ToolName: "mock.tool",
				Timeout:  100 * time.Millisecond,
			},
		},
	}

	results, err := exec.ExecutePlan(context.Background(), plan)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if len(results) != 1 || results[0].Error != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded in results, got %v", results)
	}
}
