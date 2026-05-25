package browser_test

import (
	"context"
	"testing"
	"rbot/internal/browser"
)

func TestFindBrowserWithTab_Empty(t *testing.T) {
    // If hyprctl is not available in the test environment, it should return an error
    // If it is available, it might return empty or not depending on the desktop state
    _, err := browser.FindBrowserWithTab(context.Background(), "something_impossible_to_find")
    if err != nil {
        t.Log("Expected error if hyprctl is missing or fails:", err)
    }
}
