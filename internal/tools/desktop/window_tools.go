package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rbot/internal/executor"
)

// ─── List Windows ────────────────────────────────────────────────────────────

type ListWindowsTool struct {
	WM WindowManager
}

func NewListWindowsTool(wm WindowManager) *ListWindowsTool {
	return &ListWindowsTool{WM: wm}
}

func (t *ListWindowsTool) Name() string        { return "desktop.list_windows" }
func (t *ListWindowsTool) Category() string    { return "desktop" }
func (t *ListWindowsTool) RiskLevel() string   { return "low" }
func (t *ListWindowsTool) Description() string {
	return "Lista todas las ventanas abiertas en el escritorio, con información de título, app, workspace y estado de foco."
}
func (t *ListWindowsTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ListWindowsTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	windows, err := t.WM.ListWindows(ctx)
	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	raw, err := json.Marshal(windows)
	if err != nil {
		return nil, fmt.Errorf("serializing windows: %w", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("%d ventana(s) abiertas.", len(windows)),
		Data:       map[string]interface{}{"windows": json.RawMessage(raw)},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Active Window ───────────────────────────────────────────────────────────

type ActiveWindowTool struct {
	WM WindowManager
}

func NewActiveWindowTool(wm WindowManager) *ActiveWindowTool {
	return &ActiveWindowTool{WM: wm}
}

func (t *ActiveWindowTool) Name() string        { return "desktop.active_window" }
func (t *ActiveWindowTool) Category() string    { return "desktop" }
func (t *ActiveWindowTool) RiskLevel() string   { return "low" }
func (t *ActiveWindowTool) Description() string {
	return "Devuelve información sobre la ventana actualmente enfocada (título, app, workspace, PID, etc.)."
}
func (t *ActiveWindowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ActiveWindowTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	win, err := t.WM.ActiveWindow(ctx)
	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	raw, err := json.Marshal(win)
	if err != nil {
		return nil, fmt.Errorf("serializing window: %w", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Ventana activa: '%s' (%s)", win.Title, win.App),
		Data:       map[string]interface{}{"window": json.RawMessage(raw)},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Focus Window ────────────────────────────────────────────────────────────

type FocusWindowTool struct {
	WM WindowManager
}

func NewFocusWindowTool(wm WindowManager) *FocusWindowTool {
	return &FocusWindowTool{WM: wm}
}

func (t *FocusWindowTool) Name() string        { return "desktop.focus_window" }
func (t *FocusWindowTool) Category() string    { return "desktop" }
func (t *FocusWindowTool) RiskLevel() string   { return "medium" }
func (t *FocusWindowTool) Description() string {
	return "Enfoca (trae al frente) una ventana específica identificada por título, app, clase, dirección o workspace."
}
func (t *FocusWindowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"selector": map[string]interface{}{
				"type":        "object",
				"description": "Selector para identificar la ventana. Al menos uno de los campos debe estar presente.",
				"properties": map[string]interface{}{
					"id":        map[string]interface{}{"type": "string", "description": "ID interno de la ventana."},
					"address":   map[string]interface{}{"type": "string", "description": "Dirección hex (Hyprland)."},
					"app":       map[string]interface{}{"type": "string", "description": "Nombre de la aplicación."},
					"title":     map[string]interface{}{"type": "string", "description": "Título (parcial) de la ventana."},
					"class":     map[string]interface{}{"type": "string", "description": "Clase WM de la ventana."},
					"workspace": map[string]interface{}{"type": "string", "description": "Nombre o número del workspace."},
					"active":    map[string]interface{}{"type": "boolean", "description": "Si es true, enfoca la ventana activa."},
				},
			},
		},
		"required": []string{"selector"},
	}
}

func (t *FocusWindowTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	sel, err := parseSelectorArg(args)
	if err != nil {
		return nil, err
	}

	if err := t.WM.FocusWindow(ctx, sel); err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       "Ventana enfocada correctamente.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Close Window ────────────────────────────────────────────────────────────

type CloseWindowTool struct {
	WM WindowManager
}

func NewCloseWindowTool(wm WindowManager) *CloseWindowTool {
	return &CloseWindowTool{WM: wm}
}

func (t *CloseWindowTool) Name() string        { return "desktop.close_window" }
func (t *CloseWindowTool) Category() string    { return "desktop" }
func (t *CloseWindowTool) RiskLevel() string   { return "high" }
func (t *CloseWindowTool) Description() string {
	return "Cierra una ventana específica identificada por título, app, clase, dirección o workspace. Riesgo alto: puede causar pérdida de datos."
}
func (t *CloseWindowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"selector": map[string]interface{}{
				"type":        "object",
				"description": "Selector para identificar la ventana a cerrar.",
				"properties": map[string]interface{}{
					"id":        map[string]interface{}{"type": "string", "description": "ID interno de la ventana."},
					"address":   map[string]interface{}{"type": "string", "description": "Dirección hex (Hyprland)."},
					"app":       map[string]interface{}{"type": "string", "description": "Nombre de la aplicación."},
					"title":     map[string]interface{}{"type": "string", "description": "Título (parcial) de la ventana."},
					"class":     map[string]interface{}{"type": "string", "description": "Clase WM de la ventana."},
					"workspace": map[string]interface{}{"type": "string", "description": "Nombre o número del workspace."},
					"active":    map[string]interface{}{"type": "boolean", "description": "Si es true, cierra la ventana activa."},
				},
			},
		},
		"required": []string{"selector"},
	}
}

func (t *CloseWindowTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	sel, err := parseSelectorArg(args)
	if err != nil {
		return nil, err
	}

	if err := t.WM.CloseWindow(ctx, sel); err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       "Ventana cerrada correctamente.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Move Window ─────────────────────────────────────────────────────────────

type MoveWindowTool struct {
	WM WindowManager
}

func NewMoveWindowTool(wm WindowManager) *MoveWindowTool {
	return &MoveWindowTool{WM: wm}
}

func (t *MoveWindowTool) Name() string        { return "desktop.move_window" }
func (t *MoveWindowTool) Category() string    { return "desktop" }
func (t *MoveWindowTool) RiskLevel() string   { return "medium" }
func (t *MoveWindowTool) Description() string {
	return "Mueve una ventana a otro workspace (espacio de trabajo) del escritorio."
}
func (t *MoveWindowTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"selector": map[string]interface{}{
				"type":        "object",
				"description": "Selector para identificar la ventana.",
				"properties": map[string]interface{}{
					"id":        map[string]interface{}{"type": "string", "description": "ID interno de la ventana."},
					"address":   map[string]interface{}{"type": "string", "description": "Dirección hex (Hyprland)."},
					"app":       map[string]interface{}{"type": "string", "description": "Nombre de la aplicación."},
					"title":     map[string]interface{}{"type": "string", "description": "Título (parcial) de la ventana."},
					"class":     map[string]interface{}{"type": "string", "description": "Clase WM de la ventana."},
					"workspace": map[string]interface{}{"type": "string", "description": "Workspace origen (para filtrar)."},
					"active":    map[string]interface{}{"type": "boolean", "description": "Si es true, usa la ventana activa."},
				},
			},
			"workspace": map[string]interface{}{
				"type":        "string",
				"description": "Workspace destino (nombre o número).",
			},
		},
		"required": []string{"selector", "workspace"},
	}
}

func (t *MoveWindowTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	sel, err := parseSelectorArg(args)
	if err != nil {
		return nil, err
	}

	workspace, _ := args["workspace"].(string)
	if workspace == "" {
		return nil, fmt.Errorf("el argumento 'workspace' es obligatorio")
	}

	if err := t.WM.MoveToWorkspace(ctx, sel, workspace); err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Ventana movida al workspace '%s'.", workspace),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseSelectorArg extracts and decodes the "selector" field from tool args.
func parseSelectorArg(args map[string]interface{}) (WindowSelector, error) {
	raw, ok := args["selector"]
	if !ok {
		return WindowSelector{}, fmt.Errorf("el argumento 'selector' es obligatorio")
	}

	// Re-encode to JSON and decode into WindowSelector for type safety
	b, err := json.Marshal(raw)
	if err != nil {
		return WindowSelector{}, fmt.Errorf("codificando selector: %w", err)
	}

	var sel WindowSelector
	if err := json.Unmarshal(b, &sel); err != nil {
		return WindowSelector{}, fmt.Errorf("decodificando selector: %w", err)
	}

	return sel, nil
}
