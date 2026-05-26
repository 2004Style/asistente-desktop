package files

import (
	"context"
	"fmt"
	"os"
	"time"

	"rbot/internal/executor"
)

// CreateDirectoryTool crea un nuevo directorio.
type CreateDirectoryTool struct {
	BlockedPaths []string
}

func NewCreateDirectoryTool(blockedPaths []string) *CreateDirectoryTool {
	return &CreateDirectoryTool{
		BlockedPaths: blockedPaths,
	}
}

func (t *CreateDirectoryTool) Name() string { return "files.create_directory" }
func (t *CreateDirectoryTool) Description() string {
	return "Crea una nueva carpeta o directorio en la ruta especificada."
}
func (t *CreateDirectoryTool) Category() string  { return "files" }
func (t *CreateDirectoryTool) RiskLevel() string { return "medium" }
func (t *CreateDirectoryTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta física o nombre del directorio a crear.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *CreateDirectoryTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathVal, _ := args["path"].(string)
	if pathVal == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := ResolvePathForCreation(pathVal, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(resolvedPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("error al crear directorio: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Directorio '%s' creado correctamente.", resolvedPath),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
