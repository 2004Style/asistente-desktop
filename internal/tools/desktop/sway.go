package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// SwayManager implements WindowManager using swaymsg.
type SwayManager struct{}

func (s *SwayManager) Name() string { return "sway" }

// swayNode represents a node in the sway tree JSON.
type swayNode struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	AppID           string     `json:"app_id"`
	PID             int        `json:"pid"`
	Focused         bool       `json:"focused"`
	WindowProps     *swayWinProps `json:"window_properties"`
	Nodes           []swayNode `json:"nodes"`
	FloatingNodes   []swayNode `json:"floating_nodes"`
}

type swayWinProps struct {
	Class    string `json:"class"`
	Instance string `json:"instance"`
	Title    string `json:"title"`
}

// flattenSwayNodes extracts all leaf container windows from the tree.
func flattenSwayNodes(node swayNode) []swayNode {
	var result []swayNode

	isWindow := node.Type == "con" && (node.AppID != "" || node.WindowProps != nil) && node.Name != ""
	if isWindow {
		result = append(result, node)
	}

	for _, child := range node.Nodes {
		result = append(result, flattenSwayNodes(child)...)
	}
	for _, child := range node.FloatingNodes {
		result = append(result, flattenSwayNodes(child)...)
	}
	return result
}

func swayNodeToWindowInfo(n swayNode, workspace string) WindowInfo {
	class := n.AppID
	title := n.Name
	if n.WindowProps != nil {
		if class == "" {
			class = n.WindowProps.Class
		}
		if n.WindowProps.Title != "" {
			title = n.WindowProps.Title
		}
	}
	return WindowInfo{
		ID:        fmt.Sprintf("%d", n.ID),
		Title:     title,
		App:       class,
		Class:     class,
		PID:       n.PID,
		Workspace: workspace,
		Focused:   n.Focused,
	}
}

// getSwayTree fetches and parses the full sway tree.
func getSwayTree(ctx context.Context) ([]WindowInfo, error) {
	out, err := exec.CommandContext(ctx, "swaymsg", "-t", "get_tree").Output()
	if err != nil {
		return nil, fmt.Errorf("swaymsg -t get_tree: %w", err)
	}

	var root swayNode
	if err := json.Unmarshal(out, &root); err != nil {
		return nil, fmt.Errorf("parsing sway tree: %w", err)
	}

	// Walk outputs → workspaces → windows
	var windows []WindowInfo
	for _, output := range root.Nodes {
		for _, ws := range output.Nodes {
			if ws.Type != "workspace" {
				continue
			}
			wsName := ws.Name
			for _, win := range flattenSwayNodes(ws) {
				info := swayNodeToWindowInfo(win, wsName)
				windows = append(windows, info)
			}
		}
	}
	return windows, nil
}

func (s *SwayManager) ListWindows(ctx context.Context) ([]WindowInfo, error) {
	return getSwayTree(ctx)
}

func (s *SwayManager) ActiveWindow(ctx context.Context) (*WindowInfo, error) {
	windows, err := s.ListWindows(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		if w.Focused {
			return &w, nil
		}
	}
	return nil, fmt.Errorf("no focused window found")
}

func (s *SwayManager) resolveWindow(ctx context.Context, selector WindowSelector) (*WindowInfo, error) {
	if selector.Active {
		return s.ActiveWindow(ctx)
	}

	windows, err := s.ListWindows(ctx)
	if err != nil {
		return nil, err
	}

	if selector.ID != "" {
		for _, w := range windows {
			if w.ID == selector.ID {
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

	if selector.Workspace != "" {
		for _, w := range windows {
			if w.Workspace == selector.Workspace {
				return &w, nil
			}
		}
	}

	return nil, fmt.Errorf("no window matched the given selector")
}

func (s *SwayManager) FocusWindow(ctx context.Context, selector WindowSelector) error {
	win, err := s.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	return exec.CommandContext(ctx, "swaymsg", fmt.Sprintf("[con_id=%s] focus", win.ID)).Run()
}

func (s *SwayManager) CloseWindow(ctx context.Context, selector WindowSelector) error {
	win, err := s.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	return exec.CommandContext(ctx, "swaymsg", fmt.Sprintf("[con_id=%s] kill", win.ID)).Run()
}

func (s *SwayManager) MoveToWorkspace(ctx context.Context, selector WindowSelector, workspace string) error {
	win, err := s.resolveWindow(ctx, selector)
	if err != nil {
		return err
	}
	return exec.CommandContext(ctx, "swaymsg", fmt.Sprintf("[con_id=%s] move container to workspace %s", win.ID, workspace)).Run()
}
