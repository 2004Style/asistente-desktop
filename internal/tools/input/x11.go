package input

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// X11Controller usa xdotool para simular entrada en entornos X11.
type X11Controller struct{}

func (c *X11Controller) Name() string { return "x11-xdotool" }

func (c *X11Controller) TypeText(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "xdotool", "type", "--clearmodifiers", "--", text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool type falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *X11Controller) PressKey(ctx context.Context, key string) error {
	cmd := exec.CommandContext(ctx, "xdotool", "key", key)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool key falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *X11Controller) Hotkey(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("se requiere al menos una tecla")
	}
	// xdotool espera "ctrl+l" o "ctrl+shift+t"
	combo := strings.Join(keys, "+")
	cmd := exec.CommandContext(ctx, "xdotool", "key", combo)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool hotkey falló (%s): %v — %s", combo, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *X11Controller) MoveMouse(ctx context.Context, x, y int) error {
	cmd := exec.CommandContext(ctx, "xdotool", "mousemove", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool mousemove falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *X11Controller) Click(ctx context.Context, button string) error {
	var buttonNum string
	switch strings.ToLower(button) {
	case "left", "izquierda", "":
		buttonNum = "1"
	case "middle", "medio", "centro":
		buttonNum = "2"
	case "right", "derecha":
		buttonNum = "3"
	case "double", "doble":
		cmd := exec.CommandContext(ctx, "xdotool", "click", "--repeat", "2", "1")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("xdotool doble clic falló: %v — %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	default:
		buttonNum = "1"
	}
	cmd := exec.CommandContext(ctx, "xdotool", "click", buttonNum)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("xdotool click falló: %v — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *X11Controller) Scroll(ctx context.Context, direction string, amount int) error {
	if amount <= 0 {
		amount = 3
	}
	// Botón 4 = scroll arriba, botón 5 = scroll abajo
	buttonNum := "4"
	if strings.ToLower(direction) == "down" || strings.ToLower(direction) == "abajo" {
		buttonNum = "5"
	}
	for i := 0; i < amount; i++ {
		cmd := exec.CommandContext(ctx, "xdotool", "click", buttonNum)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("xdotool scroll falló: %v — %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
