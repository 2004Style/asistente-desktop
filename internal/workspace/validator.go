package workspace

import (
	"fmt"
	"strings"
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidatePolicies(policiesContent string) error {
	lines := strings.Split(policiesContent, "\n")
	disallowedKeywords := []string{"rm ", "remove", "delete", "borrar", ".ssh", ".env", "sudo", "keyring"}
	allowVerbs := []string{"permitir", "allow", "skip", "no confirmar", "omitir", "desactivar", "bypass", "sin confirmacion"}

	for _, line := range lines {
		lineLower := strings.ToLower(line)
		for _, kw := range disallowedKeywords {
			if strings.Contains(lineLower, kw) {
				for _, verb := range allowVerbs {
					if strings.Contains(lineLower, verb) {
						return fmt.Errorf("política inválida: no se permite relajar reglas críticas de seguridad (%s)", strings.TrimSpace(line))
					}
				}
			}
		}
	}
	return nil
}

func (v *Validator) ValidateShortcuts(shortcuts []Shortcut) error {
	for _, s := range shortcuts {
		if s.Name == "" {
			return fmt.Errorf("macro inválida: el nombre del shortcut es requerido")
		}
		if len(s.Triggers) == 0 {
			return fmt.Errorf("macro '%s' inválida: debe tener al menos un trigger", s.Name)
		}
		if len(s.Steps) == 0 {
			return fmt.Errorf("macro '%s' inválida: debe tener al menos un paso de acción", s.Name)
		}
		// Validar cada paso
		for _, step := range s.Steps {
			if step.Intent == "" {
				return fmt.Errorf("macro '%s' inválida: el paso de acción no tiene intención declarada", s.Name)
			}
			// Evitar intents destructivos con argumentos maliciosos en shortcuts
			intentLower := strings.ToLower(step.Intent)
			if strings.Contains(intentLower, "delete") || strings.Contains(intentLower, "remove") || strings.Contains(intentLower, "shell") || strings.Contains(intentLower, "run_command") {
				for _, val := range step.Args {
					valStr, ok := val.(string)
					if ok {
						valLower := strings.ToLower(valStr)
						if strings.Contains(valLower, "rm ") || strings.Contains(valLower, "sudo") || strings.Contains(valLower, ".ssh") || strings.Contains(valLower, ".env") {
							return fmt.Errorf("macro '%s' inválida: contiene argumentos destructivos o prohibidos en el paso '%s'", s.Name, step.Intent)
						}
					}
				}
			}
		}
	}
	return nil
}
