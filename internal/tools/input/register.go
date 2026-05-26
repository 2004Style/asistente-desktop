package input

import "rbot/internal/executor"

// RegisterTools registra todas las herramientas del paquete input en el Registry.
func RegisterTools(reg *executor.Registry) error {
	if err := reg.Register(NewTypeTextTool()); err != nil {
		return err
	}
	if err := reg.Register(NewPressKeyTool()); err != nil {
		return err
	}
	if err := reg.Register(NewHotkeyTool()); err != nil {
		return err
	}
	if err := reg.Register(NewMouseMoveTool()); err != nil {
		return err
	}
	if err := reg.Register(NewMouseClickTool()); err != nil {
		return err
	}
	if err := reg.Register(NewMouseScrollTool()); err != nil {
		return err
	}
	return nil
}
