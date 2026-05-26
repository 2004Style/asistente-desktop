package input

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rbot/internal/executor"
)

// ─── TypeText ────────────────────────────────────────────────────────────────

// TypeTextTool simula la escritura de texto en la ventana activa.
type TypeTextTool struct {
	ctrl InputController
}

func NewTypeTextTool() *TypeTextTool {
	return &TypeTextTool{ctrl: NewInputController()}
}

func (t *TypeTextTool) Name() string        { return "input.type_text" }
func (t *TypeTextTool) Category() string    { return "input" }
func (t *TypeTextTool) RiskLevel() string   { return "medium" }
func (t *TypeTextTool) Description() string {
	return "Escribe texto en la ventana activa simulando pulsaciones de teclado."
}
func (t *TypeTextTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Texto a escribir en la ventana activa.",
			},
			"window_class": map[string]interface{}{
				"type":        "string",
				"description": "Clase de ventana destino (opcional, ej: 'firefox', 'code').",
			},
		},
		"required": []string{"text"},
	}
}

func (t *TypeTextTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	text, _ := args["text"].(string)
	if text == "" {
		return nil, fmt.Errorf("el argumento 'text' es obligatorio")
	}

	// Seguridad: bloquear datos sensibles
	textLower := strings.ToLower(text)
	sensitiveKeywords := []string{"contraseña", "password", "clave", "token", "api_key", "secret", "passwd"}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(textLower, kw) {
			return nil, fmt.Errorf("escritura de datos sensibles bloqueada")
		}
	}

	// Bloquear textos demasiado largos
	if len(text) > 1000 {
		return nil, fmt.Errorf("texto demasiado largo: máximo 1000 caracteres, recibido %d", len(text))
	}

	if err := t.ctrl.TypeText(ctx, text); err != nil {
		return nil, err
	}

	riskNote := ""
	if len(text) > 200 {
		riskNote = " (texto largo, riesgo alto)"
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Texto escrito correctamente (%d caracteres)%s.", len(text), riskNote),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── PressKey ────────────────────────────────────────────────────────────────

// PressKeyTool simula la pulsación de una tecla individual.
type PressKeyTool struct {
	ctrl InputController
}

func NewPressKeyTool() *PressKeyTool {
	return &PressKeyTool{ctrl: NewInputController()}
}

func (t *PressKeyTool) Name() string        { return "input.press_key" }
func (t *PressKeyTool) Category() string    { return "input" }
func (t *PressKeyTool) RiskLevel() string   { return "low" }
func (t *PressKeyTool) Description() string {
	return "Pulsa una tecla individual en el sistema (ej: escape, enter, tab, f5)."
}
func (t *PressKeyTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Tecla a pulsar (ej: 'escape', 'enter', 'tab', 'f5', 'retroceso').",
			},
		},
		"required": []string{"key"},
	}
}

func (t *PressKeyTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	key, _ := args["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("el argumento 'key' es obligatorio")
	}

	normalized := NormalizeSingleKey(key)
	if err := t.ctrl.PressKey(ctx, normalized); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tecla '%s' pulsada.", normalized),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Hotkey ──────────────────────────────────────────────────────────────────

// HotkeyTool ejecuta una combinación de teclas (atajo de teclado).
type HotkeyTool struct {
	ctrl InputController
}

func NewHotkeyTool() *HotkeyTool {
	return &HotkeyTool{ctrl: NewInputController()}
}

func (t *HotkeyTool) Name() string        { return "input.hotkey" }
func (t *HotkeyTool) Category() string    { return "input" }
func (t *HotkeyTool) RiskLevel() string   { return "medium" }
func (t *HotkeyTool) Description() string {
	return "Ejecuta una combinación de teclas o atajo (ej: ctrl+c, alt+tab, ctrl+shift+t)."
}
func (t *HotkeyTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keys": map[string]interface{}{
				"description": "Teclas a pulsar simultáneamente. Puede ser un string ('ctrl+l') o array (['ctrl','l']).",
			},
		},
		"required": []string{"keys"},
	}
}

// dangerousHotkeys define combinaciones peligrosas bloqueadas.
var dangerousHotkeys = [][]string{
	{"alt", "f4"},
	{"ctrl", "alt", "delete"},
	{"ctrl", "alt", "del"},
	{"super", "l"},     // bloquear pantalla (podría ser legítimo, pero bloqueamos por precaución)
}

func isDangerousHotkey(keys []string) bool {
	normalized := make([]string, len(keys))
	for i, k := range keys {
		normalized[i] = strings.ToLower(k)
	}

	for _, dangerous := range dangerousHotkeys {
		if len(normalized) != len(dangerous) {
			continue
		}
		match := true
		for i, d := range dangerous {
			if normalized[i] != d {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (t *HotkeyTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	var keys []string
	switch v := args["keys"].(type) {
	case string:
		keys = NormalizeKeys(v)
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				keys = append(keys, s)
			}
		}
		// Normalizar cada parte
		var normalized []string
		for _, k := range keys {
			parts := NormalizeKeys(k)
			normalized = append(normalized, parts...)
		}
		keys = normalized
	case []string:
		keys = v
	default:
		return nil, fmt.Errorf("el argumento 'keys' debe ser un string o array de strings")
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("se requiere al menos una tecla")
	}

	if isDangerousHotkey(keys) {
		return nil, fmt.Errorf("combinación de teclas peligrosa bloqueada: %s", strings.Join(keys, "+"))
	}

	if err := t.ctrl.Hotkey(ctx, keys); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Atajo '%s' ejecutado.", strings.Join(keys, "+")),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── MouseMove ───────────────────────────────────────────────────────────────

// MouseMoveTool mueve el cursor del ratón a coordenadas absolutas.
type MouseMoveTool struct {
	ctrl InputController
}

func NewMouseMoveTool() *MouseMoveTool {
	return &MouseMoveTool{ctrl: NewInputController()}
}

func (t *MouseMoveTool) Name() string        { return "input.mouse_move" }
func (t *MouseMoveTool) Category() string    { return "input" }
func (t *MouseMoveTool) RiskLevel() string   { return "medium" }
func (t *MouseMoveTool) Description() string {
	return "Mueve el cursor del ratón a las coordenadas de pantalla especificadas."
}
func (t *MouseMoveTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"x": map[string]interface{}{
				"type":        "integer",
				"description": "Coordenada X en píxeles.",
			},
			"y": map[string]interface{}{
				"type":        "integer",
				"description": "Coordenada Y en píxeles.",
			},
		},
		"required": []string{"x", "y"},
	}
}

func (t *MouseMoveTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	x, err := toInt(args["x"])
	if err != nil {
		return nil, fmt.Errorf("argumento 'x' inválido: %v", err)
	}
	y, err := toInt(args["y"])
	if err != nil {
		return nil, fmt.Errorf("argumento 'y' inválido: %v", err)
	}

	if err := t.ctrl.MoveMouse(ctx, x, y); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Ratón movido a (%d, %d).", x, y),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── MouseClick ──────────────────────────────────────────────────────────────

// MouseClickTool simula un clic del ratón.
type MouseClickTool struct {
	ctrl InputController
}

func NewMouseClickTool() *MouseClickTool {
	return &MouseClickTool{ctrl: NewInputController()}
}

func (t *MouseClickTool) Name() string        { return "input.mouse_click" }
func (t *MouseClickTool) Category() string    { return "input" }
func (t *MouseClickTool) RiskLevel() string   { return "medium" }
func (t *MouseClickTool) Description() string {
	return "Simula un clic del ratón (izquierdo, derecho, medio o doble)."
}
func (t *MouseClickTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"button": map[string]interface{}{
				"type":        "string",
				"description": "Botón a pulsar: 'left' (izquierdo, por defecto), 'right', 'middle', 'double'.",
				"enum":        []string{"left", "right", "middle", "double"},
			},
		},
	}
}

func (t *MouseClickTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	button, _ := args["button"].(string)
	if button == "" {
		button = "left"
	}

	if err := t.ctrl.Click(ctx, button); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Clic '%s' ejecutado.", button),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── MouseScroll ─────────────────────────────────────────────────────────────

// MouseScrollTool simula el desplazamiento de la rueda del ratón.
type MouseScrollTool struct {
	ctrl InputController
}

func NewMouseScrollTool() *MouseScrollTool {
	return &MouseScrollTool{ctrl: NewInputController()}
}

func (t *MouseScrollTool) Name() string        { return "input.mouse_scroll" }
func (t *MouseScrollTool) Category() string    { return "input" }
func (t *MouseScrollTool) RiskLevel() string   { return "low" }
func (t *MouseScrollTool) Description() string {
	return "Desplaza la rueda del ratón hacia arriba o abajo."
}
func (t *MouseScrollTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"direction": map[string]interface{}{
				"type":        "string",
				"description": "Dirección del desplazamiento: 'up' (arriba) o 'down' (abajo).",
				"enum":        []string{"up", "down"},
			},
			"amount": map[string]interface{}{
				"type":        "integer",
				"description": "Número de pasos de desplazamiento (por defecto: 3).",
			},
		},
		"required": []string{"direction"},
	}
}

func (t *MouseScrollTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	direction, _ := args["direction"].(string)
	if direction == "" {
		direction = "down"
	}

	amount := 3
	if v, err := toInt(args["amount"]); err == nil && v > 0 {
		amount = v
	}

	if err := t.ctrl.Scroll(ctx, direction, amount); err != nil {
		return nil, err
	}

	dirText := "abajo"
	if strings.ToLower(direction) == "up" || strings.ToLower(direction) == "arriba" {
		dirText = "arriba"
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Desplazado %d pasos hacia %s.", amount, dirText),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case float32:
		return int(n), nil
	case nil:
		return 0, fmt.Errorf("valor nulo")
	default:
		return 0, fmt.Errorf("tipo no soportado: %T", v)
	}
}
