package system_test

import (
	"rbot/internal/system"
	"testing"
)

func TestRunCommandToolRisk(t *testing.T) {
	tool := &system.RunCommandTool{}

	riskLow := tool.Risk(map[string]interface{}{"command": "ls -la"})
	if riskLow != "medium" { // Default is medium
		t.Errorf("Expected medium risk for ls -la, got %s", riskLow)
	}

	riskHigh := tool.Risk(map[string]interface{}{"command": "rm -rf /"})
	if riskHigh != "high" {
		t.Errorf("Expected high risk for rm -rf, got %s", riskHigh)
	}
}
