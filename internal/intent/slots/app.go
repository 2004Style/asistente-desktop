package slots

import (
	"strings"
)

// ExtractApp extrae el nombre de la aplicación a partir de verbos de acción o nombres comunes.
func ExtractApp(input string) string {
	inputLower := strings.ToLower(input)
	words := strings.Fields(inputLower)
	triggers := []string{"abre", "lanzar", "lanza", "ejecuta", "inicia", "cierra", "matar", "mata", "termina"}

	for i, w := range words {
		for _, t := range triggers {
			if w == t && i+1 < len(words) {
				return words[i+1]
			}
		}
	}

	commonApps := []string{"brave", "firefox", "chrome", "code", "spotify", "discord", "kitty", "nautilus", "thunar"}
	for _, app := range commonApps {
		if strings.Contains(inputLower, app) {
			return app
		}
	}
	return ""
}
