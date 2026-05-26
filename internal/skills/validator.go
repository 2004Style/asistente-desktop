package skills

import (
	"fmt"
	"strings"
)

type Validator struct {
	toolExists func(string) bool
}

func NewValidator(toolExists func(string) bool) *Validator {
	return &Validator{
		toolExists: toolExists,
	}
}

func (v *Validator) Validate(meta *SkillMetadata) error {
	if meta.Name == "" {
		return fmt.Errorf("el campo 'name' es obligatorio en la metadata de la skill")
	}

	if meta.Description == "" {
		return fmt.Errorf("el campo 'description' es obligatorio en la metadata de la skill")
	}

	validRisks := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	if !validRisks[strings.ToLower(meta.RiskLevel)] {
		return fmt.Errorf("nivel de riesgo 'risk_level' inválido: %s", meta.RiskLevel)
	}

	validStatuses := map[string]bool{
		"disabled":     true,
		"experimental": true,
		"enabled":      true,
		"trusted":      true,
		"quarantined":  true,
	}
	if !validStatuses[strings.ToLower(meta.Status)] {
		return fmt.Errorf("estado 'status' inválido: %s", meta.Status)
	}

	// Validar que las herramientas declaradas existan (si se configuró el callback de validación)
	if v.toolExists != nil {
		for _, tool := range meta.Tools {
			if !v.toolExists(tool) {
				return fmt.Errorf("la herramienta declarada '%s' no existe en el ToolRegistry", tool)
			}
		}
	}

	// Validar triggers demasiado genéricos
	genericTriggers := map[string]bool{
		"abre":     true,
		"busca":    true,
		"haz":      true,
		"ejecuta":  true,
		"pon":      true,
		"ver":      true,
		"ir":       true,
		"dime":     true,
		"cómo":     true,
		"como":     true,
		"hola":     true,
		"escribe":  true,
		"presiona": true,
		"clic":     true,
	}

	for _, trigger := range meta.VoiceTriggers {
		trimmed := strings.ToLower(strings.TrimSpace(trigger))
		if trimmed == "" {
			return fmt.Errorf("trigger vacío detectado en la skill '%s'", meta.Name)
		}
		if genericTriggers[trimmed] {
			return fmt.Errorf("el trigger de voz '%s' es demasiado genérico y causaría conflictos en el IntentRouter", trigger)
		}
	}

	// Validar consistencia de permisos
	for _, perm := range meta.Permissions {
		if strings.TrimSpace(perm) == "" {
			return fmt.Errorf("permiso vacío declarado en la skill '%s'", meta.Name)
		}
	}

	return nil
}
