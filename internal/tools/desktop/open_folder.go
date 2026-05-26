package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
	filesTool "rbot/internal/tools/files"
)

type OpenFolderTool struct {
	DB           *sql.DB
	AllowedRoots []string
	BlockedPaths []string
}

func NewOpenFolderTool(db *sql.DB, allowedRoots []string, blockedPaths []string) *OpenFolderTool {
	return &OpenFolderTool{
		DB:           db,
		AllowedRoots: allowedRoots,
		BlockedPaths: blockedPaths,
	}
}

func (t *OpenFolderTool) Name() string { return "desktop.open_folder" }
func (t *OpenFolderTool) Description() string {
	return "Abre una carpeta/directorio del sistema en Visual Studio Code o en el explorador de archivos Nautilus."
}
func (t *OpenFolderTool) Category() string  { return "desktop" }
func (t *OpenFolderTool) RiskLevel() string { return "low" }
func (t *OpenFolderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Ruta de la carpeta (ej: Descargas, Documentos, o ruta absoluta) a abrir.",
			},
			"app": map[string]interface{}{
				"type":        "string",
				"description": "Aplicación con la que abrir la carpeta: 'vscode' (para Visual Studio Code) o 'nautilus' (para explorador de archivos).",
				"enum":        []string{"vscode", "code", "nautilus"},
			},
		},
		"required": []string{"path"},
	}
}

func (t *OpenFolderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	pathVal, _ := args["path"].(string)
	app, _ := args["app"].(string)
	if pathVal == "" {
		return nil, fmt.Errorf("el argumento 'path' es obligatorio")
	}

	resolvedPath, err := filesTool.ResolvePathForReading(pathVal, t.DB, t.AllowedRoots, t.BlockedPaths)
	if err != nil {
		return nil, err
	}

	appLower := strings.ToLower(strings.TrimSpace(app))
	var msg string
	if appLower == "vscode" || appLower == "code" {
		err = desktop.LaunchApplication("code " + resolvedPath)
		if err != nil {
			return nil, err
		}
		msg = fmt.Sprintf("Carpeta '%s' abierta en VS Code.", resolvedPath)
	} else {
		err = desktop.LaunchApplication("nautilus " + resolvedPath)
		if err != nil {
			return nil, err
		}
		msg = fmt.Sprintf("Carpeta '%s' abierta en el explorador de archivos.", resolvedPath)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       msg,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
