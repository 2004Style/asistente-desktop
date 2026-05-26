package workspace

import (
	"fmt"
	"strings"
)

type ContextBuilder struct{}

func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

func (cb *ContextBuilder) Build(userInput string, wCtx *WorkspaceContext) string {
	if wCtx == nil {
		return ""
	}

	var parts []string

	// 1. Identidad siempre (breve, primeras líneas de IDENTITY.md)
	identityBrief := ""
	if wCtx.Identity != "" {
		lines := strings.Split(wCtx.Identity, "\n")
		var relevant []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			relevant = append(relevant, line)
			if len(relevant) >= 5 {
				break
			}
		}
		if len(relevant) > 0 {
			identityBrief = "[IDENTIDAD Y TONO]\n" + strings.Join(relevant, "\n")
		}
	}
	if identityBrief != "" {
		parts = append(parts, identityBrief)
	}

	// 2. Reglas críticas siempre (breves, primeras líneas de AGENTS.md)
	agentBrief := ""
	if wCtx.AgentRules != "" {
		lines := strings.Split(wCtx.AgentRules, "\n")
		var relevant []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			relevant = append(relevant, line)
			if len(relevant) >= 5 {
				break
			}
		}
		if len(relevant) > 0 {
			agentBrief = "[REGLAS DE COMPORTAMIENTO]\n" + strings.Join(relevant, "\n")
		}
	}
	if agentBrief != "" {
		parts = append(parts, agentBrief)
	}

	inputLower := strings.ToLower(userInput)

	// 3. Políticas locales: inyectar si el input se relaciona con políticas/seguridad/permisos
	if wCtx.Policies != "" {
		if strings.Contains(inputLower, "polític") || strings.Contains(inputLower, "permis") || strings.Contains(inputLower, "segurid") || strings.Contains(inputLower, "cerrar") || strings.Contains(inputLower, "login") || strings.Contains(inputLower, "password") {
			parts = append(parts, "[POLÍTICAS LOCALES ADICIONALES]\n"+wCtx.Policies)
		}
	}

	// 4. Memoria: inyectar si se relaciona con datos personales o preferencias
	if wCtx.Memory != "" {
		if strings.Contains(inputLower, "sobre mí") || strings.Contains(inputLower, "quien soy") || strings.Contains(inputLower, "quién soy") || strings.Contains(inputLower, "prefer") || strings.Contains(inputLower, "gust") || strings.Contains(inputLower, "desarroll") || strings.Contains(inputLower, "lenguaje") {
			parts = append(parts, "[PREFERENCIAS DEL USUARIO]\n"+wCtx.Memory)
		}
	}

	// 5. Tareas: inyectar si se relaciona con TODO/tareas/pendientes
	if wCtx.Tasks != "" {
		if strings.Contains(inputLower, "tarea") || strings.Contains(inputLower, "pendiente") || strings.Contains(inputLower, "todo") || strings.Contains(inputLower, "hacer") {
			parts = append(parts, "[TAREAS LOCALES]\n"+wCtx.Tasks)
		}
	}

	// 6. Shortcuts: inyectar si coincide con triggers de macros
	var activeShortcuts []string
	for _, s := range wCtx.Shortcuts {
		matched := false
		for _, trig := range s.Triggers {
			if strings.Contains(inputLower, strings.ToLower(trig)) {
				matched = true
				break
			}
		}
		if matched {
			activeShortcuts = append(activeShortcuts, fmt.Sprintf("- Macro '%s': %s", s.Name, s.Description))
		}
	}
	if len(activeShortcuts) > 0 {
		parts = append(parts, "[MACROS / ATADOS DISPONIBLES DETECTADOS]\n"+strings.Join(activeShortcuts, "\n"))
	}

	return strings.Join(parts, "\n\n")
}
