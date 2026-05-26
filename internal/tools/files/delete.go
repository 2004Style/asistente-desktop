package files

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"rbot/internal/executor"
)

// DeleteFileTool elimina un archivo o directorio.
type DeleteFileTool struct {
	DB           *sql.DB
	AllowedRoots []string
	BlockedPaths []string
}

func NewDeleteFileTool(db *sql.DB, allowedRoots []string, blockedPaths []string) *DeleteFileTool {
	return &DeleteFileTool{
		DB:           db,
		AllowedRoots: allowedRoots,
		BlockedPaths: blockedPaths,
	}
}

func (t *DeleteFileTool) Name() string { return "files.delete_file" }
func (t *DeleteFileTool) Description() string {
	return "Elimina de forma permanente un archivo o directorio del sistema."
}
func (t *DeleteFileTool) Category() string  { return "files" }
func (t *DeleteFileTool) RiskLevel() string { return "high" }
func (t *DeleteFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta física absoluta, relativa o nombre del archivo o carpeta a eliminar.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *DeleteFileTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathVal, _ := args["path"].(string)
	if pathVal == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := ResolvePathForReading(pathVal, t.DB, t.AllowedRoots, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	err = os.RemoveAll(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("error al eliminar: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Archivo o directorio '%s' eliminado correctamente del sistema.", resolvedPath),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
