package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"rbot/internal/executor"
)

type CloseAppTool struct {
	DB *sql.DB
}

func NewCloseAppTool(db *sql.DB) *CloseAppTool {
	return &CloseAppTool{DB: db}
}

func (t *CloseAppTool) Name() string { return "desktop.close_app" }
func (t *CloseAppTool) Description() string {
	return "Cierra o termina una aplicación o programa en ejecución (ej: firefox, chrome, code, nautilus, spotify, etc.)."
}
func (t *CloseAppTool) Category() string  { return "desktop" }
func (t *CloseAppTool) RiskLevel() string { return "medium" } // El cierre de apps puede considerarse de riesgo medio
func (t *CloseAppTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"app": map[string]interface{}{
				"type":        "string",
				"description": "Nombre de la aplicación a cerrar.",
			},
		},
		"required": []string{"app"},
	}
}

func (t *CloseAppTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	app, _ := args["app"].(string)
	if app == "" {
		return nil, fmt.Errorf("el argumento 'app' es obligatorio")
	}

	appLower := strings.ToLower(strings.TrimSpace(app))

	// Reutilizar la lógica de coincidencia inteligente si está disponible
	matchedApp := appLower
	// Intentar obtener del helper de open_app.go si se comparte
	// Para evitar problemas de visibilidad, podemos implementar una pequeña búsqueda directa o usar la compartida
	
	// Cierres específicos
	if appLower == "navegador" || appLower == "chrome" || appLower == "google-chrome" || appLower == "firefox" || appLower == "brave" {
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:google-chrome").Run()
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:firefox").Run()
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:brave").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "chrome").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "google-chrome").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "firefox").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "brave").Run()
		return &executor.ToolResult{
			Success:    true,
			Text:       "Navegador web cerrado.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	if appLower == "vscode" || appLower == "code" || appLower == "visual studio code" {
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:Code").Run()
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:code").Run()
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:code-oss").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "code").Run()
		return &executor.ToolResult{
			Success:    true,
			Text:       "Visual Studio Code cerrado.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	if appLower == "nautilus" || appLower == "gestor de archivos" || appLower == "archivos" {
		_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:nautilus").Run()
		_ = exec.CommandContext(ctx, "pkill", "-x", "nautilus").Run()
		return &executor.ToolResult{
			Success:    true,
			Text:       "Gestor de archivos cerrado.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	// Intento genérico
	_ = exec.CommandContext(ctx, "hyprctl", "dispatch", "closewindow", "class:"+matchedApp).Run()
	_ = exec.CommandContext(ctx, "pkill", "-x", matchedApp).Run()
	_ = exec.CommandContext(ctx, "pkill", "-f", matchedApp).Run()

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Aplicación '%s' cerrada.", app),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
