package security

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsPathBlocked comprueba si una ruta dada está dentro de la lista de rutas bloqueadas por seguridad.
func IsPathBlocked(path string, blockedPaths []string) bool {
	if path == "" {
		return false
	}
	
	// Expandir tildes de home si aplica
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			cleanPath = filepath.Join(home, cleanPath[1:])
		}
	}

	for _, blocked := range blockedPaths {
		cleanBlocked := filepath.Clean(blocked)
		if strings.HasPrefix(cleanBlocked, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				cleanBlocked = filepath.Join(home, cleanBlocked[1:])
			}
		}

		if strings.HasPrefix(cleanPath, cleanBlocked) {
			return true
		}

		// Soporte básico de wildcards de extensión (ej: **/.env o **/.env.local)
		if strings.HasPrefix(blocked, "**/") {
			suffix := strings.TrimPrefix(blocked, "**/")
			if strings.HasSuffix(cleanPath, suffix) || filepath.Base(cleanPath) == suffix {
				return true
			}
		}
	}
	return false
}

// ValidateToolAction comprueba si una acción/herramienta está autorizada y si requiere confirmación.
func ValidateToolAction(db *sql.DB, toolName string, targetPath string, blockedPaths []string) (allowed bool, requiresConfirm bool, reason string) {
	// 1. Verificar si la ruta física del archivo está bloqueada
	if targetPath != "" && IsPathBlocked(targetPath, blockedPaths) {
		return false, false, fmt.Sprintf("acceso denegado por política de seguridad: la ruta '%s' está bloqueada", targetPath)
	}

	// Forzar confirmación interactiva para herramientas intrínsecamente destructivas
	if toolName == "files.delete_file" {
		return true, true, "la eliminación de archivos es una acción destructiva"
	}

	// 2. Verificar permisos de la herramienta en base de datos
	var enabled int
	var reqConfirm int
	var riskLevel string

	// Consultar si es herramienta interna
	query := "SELECT enabled, requires_confirmation, risk_level FROM internal_tools WHERE name = ?"
	err := db.QueryRow(query, toolName).Scan(&enabled, &reqConfirm, &riskLevel)
	if err != nil {
		// Si no existe, intentar buscar en herramientas MCP
		queryMCP := "SELECT enabled, requires_confirmation, risk_level FROM mcp_tools WHERE name = ?"
		err = db.QueryRow(queryMCP, toolName).Scan(&enabled, &reqConfirm, &riskLevel)
	}

	// Si no está en BD (es nueva o temporal), por defecto se habilita si no es crítica
	if err != nil {
		// Nivel por defecto: permitido, sin confirmación
		return true, false, ""
	}

	if enabled == 0 {
		return false, false, fmt.Sprintf("la herramienta '%s' está deshabilitada", toolName)
	}

	if riskLevel == "forbidden" {
		return false, false, fmt.Sprintf("la herramienta '%s' está prohibida por la política del sistema", toolName)
	}

	// Si está explícitamente marcado para confirmación, o el riesgo es alto/destructivo
	if reqConfirm == 1 || riskLevel == "high" || riskLevel == "destructive" {
		return true, true, fmt.Sprintf("la herramienta '%s' (riesgo: %s) requiere confirmación explícita del usuario", toolName, riskLevel)
	}

	return true, false, ""
}

// IsCommandCritical comprueba si un comando de shell tiene potencial de causar daños o elevar privilegios.
func IsCommandCritical(command string) bool {
	cmdLower := strings.ToLower(command)
	criticalKeywords := []string{
		"rm ", "mv ", "dd ", "sudo ", "mkfs", "shutdown", "reboot", "poweroff",
		"kill", "pkill", "systemctl stop", "systemctl disable",
		"chmod 777", "chown", ">", ">>", " |", "| ",
	}
	for _, kw := range criticalKeywords {
		if strings.Contains(cmdLower, kw) {
			return true
		}
	}
	return false
}
