package files

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"rbot/internal/executor"
)

// ListDirectoryTool lista el contenido de un directorio.
type ListDirectoryTool struct {
	DB           *sql.DB
	AllowedRoots []string
	BlockedPaths []string
}

func NewListDirectoryTool(db *sql.DB, allowedRoots []string, blockedPaths []string) *ListDirectoryTool {
	return &ListDirectoryTool{
		DB:           db,
		AllowedRoots: allowedRoots,
		BlockedPaths: blockedPaths,
	}
}

func (t *ListDirectoryTool) Name() string { return "files.list_directory" }
func (t *ListDirectoryTool) Description() string {
	return "Lista el contenido detallado de un directorio o carpeta en el sistema."
}
func (t *ListDirectoryTool) Category() string  { return "files" }
func (t *ListDirectoryTool) RiskLevel() string { return "low" }
func (t *ListDirectoryTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta física absoluta, relativa o nombre del directorio a listar.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirectoryTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathVal, _ := args["path"].(string)
	if pathVal == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := ResolvePathForReading(pathVal, t.DB, t.AllowedRoots, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("la ruta '%s' no es un directorio", resolvedPath)
	}

	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("error al leer el directorio: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directorio: %s\nContenido:\n", resolvedPath))
	for _, entry := range entries {
		entryType := "Archivo"
		if entry.IsDir() {
			entryType = "Carpeta"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", entryType, entry.Name()))
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       sb.String(),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
