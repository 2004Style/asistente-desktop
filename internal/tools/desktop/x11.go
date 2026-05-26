package desktop

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// X11Manager implements WindowManager using wmctrl and xdotool.
type X11Manager struct{}

func (x *X11Manager) Name() string { return "x11" }

// parseWmctrlLine parses a line from `wmctrl -lpx`.
// Format: window_id desktop pid class host title
func parseWmctrlLine(line string) (WindowInfo, bool) {
	// Fields are separated by whitespace, but title may contain spaces
	// wmctrl -lpx format:
	// 0x04600001  0  12345  firefox.Firefox  hostname  Firefox - Google
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return WindowInfo{}, false
	}

	id := fields[0]
	pid, _ := strconv.Atoi(fields[2])
	classField := fields[3] // e.g. "firefox.Firefox"
	// title is everything from field 5 onwards
	title := strings.Join(fields[5:], " ")

	// class is the WM_CLASS: "instance.Class"
	class := classField
	app := classField
	if parts := strings.SplitN(classField, ".", 2); len(parts) == 2 {
		class = parts[1]
		app = parts[1]
	}

	return WindowInfo{
		ID:    id,
		Title: title,
		App:   app,
		Class: class,
		PID:   pid,
	}, true
}

func (x *X11Manager) ListWindows(ctx context.Context) ([]WindowInfo, error) {
	out, err := exec.CommandContext(ctx, "wmctrl", "-lpx").Output()
	if err != nil {
		return nil, fmt.Errorf("wmctrl -lpx: %w", err)
	}

	var windows []WindowInfo
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if w, ok := parseWmctrlLine(line); ok {
			windows = append(windows, w)
		}
	}
	return windows, nil
}

func (x *X11Manager) ActiveWindow(ctx context.Context) (*WindowInfo, error) {
	// xdotool getactivewindow returns a decimal ID
	out, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").Output()
	if err != nil {
		return nil, fmt.Errorf("xdotool getactivewindow: %w", err)
	}

	decimalID := strings.TrimSpace(string(out))
	// Convert to hex format to match wmctrl output
	decID, err := strconv.ParseInt(decimalID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing xdotool window id: %w", err)
	}
	hexID := fmt.Sprintf("0x%08x", decID)

	windows, err := x.ListWindows(ctx)
	if err != nil {
		return nil, err
	}

	for _, w := range windows {
		if strings.EqualFold(w.ID, hexID) {
			w.Focused = true
			return &w, nil
		}
	}

	// Fallback: return minimal info with just the ID
	return &WindowInfo{ID: hexID, Focused: true}, nil
}

func (x *X11Manager) resolveWindow(ctx context.Context, selector WindowSelector) (*WindowInfo, error) {
	if selector.Active {
		return x.ActiveWindow(ctx)
	}

	windows, err := x.ListWindows(ctx)
	if err != nil {
		return nil, err
	}

	if selector.ID != "" {
		for _, w := range windows {
			if strings.EqualFold(w.ID, selector.ID) {
				return &w, nil
			}
		}
	}

	if selector.Class != "" && selector.Title != "" {
		for _, w := range windows {
			if strings.EqualFold(w.Class, selector.Class) && strings.Contains(strings.ToLower(w.Title), strings.ToLower(selector.Title)) {
				return &w, nil
			}
		}
	}

	if selector.Title != "" {
		for _, w := range windows {
			if strings.Contains(strings.ToLower(w.Title), strings.ToLower(selector.Title)) {
				return &w, nil
			}
		}
	}

	if selector.App != "" {
		for _, w := range windows {
			if strings.EqualFold(w.App, selector.App) || strings.Contains(strings.ToLower(w.App), strings.ToLower(selector.App)) {
				return &w, nil
			}
		}
	}

	if selector.Class != "" {
		for _, w := range windows {
			if strings.EqualFold(w.Class, selector.Class) {
				return &w, nil
			}
		}
	}

	return nil, fmt.Errorf("no window matched the given selector")
}

func (x *X11Manager) FocusWindow(ctx context.Context, selector WindowSelector) error {
	win, err := x.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	return exec.CommandContext(ctx, "wmctrl", "-ia", win.ID).Run()
}

func (x *X11Manager) CloseWindow(ctx context.Context, selector WindowSelector) error {
	win, err := x.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	return exec.CommandContext(ctx, "wmctrl", "-ic", win.ID).Run()
}

func (x *X11Manager) MoveToWorkspace(ctx context.Context, selector WindowSelector, workspace string) error {
	win, err := x.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	// wmctrl uses -t for workspace number; attempt numeric conversion
	wsNum := workspace
	return exec.CommandContext(ctx, "wmctrl", "-ir", win.ID, "-t", wsNum).Run()
}
