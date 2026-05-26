package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// HyprlandManager implements WindowManager using hyprctl.
type HyprlandManager struct{}

func (h *HyprlandManager) Name() string { return "hyprland" }

// hyprlandClient is the JSON structure returned by `hyprctl clients -j`
type hyprlandClient struct {
	Address         string `json:"address"`
	PID             int    `json:"pid"`
	Class           string `json:"class"`
	Title           string `json:"title"`
	FocusHistoryID  int    `json:"focusHistoryID"`
	Workspace       struct {
		Name string `json:"name"`
	} `json:"workspace"`
}

// ListWindows calls `hyprctl clients -j` and returns all windows.
func (h *HyprlandManager) ListWindows(ctx context.Context) ([]WindowInfo, error) {
	out, err := exec.CommandContext(ctx, "hyprctl", "clients", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl clients -j: %w", err)
	}

	var clients []hyprlandClient
	if err := json.Unmarshal(out, &clients); err != nil {
		return nil, fmt.Errorf("parsing hyprctl clients output: %w", err)
	}

	// Determine the focused window (focusHistoryID == 0 is most recently focused)
	windows := make([]WindowInfo, 0, len(clients))
	for _, c := range clients {
		windows = append(windows, WindowInfo{
			ID:        c.Address,
			Address:   c.Address,
			Title:     c.Title,
			App:       c.Class,
			Class:     c.Class,
			PID:       c.PID,
			Workspace: c.Workspace.Name,
			Focused:   c.FocusHistoryID == 0,
		})
	}
	return windows, nil
}

// ActiveWindow calls `hyprctl activewindow -j`.
func (h *HyprlandManager) ActiveWindow(ctx context.Context) (*WindowInfo, error) {
	out, err := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl activewindow -j: %w", err)
	}

	var c hyprlandClient
	if err := json.Unmarshal(out, &c); err != nil {
		return nil, fmt.Errorf("parsing hyprctl activewindow output: %w", err)
	}

	// hyprctl returns {"address":"0x0",...} when no window is active
	if c.Address == "" || c.Address == "0x0" {
		return nil, fmt.Errorf("no active window")
	}

	return &WindowInfo{
		ID:        c.Address,
		Address:   c.Address,
		Title:     c.Title,
		App:       c.Class,
		Class:     c.Class,
		PID:       c.PID,
		Workspace: c.Workspace.Name,
		Focused:   true,
	}, nil
}

// resolveWindow resolves a selector to a WindowInfo.
func (h *HyprlandManager) resolveWindow(ctx context.Context, selector WindowSelector) (*WindowInfo, error) {
	// Priority 1: Address
	if selector.Address != "" {
		return &WindowInfo{Address: selector.Address}, nil
	}

	// Priority 2: Active window
	if selector.Active {
		return h.ActiveWindow(ctx)
	}

	// Need to list windows for remaining priorities
	windows, err := h.ListWindows(ctx)
	if err != nil {
		return nil, err
	}

	// Priority 3: ID (same as address in Hyprland)
	if selector.ID != "" {
		for _, w := range windows {
			if w.ID == selector.ID {
				return &w, nil
			}
		}
	}

	// Priority 4: Class + Title match
	if selector.Class != "" && selector.Title != "" {
		for _, w := range windows {
			if strings.EqualFold(w.Class, selector.Class) && strings.Contains(strings.ToLower(w.Title), strings.ToLower(selector.Title)) {
				return &w, nil
			}
		}
	}

	// Priority 5: Title only
	if selector.Title != "" {
		for _, w := range windows {
			if strings.Contains(strings.ToLower(w.Title), strings.ToLower(selector.Title)) {
				return &w, nil
			}
		}
	}

	// Priority 6: App name match
	if selector.App != "" {
		for _, w := range windows {
			if strings.EqualFold(w.App, selector.App) || strings.Contains(strings.ToLower(w.App), strings.ToLower(selector.App)) {
				return &w, nil
			}
		}
	}

	// Priority 7: Class only
	if selector.Class != "" {
		for _, w := range windows {
			if strings.EqualFold(w.Class, selector.Class) {
				return &w, nil
			}
		}
	}

	// Priority 8: Workspace filter (return first in workspace)
	if selector.Workspace != "" {
		for _, w := range windows {
			if w.Workspace == selector.Workspace {
				return &w, nil
			}
		}
	}

	return nil, fmt.Errorf("no window matched the given selector")
}

// FocusWindow focuses the window described by the selector.
func (h *HyprlandManager) FocusWindow(ctx context.Context, selector WindowSelector) error {
	win, err := h.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}

	// Primary: address dispatch
	if win.Address != "" {
		cmd := exec.CommandContext(ctx, "hyprctl", "dispatch", "focuswindow", "address:"+win.Address)
		if out, err := cmd.CombinedOutput(); err != nil {
			// Fallback to class
			if win.Class != "" {
				_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "focuswindow", "class:"+win.Class).Run()
				return nil
			}
			return fmt.Errorf("focuswindow failed: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	if win.Class != "" {
		return exec.CommandContext(ctx, "hyprctl", "dispatch", "focuswindow", "class:"+win.Class).Run()
	}
	return fmt.Errorf("cannot focus window: no address or class available")
}

// CloseWindow closes the window described by the selector.
func (h *HyprlandManager) CloseWindow(ctx context.Context, selector WindowSelector) error {
	win, err := h.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}

	if win.Address != "" {
		return exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "address:"+win.Address).Run()
	}
	if win.Class != "" {
		return exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:"+win.Class).Run()
	}
	return fmt.Errorf("cannot close window: no address or class available")
}

// MoveToWorkspace moves the window to the given workspace.
func (h *HyprlandManager) MoveToWorkspace(ctx context.Context, selector WindowSelector, workspace string) error {
	win, err := h.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}

	target := workspace + ",address:" + win.Address
	return exec.CommandContext(ctx, "hyprctl", "dispatch", "movetoworkspacesilent", target).Run()
}
