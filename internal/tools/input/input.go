package input

import (
	"context"
	"os"
	"os/exec"
)

// InputController define la interfaz para todos los backends de simulación de entrada.
type InputController interface {
	Name() string
	TypeText(ctx context.Context, text string) error
	PressKey(ctx context.Context, key string) error
	Hotkey(ctx context.Context, keys []string) error
	MoveMouse(ctx context.Context, x, y int) error
	Click(ctx context.Context, button string) error
	Scroll(ctx context.Context, direction string, amount int) error
}

// NewInputController devuelve el mejor controlador disponible para el entorno actual.
func NewInputController() InputController {
	// Intentar X11 primero
	if os.Getenv("DISPLAY") != "" {
		if _, err := exec.LookPath("xdotool"); err == nil {
			return &X11Controller{}
		}
	}
	// Intentar Wayland (wtype + ydotool)
	if os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		return newWaylandController()
	}
	return &NoopController{}
}
