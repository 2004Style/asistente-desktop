package files

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"rbot/internal/executor"
	"rbot/internal/files"
)

// SearchIndexTool busca un archivo o directorio en la base de datos indexada FTS5.
type SearchIndexTool struct {
	DB           *sql.DB
	AllowedRoots []string
	BlockedPaths []string
}

func NewSearchIndexTool(db *sql.DB, allowedRoots []string, blockedPaths []string) *SearchIndexTool {
	return &SearchIndexTool{
		DB:           db,
		AllowedRoots: allowedRoots,
		BlockedPaths: blockedPaths,
	}
}

func (t *SearchIndexTool) Name() string { return "files.search_index" }
func (t *SearchIndexTool) Description() string {
	return "Busca la ruta de un archivo o directorio en el sistema usando aliases, índice FTS5 o búsqueda recursiva en raíces permitidas."
}
func (t *SearchIndexTool) Category() string  { return "files" }
func (t *SearchIndexTool) RiskLevel() string { return "low" }
func (t *SearchIndexTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Nombre o término del archivo/directorio a buscar (ej: notas.txt, rbot.yaml).",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchIndexTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("el argumento 'query' es obligatorio")
	}

	path, err := files.FindFileOrDirectory(t.DB, query, t.AllowedRoots, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Encontrado en: %s", path),
		Data:       map[string]interface{}{"path": path},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
