package slots

import (
	"regexp"
	"strings"
)

// ExtractFileSlots extrae slots relacionados con archivos y carpetas.
// Retorna (path, fileName, folderName)
func ExtractFileSlots(input string) (string, string, string) {
	inputLower := strings.ToLower(input)

	var path, fileName, folderName string

	// Expresiones regulares para capturar archivos y carpetas
	// Ejemplos: "archivo README.md", "carpeta build", "directorio src/main"
	fileRegex := regexp.MustCompile(`(?:archivo|fichero)\s+([^\s,]+)`)
	folderRegex := regexp.MustCompile(`(?:carpeta|directorio|folder)\s+([^\s,]+)`)

	if matches := fileRegex.FindStringSubmatch(inputLower); len(matches) > 1 {
		fileName = matches[1]
		path = fileName
	}

	if matches := folderRegex.FindStringSubmatch(inputLower); len(matches) > 1 {
		folderName = matches[1]
		if path == "" {
			path = folderName
		}
	}

	// Heurística alternativa: palabras con extensiones o rutas
	if fileName == "" || folderName == "" {
		words := strings.Fields(inputLower)
		for _, w := range words {
			// Limpiar puntuación al final
			wClean := strings.Trim(w, ".,!?;:")
			if strings.Contains(wClean, ".") && len(wClean) > 2 {
				// Probablemente un nombre de archivo
				if fileName == "" {
					fileName = wClean
					if path == "" {
						path = wClean
					}
				}
			}
			if (strings.Contains(wClean, "/") || strings.Contains(wClean, "\\")) && len(wClean) > 1 {
				if path == "" {
					path = wClean
				}
			}
		}
	}

	// Restaurar el caso original si se encuentra la subcadena
	if path != "" {
		idx := strings.Index(strings.ToLower(input), path)
		if idx != -1 {
			path = input[idx : idx+len(path)]
		}
	}
	if fileName != "" {
		idx := strings.Index(strings.ToLower(input), fileName)
		if idx != -1 {
			fileName = input[idx : idx+len(fileName)]
		}
	}
	if folderName != "" {
		idx := strings.Index(strings.ToLower(input), folderName)
		if idx != -1 {
			folderName = input[idx : idx+len(folderName)]
		}
	}

	return path, fileName, folderName
}
