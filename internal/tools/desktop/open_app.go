package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

type OpenAppTool struct {
	DB *sql.DB
}

func NewOpenAppTool(db *sql.DB) *OpenAppTool {
	return &OpenAppTool{DB: db}
}

func (t *OpenAppTool) Name() string { return "desktop.open_app" }
func (t *OpenAppTool) Description() string {
	return "Abre una aplicación o programa instalado en el escritorio del sistema (ej: firefox, brave, chrome, code, spotify, etc.)."
}
func (t *OpenAppTool) Category() string  { return "desktop" }
func (t *OpenAppTool) RiskLevel() string { return "low" }
func (t *OpenAppTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"app": map[string]interface{}{
				"type":        "string",
				"description": "Nombre de la aplicación a abrir.",
			},
		},
		"required": []string{"app"},
	}
}

func (t *OpenAppTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	app, _ := args["app"].(string)
	if app == "" {
		return nil, fmt.Errorf("el argumento 'app' es obligatorio")
	}

	appLower := strings.ToLower(strings.TrimSpace(app))
	var command string

	if matchedApp, ok := t.findBestAppMatch(appLower); ok {
		if matchedApp == "navegador" {
			err := desktop.OpenURL("https://google.com")
			if err != nil {
				return nil, err
			}
			return &executor.ToolResult{
				Success:    true,
				Text:       "Navegador predeterminado abierto.",
				StartedAt:  started,
				FinishedAt: time.Now(),
			}, nil
		}

		// Buscar el comando en BD
		err := t.DB.QueryRow("SELECT command FROM app_launchers WHERE executable = ? OR name = ? LIMIT 1", matchedApp, matchedApp).Scan(&command)
		if err != nil {
			command = matchedApp
		}
		app = matchedApp
	} else {
		command = app
	}

	// Validar que el ejecutable existe
	firstWord := strings.Fields(command)[0]
	if _, err := exec.LookPath(firstWord); err != nil {
		if info, statErr := os.Stat(firstWord); statErr != nil || info.IsDir() {
			return nil, fmt.Errorf("no se encontró el programa '%s' en el PATH del sistema", app)
		}
	}

	err := desktop.LaunchApplication(command)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Aplicación '%s' lanzada correctamente.", app),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

func (t *OpenAppTool) findBestAppMatch(query string) (string, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "", false
	}

	// Normalizaciones específicas para VS Code
	if strings.Contains(query, "code") || strings.Contains(query, "vscode") || strings.Contains(query, "visual studio") || strings.Contains(query, "vusaul") {
		var execName string
		err := t.DB.QueryRow("SELECT executable FROM app_launchers WHERE name LIKE '%code%' OR name LIKE '%vscode%' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "code", true
	}

	// Normalización para navegadores
	if strings.Contains(query, "navegador") || strings.Contains(query, "browser") || strings.Contains(query, "internet") || query == "web" {
		return "navegador", true
	}

	// Normalización para gestor de archivos
	if strings.Contains(query, "gestor de archivos") || strings.Contains(query, "explorador de archivos") || query == "archivos" || query == "carpetas" {
		var execName string
		err := t.DB.QueryRow("SELECT executable FROM app_launchers WHERE name = 'files' OR name = 'nautilus' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "nautilus", true
	}

	// Normalización para terminal/consola
	if query == "consola" || query == "terminal" || query == "kitty" || query == "alacritty" || query == "konsole" || strings.Contains(query, "linea de comandos") {
		var execName string
		err := t.DB.QueryRow("SELECT executable FROM app_launchers WHERE name = 'kitty' OR name = 'gnome-terminal' OR name = 'konsole' OR name = 'terminal' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "kitty", true
	}

	rows, err := t.DB.Query("SELECT name, display_name, executable FROM app_launchers WHERE is_available = 1")
	if err != nil {
		return "", false
	}
	defer rows.Close()

	stopwords := map[string]bool{
		"el": true, "la": true, "los": true, "las": true,
		"un": true, "una": true, "unos": true, "unas": true,
		"de": true, "del": true, "al": true,
		"y": true, "o": true, "e": true, "u": true,
		"en": true, "es": true, "con": true, "por": true, "para": true,
		"se": true, "me": true, "te": true, "le": true, "lo": true,
		"nos": true, "les": true, "os": true,
		"que": true, "qué": true, "como": true, "cómo": true,
		"si": true, "sí": true, "no": true,
		"mi": true, "mis": true, "su": true, "sus": true, "tu": true, "tus": true,
		"este": true, "esta": true, "esto": true, "estos": true, "estas": true,
		"ser": true, "estar": true, "hacer": true, "ir": true,
		"a": true, "an": true, "the": true, "and": true, "or": true, "is": true, "are": true, "it": true,
	}

	var bestMatch string
	var maxScore int = 0
	queryTokens := strings.Fields(query)

	for rows.Next() {
		var name, displayName, executable string
		if err := rows.Scan(&name, &displayName, &executable); err != nil {
			continue
		}

		nameLower := strings.ToLower(name)
		displayLower := strings.ToLower(displayName)
		execLower := strings.ToLower(executable)

		if nameLower == query || execLower == query {
			return executable, true
		}

		score := 0
		for _, token := range queryTokens {
			if token == "" || len(token) < 2 {
				continue
			}
			if stopwords[token] {
				continue
			}

			// Para tokens cortos (longitud <= 3), evitar coincidencia de subcadenas en medio de palabras
			if len(token) <= 3 {
				baseExec := execLower
				if idx := strings.LastIndex(execLower, "/"); idx != -1 {
					baseExec = execLower[idx+1:]
				}
				if nameLower == token || strings.HasPrefix(nameLower, token) {
					score += 3
				}
				if displayLower == token || strings.HasPrefix(displayLower, token) {
					score += 2
				}
				if baseExec == token || strings.HasPrefix(baseExec, token) {
					score += 3
				}
			} else {
				if strings.Contains(nameLower, token) {
					score += 3
				}
				if strings.Contains(displayLower, token) {
					score += 2
				}
				if strings.Contains(execLower, token) {
					score += 3
				}
			}
		}

		// Solo aplicar coincidencia completa de consulta si la consulta entera no es un stopword y no es extremadamente corta
		if !stopwords[query] && len(query) > 3 {
			if strings.Contains(displayLower, query) || strings.Contains(query, displayLower) {
				score += 5
			}
			if strings.Contains(nameLower, query) || strings.Contains(query, nameLower) {
				score += 5
			}
		}

		if score > maxScore {
			maxScore = score
			bestMatch = executable
		}
	}

	if maxScore >= 5 {
		return bestMatch, true
	}

	return "", false
}
