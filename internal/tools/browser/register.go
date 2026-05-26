package browser

import (
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete browser en el Registry.
func RegisterTools(reg *executor.Registry) error {
	if err := reg.Register(NewOpenURLTool()); err != nil {
		return err
	}
	if err := reg.Register(NewSearchTool()); err != nil {
		return err
	}
	if err := reg.Register(NewYouTubePlayTool()); err != nil {
		return err
	}
	if err := reg.Register(NewYouTubeSearchTool()); err != nil {
		return err
	}
	if err := reg.Register(NewReadURLTool()); err != nil {
		return err
	}
	// Herramientas de sesión (reutilización de ventanas)
	if err := reg.Register(NewOpenOrReuseTool(defaultWindowListFunc, defaultWindowFocusFunc)); err != nil {
		return err
	}
	if err := reg.Register(NewYouTubeOpenOrReuseTool(defaultWindowListFunc, defaultWindowFocusFunc)); err != nil {
		return err
	}
	return nil
}
