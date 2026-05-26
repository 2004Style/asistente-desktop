package files

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rbot/internal/files"
	"rbot/internal/security"
)

// ResolvePathForReading resuelve de manera segura una ruta física para leer o listar.
func ResolvePathForReading(query string, db *sql.DB, allowedRoots []string, blockedPaths []string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("ruta o consulta vacía")
	}

	var targetPath string
	// Expandir tilde ~
	if strings.HasPrefix(query, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			targetPath = filepath.Join(home, query[1:])
		} else {
			targetPath = query
		}
	} else if filepath.IsAbs(query) {
		targetPath = query
	} else {
		// Comprobar si existe en el CWD actual
		cwd, err := os.Getwd()
		if err == nil {
			local := filepath.Join(cwd, query)
			if _, err := os.Stat(local); err == nil {
				targetPath = local
			}
		}
	}

	// Si no se encontró físicamente de forma directa, buscar a través de base de datos / búsqueda recursiva
	if targetPath == "" {
		found, err := files.FindFileOrDirectory(db, query, allowedRoots, blockedPaths)
		if err != nil {
			return "", err
		}
		targetPath = found
	}

	targetPath = filepath.Clean(targetPath)

	// Validar que la ruta final no esté bloqueada por seguridad
	if security.IsPathBlocked(targetPath, blockedPaths) {
		return "", fmt.Errorf("acceso denegado por política de seguridad: la ruta '%s' está bloqueada", targetPath)
	}

	return targetPath, nil
}

// ResolvePathForCreation resuelve una ruta de destino para crear nuevos archivos o directorios de forma segura.
func ResolvePathForCreation(query string, blockedPaths []string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("ruta vacía")
	}

	var targetPath string
	// Expandir tilde ~
	if strings.HasPrefix(query, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			targetPath = filepath.Join(home, query[1:])
		} else {
			targetPath = query
		}
	} else if filepath.IsAbs(query) {
		targetPath = query
	} else {
		cleaned := filepath.Clean(query)
		parts := strings.Split(cleaned, string(filepath.Separator))
		// Si es solo un nombre simple de archivo sin subcarpetas, colocarlo en Descargas
		if len(parts) == 1 {
			home, err := os.UserHomeDir()
			if err == nil && home != "" {
				targetPath = filepath.Join(home, "Descargas", cleaned)
			} else {
				targetPath = cleaned
			}
		} else {
			// Si contiene subcarpetas relativas, resolver a partir de CWD
			cwd, err := os.Getwd()
			if err == nil {
				targetPath = filepath.Join(cwd, cleaned)
			} else {
				targetPath = cleaned
			}
		}
	}

	targetPath = filepath.Clean(targetPath)

	// Validar que la ruta de destino no esté bloqueada
	if security.IsPathBlocked(targetPath, blockedPaths) {
		return "", fmt.Errorf("acceso denegado por política de seguridad: la ruta de destino '%s' está bloqueada", targetPath)
	}

	return targetPath, nil
}
