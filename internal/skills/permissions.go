package skills

import (
	"fmt"
	"strings"
)

func ValidatePermissions(declared []string, toolsUsed []string) error {
	for _, tool := range toolsUsed {
		parts := strings.Split(tool, ".")
		category := parts[0]

		matched := false
		for _, dec := range declared {
			dec = strings.ToLower(strings.TrimSpace(dec))
			if dec == strings.ToLower(tool) ||
				dec == category+":*" ||
				dec == category+":all" ||
				strings.HasPrefix(dec, category+":") ||
				dec == "all" || dec == "*" {
				matched = true
				break
			}
		}

		if !matched {
			return fmt.Errorf("permisos insuficientes: la skill utiliza la herramienta '%s' pero no declara permisos compatibles (ej: '%s:*' o '%s')", tool, category, tool)
		}
	}
	return nil
}
