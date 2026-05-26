package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"rbot/internal/executor"
)

// CreateFileTool crea o sobrescribe un archivo con el contenido especificado.
type CreateFileTool struct {
	BlockedPaths []string
}

func NewCreateFileTool(blockedPaths []string) *CreateFileTool {
	return &CreateFileTool{
		BlockedPaths: blockedPaths,
	}
}

func (t *CreateFileTool) Name() string { return "files.create_file" }
func (t *CreateFileTool) Description() string {
	return "Crea un archivo nuevo o sobrescribe uno existente con el contenido de texto proporcionado."
}
func (t *CreateFileTool) Category() string  { return "files" }
func (t *CreateFileTool) RiskLevel() string { return "medium" }
func (t *CreateFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta de destino o nombre del archivo a crear (ej: ~/Documentos/reporte.txt, script.py).",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Contenido de texto a escribir dentro del archivo.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *CreateFileTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathArg, _ := args["path"].(string)
	contentArg, _ := args["content"].(string)
	if pathArg == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := ResolvePathForCreation(pathArg, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(filepath.Dir(resolvedPath), 0755)
	if err != nil {
		return nil, fmt.Errorf("error al crear directorios padres: %v", err)
	}

	err = os.WriteFile(resolvedPath, []byte(contentArg), 0644)
	if err != nil {
		return nil, fmt.Errorf("error al escribir el archivo: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Archivo '%s' creado correctamente con %d bytes.", resolvedPath, len(contentArg)),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
