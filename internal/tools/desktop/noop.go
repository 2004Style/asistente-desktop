package desktop

import (
	"context"
	"fmt"
)

// NoopManager is a fallback WindowManager when no desktop backend is available.
type NoopManager struct{}

func (n *NoopManager) Name() string { return "noop" }

func (n *NoopManager) ListWindows(_ context.Context) ([]WindowInfo, error) {
	return nil, fmt.Errorf("no window manager backend available: install hyprctl (Hyprland), swaymsg (Sway), or wmctrl (X11)")
}

func (n *NoopManager) ActiveWindow(_ context.Context) (*WindowInfo, error) {
	return nil, fmt.Errorf("no window manager backend available: install hyprctl (Hyprland), swaymsg (Sway), or wmctrl (X11)")
}

func (n *NoopManager) FocusWindow(_ context.Context, _ WindowSelector) error {
	return fmt.Errorf("no window manager backend available: install hyprctl (Hyprland), swaymsg (Sway), or wmctrl (X11)")
}

func (n *NoopManager) CloseWindow(_ context.Context, _ WindowSelector) error {
	return fmt.Errorf("no window manager backend available: install hyprctl (Hyprland), swaymsg (Sway), or wmctrl (X11)")
}

func (n *NoopManager) MoveToWorkspace(_ context.Context, _ WindowSelector, _ string) error {
	return fmt.Errorf("no window manager backend available: install hyprctl (Hyprland), swaymsg (Sway), or wmctrl (X11)")
}
