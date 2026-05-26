package media

import "rbot/internal/executor"

// RegisterTools registra todas las herramientas del paquete media en el Registry.
func RegisterTools(reg *executor.Registry) error {
	tools := []executor.ToolHandler{
		NewPlayTool(),
		NewPauseTool(),
		NewResumeTool(),
		NewToggleTool(),
		NewNextTool(),
		NewPreviousTool(),
		NewVolumeUpTool(),
		NewVolumeDownTool(),
		NewMuteTool(),
		NewStatusTool(),
	}
	for _, tool := range tools {
		if err := reg.Register(tool); err != nil {
			return err
		}
	}
	return nil
}
