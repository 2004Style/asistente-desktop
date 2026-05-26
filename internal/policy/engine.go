package policy

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"rbot/internal/executor"
	"rbot/internal/security"
)

// Engine representa el motor de políticas de seguridad encargado de validar permisos y riesgos de ejecución.
type Engine struct {
	mu              sync.Mutex
	pending         map[string]*PendingConfirmation
	BlockedPaths    []string
	ConfirmHighRisk bool
	localPolicy     *WorkspacePolicy
	DB              *sql.DB
}

// NewEngine inicializa un nuevo motor de políticas.
func NewEngine(blockedPaths []string, confirmHighRisk bool) *Engine {
	return &Engine{
		pending:         make(map[string]*PendingConfirmation),
		BlockedPaths:    blockedPaths,
		ConfirmHighRisk: confirmHighRisk,
	}
}

// SetDB asigna la DB para que el engine pueda consultar permisos de herramienta almacenados.
func (e *Engine) SetDB(db *sql.DB) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.DB = db
}

// SetWorkspacePolicies actualiza las políticas de usuario del workspace.
func (e *Engine) SetWorkspacePolicies(content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.localPolicy = NewWorkspacePolicy(content)
}

// EvaluateTool analiza si la herramienta y los argumentos cumplen con las políticas locales de seguridad.
func (e *Engine) EvaluateTool(ctx context.Context, tool executor.ToolHandler, args map[string]interface{}) executor.PolicyDecision {
	toolName := tool.Name()
	riskLevel := tool.RiskLevel()

	// 1. Validar contra reglas inmutables del sistema
	if immBlocked, reason := IsImmutablyBlocked(toolName, args); immBlocked {
		return executor.PolicyDecision{
			Allowed:   false,
			RiskLevel: "critical",
			Reason:    reason,
		}
	}

	// 2. Validar accesos a archivos contra rutas bloqueadas por seguridad
	for _, argVal := range args {
		if pathStr, ok := argVal.(string); ok {
			// Comprobar si parece una ruta absoluta o relativa con tilde
			if strings.HasPrefix(pathStr, "/") || strings.HasPrefix(pathStr, "~") || strings.Contains(pathStr, ".") {
				if security.IsPathBlocked(pathStr, e.BlockedPaths) {
					return executor.PolicyDecision{
						Allowed:   false,
						RiskLevel: "critical",
						Reason:    fmt.Sprintf("acceso denegado por política: la ruta '%s' está bloqueada por seguridad", pathStr),
					}
				}
			}
		}
	}

	// 2b. Si hay una DB de herramientas, respetar la configuración almacenada en la BD
	if e.DB != nil {
		// intentar extraer una ruta objetivo si está en args
		var targetPath string
		for _, v := range args {
			if p, ok := v.(string); ok {
				// heurística: si contiene '/' o '~' o '.' tratar como path
				if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || strings.Contains(p, ".") {
					targetPath = p
					break
				}
			}
		}
		allowedDB, reqConfirmDB, reasonDB := security.ValidateToolAction(e.DB, toolName, targetPath, e.BlockedPaths)
		if !allowedDB {
			return executor.PolicyDecision{Allowed: false, RiskLevel: "critical", Reason: reasonDB}
		}
		if reqConfirmDB {
			return executor.PolicyDecision{Allowed: true, RequiresConfirm: true, RiskLevel: riskLevel, Reason: reasonDB}
		}
	}

	// 3. Validación estricta para comandos de consola destructivos o de elevación de privilegios
	if toolName == "system.run_command_safe" {
		command, _ := args["command"].(string)
		if command != "" {
			if security.IsCommandCritical(command) {
				return executor.PolicyDecision{
					Allowed:         false,
					RequiresConfirm: false,
					RiskLevel:       "critical",
					Reason:          fmt.Sprintf("el comando shell '%s' es de riesgo crítico y está bloqueado de forma preventiva", command),
				}
			}
		}
	}

	// 4. Evaluar políticas locales del workspace de usuario
	e.mu.Lock()
	localPolicy := e.localPolicy
	e.mu.Unlock()

	if localPolicy != nil {
		if allowed, reqConfirm, reason := localPolicy.Evaluate(toolName, args); !allowed {
			return executor.PolicyDecision{
				Allowed:   false,
				RiskLevel: "critical",
				Reason:    reason,
			}
		} else if reqConfirm {
			return executor.PolicyDecision{
				Allowed:         true,
				RequiresConfirm: true,
				RiskLevel:       riskLevel,
				Reason:          reason,
			}
		}
	}

	// 5. Evaluar según el nivel de riesgo intrínseco de la herramienta
	switch riskLevel {
	case "critical":
		return executor.PolicyDecision{
			Allowed:         false,
			RequiresConfirm: false,
			RiskLevel:       "critical",
			Reason:          fmt.Sprintf("la herramienta '%s' es de riesgo crítico y está bloqueada de forma preventiva", toolName),
		}

	case "high":
		return executor.PolicyDecision{
			Allowed:         true,
			RequiresConfirm: e.ConfirmHighRisk,
			RiskLevel:       "high",
			Reason:          fmt.Sprintf("la herramienta '%s' es de riesgo alto", toolName),
		}

	case "medium":
		return executor.PolicyDecision{
			Allowed:         true,
			RequiresConfirm: false,
			RiskLevel:       "medium",
			Reason:          "",
		}

	default:
		return executor.PolicyDecision{
			Allowed:         true,
			RequiresConfirm: false,
			RiskLevel:       "low",
			Reason:          "",
		}
	}
}
