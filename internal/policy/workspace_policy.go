package policy

import (
	"strings"
)

// WorkspacePolicy representa las políticas locales de seguridad definidas en el workspace.
type WorkspacePolicy struct {
	Rules []string
}

// NewWorkspacePolicy parsea las reglas del contenido Markdown de POLICIES.md.
func NewWorkspacePolicy(content string) *WorkspacePolicy {
	var rules []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			rule := strings.TrimSpace(strings.TrimLeft(line, "-*"))
			if rule != "" {
				rules = append(rules, rule)
			}
		}
	}
	return &WorkspacePolicy{Rules: rules}
}

// Evaluate evalúa si una herramienta y sus argumentos cumplen con las políticas del workspace.
func (wp *WorkspacePolicy) Evaluate(toolName string, args map[string]interface{}) (allowed bool, requiresConfirm bool, reason string) {
	allowed = true
	requiresConfirm = false

	for _, rule := range wp.Rules {
		ruleLower := strings.ToLower(rule)

		// Reglas que indican que no se debe hacer ALGO "sin confirmación" (o similar)
		if strings.Contains(ruleLower, "sin confirmación") || strings.Contains(ruleLower, "sin confirmacion") || strings.Contains(ruleLower, "sin confirmar") {
			if matchesToolOrContext(toolName, args, ruleLower) {
				requiresConfirm = true
				reason = "confirmación requerida por política local: " + rule
				continue
			}
		}

		// Reglas prohibitivas ("no ejecutar", "bloquear", "prohibir")
		if strings.Contains(ruleLower, "no ejecutar") || strings.Contains(ruleLower, "bloquear") || strings.Contains(ruleLower, "prohibir") || strings.Contains(ruleLower, "no usar") {
			if matchesToolOrContext(toolName, args, ruleLower) {
				return false, false, "bloqueado por política local: " + rule
			}
		}
		// Reglas de confirmación ("confirmar", "pedir confirmación")
		if strings.Contains(ruleLower, "confirmar") || strings.Contains(ruleLower, "requerir confirmacion") || strings.Contains(ruleLower, "pedir confirmacion") {
			if matchesToolOrContext(toolName, args, ruleLower) {
				requiresConfirm = true
				reason = "confirmación requerida por política local: " + rule
			}
		}
	}

	return allowed, requiresConfirm, reason
}

func matchesToolOrContext(toolName string, args map[string]interface{}, ruleLower string) bool {
	// Comandos de consola
	if strings.Contains(toolName, "run_command") && (strings.Contains(ruleLower, "comando") || strings.Contains(ruleLower, "shell") || strings.Contains(ruleLower, "consola")) {
		return true
	}
	// Cerrar ventanas/aplicaciones
	if (strings.Contains(toolName, "close_window") || strings.Contains(toolName, "kill_app")) && (strings.Contains(ruleLower, "cerrar ventana") || strings.Contains(ruleLower, "cerrar ventanas") || strings.Contains(ruleLower, "terminar app")) {
		return true
	}
	// Clics e inputs
	if (strings.Contains(toolName, "click") || strings.Contains(toolName, "mouse")) && (strings.Contains(ruleLower, "clic") || strings.Contains(ruleLower, "click")) {
		return true
	}
	if (strings.Contains(toolName, "write") || strings.Contains(toolName, "type")) && (strings.Contains(ruleLower, "escribir") || strings.Contains(ruleLower, "escribir texto")) {
		return true
	}
	return false
}
