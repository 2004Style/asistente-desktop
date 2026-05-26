package planner_test

import (
	"rbot/internal/planner"
	"testing"
)

func TestResolveDependenciesSuccess(t *testing.T) {
	steps := []planner.PlanStep{
		{
			ID:        "step_2",
			ToolName:  "files.delete_file",
			DependsOn: []string{"step_1"},
		},
		{
			ID:        "step_1",
			ToolName:  "browser-open",
			DependsOn: []string{},
		},
		{
			ID:        "step_3",
			ToolName:  "media",
			DependsOn: []string{"step_2"},
		},
	}

	ordered, err := planner.ResolveDependencies(steps)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(ordered) != 3 {
		t.Fatalf("Expected 3 ordered steps, got %d", len(ordered))
	}

	if ordered[0].ID != "step_1" || ordered[1].ID != "step_2" || ordered[2].ID != "step_3" {
		t.Errorf("Unexpected ordering: %s -> %s -> %s", ordered[0].ID, ordered[1].ID, ordered[2].ID)
	}
}

func TestResolveDependenciesCycle(t *testing.T) {
	steps := []planner.PlanStep{
		{
			ID:        "step_1",
			ToolName:  "files.read_file",
			DependsOn: []string{"step_2"},
		},
		{
			ID:        "step_2",
			ToolName:  "files.delete_file",
			DependsOn: []string{"step_1"},
		},
	}

	_, err := planner.ResolveDependencies(steps)
	if err == nil {
		t.Error("Expected error due to cyclic dependencies, got nil")
	}
}

func TestRecoveryAction(t *testing.T) {
	stepBrowser := planner.PlanStep{ToolName: "browser-open"}
	action, _ := planner.DetermineRecoveryAction(stepBrowser, nil)
	if action != planner.RecoveryContinue {
		t.Errorf("Expected continue action for browser-open, got %v", action)
	}

	stepDestructive := planner.PlanStep{ToolName: "files.delete_file"}
	action, _ = planner.DetermineRecoveryAction(stepDestructive, nil)
	if action != planner.RecoveryAbort {
		t.Errorf("Expected abort action for critical tools, got %v", action)
	}
}
