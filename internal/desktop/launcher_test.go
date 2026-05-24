package desktop

import (
	"testing"
)

func TestLaunchApplication(t *testing.T) {
	// 1. Empty command validation
	err := LaunchApplication("")
	if err == nil {
		t.Errorf("Expected error for empty command, got nil")
	}

	// 2. Launch valid safe command (no-op)
	err = LaunchApplication("true")
	if err != nil {
		t.Errorf("LaunchApplication('true') failed: %v", err)
	}
}

func TestOpenURL_Validation(t *testing.T) {
	// 1. Empty URL validation
	err := OpenURL("")
	if err == nil {
		t.Errorf("Expected error for empty URL, got nil")
	}
}
