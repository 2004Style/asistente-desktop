package desktop

import (
	"context"
	"os"
	"os/exec"
)

// WindowInfo holds information about an open window.
type WindowInfo struct {
	ID        string `json:"id"`
	Address   string `json:"address,omitempty"` // Hyprland hex address (strongest identifier)
	Title     string `json:"title"`
	App       string `json:"app"`
	Class     string `json:"class"`
	PID       int    `json:"pid"`
	Workspace string `json:"workspace"`
	Focused   bool   `json:"focused"`
}

// WindowSelector identifies one or more windows to act upon.
type WindowSelector struct {
	ID        string `json:"id,omitempty"`
	Address   string `json:"address,omitempty"`
	App       string `json:"app,omitempty"`
	Title     string `json:"title,omitempty"`
	Class     string `json:"class,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Active    bool   `json:"active,omitempty"`
}

// WindowManager is the interface all window backends implement.
type WindowManager interface {
	Name() string
	ListWindows(ctx context.Context) ([]WindowInfo, error)
	ActiveWindow(ctx context.Context) (*WindowInfo, error)
	FocusWindow(ctx context.Context, selector WindowSelector) error
	CloseWindow(ctx context.Context, selector WindowSelector) error
	MoveToWorkspace(ctx context.Context, selector WindowSelector, workspace string) error
}

// NewWindowManager returns the best available WindowManager based on the environment.
func NewWindowManager() WindowManager {
	// Check Hyprland
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		if _, err := exec.LookPath("hyprctl"); err == nil {
			return &HyprlandManager{}
		}
	}
	// Check Sway
	if os.Getenv("SWAYSOCK") != "" {
		if _, err := exec.LookPath("swaymsg"); err == nil {
			return &SwayManager{}
		}
	}
	// Check X11
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("wmctrl"); err == nil {
			return &X11Manager{}
		}
	}
	return &NoopManager{}
}
