package system

import (
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete system en el Registry.
func RegisterTools(reg *executor.Registry) error {
	if err := reg.Register(NewNotifyTool()); err != nil {
		return err
	}
	if err := reg.Register(NewRunCommandSafeTool()); err != nil {
		return err
	}
	if err := reg.Register(NewDateTimeTool()); err != nil {
		return err
	}
	if err := reg.Register(NewClipboardCopyTool()); err != nil {
		return err
	}
	if err := reg.Register(NewRunCommandTool()); err != nil {
		return err
	}
	return nil
}
