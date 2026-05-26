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

// ReadFileTool lee el contenido de un archivo o lista un directorio si es tal.
type ReadFileTool struct {
	DB           *sql.DB
	AllowedRoots []string
	BlockedPaths []string
}

func NewReadFileTool(db *sql.DB, allowedRoots []string, blockedPaths []string) *ReadFileTool {
	return &ReadFileTool{
		DB:           db,
		AllowedRoots: allowedRoots,
		BlockedPaths: blockedPaths,
	}
}

func (t *ReadFileTool) Name() string { return "files.read_file" }
func (t *ReadFileTool) Description() string {
	return "Lee y devuelve el contenido de un archivo de texto en el sistema, o lista sus directorios hijos si la ruta es una carpeta."
}
func (t *ReadFileTool) Category() string  { return "files" }
func (t *ReadFileTool) RiskLevel() string { return "low" }
func (t *ReadFileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta física absoluta, relativa o nombre del archivo/directorio a leer (ej: ~/Documentos/notas.txt, README.md).",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathArg, _ := args["path"].(string)
	if pathArg == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := ResolvePathForReading(pathArg, t.DB, t.AllowedRoots, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, err
	}

	// Si es directorio, listar contenido
	if info.IsDir() {
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

	// Si es archivo, leer
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, err
	}

	text := string(content)
	if len(text) > 4000 {
		text = text[:4000] + "\n... (contenido truncado por longitud)"
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
