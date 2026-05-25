package planner_test

import (
	"rbot/internal/intent"
	"rbot/internal/planner"
	"testing"
)

func TestFromIntent(t *testing.T) {
	candidate := intent.IntentCandidate{
		Intent:     "test-intent",
		Confidence: 0.95,
		RiskLevel:  "high",
	}

	plan := planner.FromIntent("do it", candidate)
	if plan.ID == "" {
		t.Errorf("Expected Plan to have a generated ID")
	}
	if plan.Intent != "test-intent" {
		t.Errorf("Expected Intent to be test-intent")
	}
	if plan.Confidence != 0.95 {
		t.Errorf("Expected Confidence 0.95")
	}
	if plan.RiskLevel != "high" {
		t.Errorf("Expected RiskLevel high")
	}
}
