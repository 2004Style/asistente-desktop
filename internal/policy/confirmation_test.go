package policy_test

import (
	"rbot/internal/planner"
	"rbot/internal/policy"
	"testing"
	"time"
)

func TestConfirmationEngine(t *testing.T) {
	engine := policy.NewEngine()

	plan := &planner.Plan{ID: "plan-123"}
	engine.AddPending(plan, "dangerous action", 50*time.Millisecond)

	pending := engine.GetPending()
	if pending == nil {
		t.Fatalf("Expected pending plan, got nil")
	}
	if pending.PlanID != "plan-123" {
		t.Errorf("Expected plan-123, got %s", pending.PlanID)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)
	expired := engine.GetPending()
	if expired != nil {
		t.Fatalf("Expected nil after expiration, got plan")
	}
}
