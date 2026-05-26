package input

import (
	"context"
	"fmt"
)

// NoopController es un controlador fallback que devuelve errores descriptivos.
type NoopController struct{}

func (c *NoopController) Name() string { return "noop" }

func (c *NoopController) TypeText(_ context.Context, _ string) error {
	return fmt.Errorf("simulación de teclado no disponible: no se encontró xdotool ni wtype")
}

func (c *NoopController) PressKey(_ context.Context, _ string) error {
	return fmt.Errorf("simulación de teclado no disponible: no se encontró xdotool ni wtype")
}

func (c *NoopController) Hotkey(_ context.Context, _ []string) error {
	return fmt.Errorf("simulación de teclado no disponible: no se encontró xdotool ni wtype")
}

func (c *NoopController) MoveMouse(_ context.Context, _, _ int) error {
	return fmt.Errorf("simulación de ratón no disponible: no se encontró xdotool ni ydotool")
}

func (c *NoopController) Click(_ context.Context, _ string) error {
	return fmt.Errorf("simulación de ratón no disponible: no se encontró xdotool ni ydotool")
}

func (c *NoopController) Scroll(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("simulación de ratón no disponible: no se encontró xdotool ni ydotool")
}
