package files

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FindFileOrDirectory busca un archivo/directorio utilizando alias, base de datos e índices FTS5.
// Si no lo encuentra, realiza una búsqueda física en los roots autorizados.
func FindFileOrDirectory(db *sql.DB, query string, allowedRoots []string, blockedPaths []string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("consulta vacía")
	}

	// 1. Buscar en aliases
	var aliasPath string
	aliasQuery := `
	SELECT p.path FROM path_aliases a
	JOIN path_entries p ON a.path_entry_id = p.id
	WHERE a.alias = ? COLLATE NOCASE;
	`
	err := db.QueryRow(aliasQuery, query).Scan(&aliasPath)
	if err == nil {
		if verifyPath(db, aliasPath) {
			incrementOpenCount(db, aliasPath)
			return aliasPath, nil
		}
	}

	// 2. Buscar en base de datos FTS5 o LIKE
	var dbPath string
	// Intentamos coincidencia exacta primero
	err = db.QueryRow("SELECT path FROM path_entries WHERE name = ? AND exists_now = 1 LIMIT 1", query).Scan(&dbPath)
	if err == nil && verifyPath(db, dbPath) {
		incrementOpenCount(db, dbPath)
		return dbPath, nil
	}

	// Coincidencia parcial usando FTS5
	ftsQuery := `
	SELECT path FROM search_index 
	WHERE title MATCH ? LIMIT 1;
	`
	// FTS5 requiere escapar caracteres especiales, usamos un formato de búsqueda simple
	matchQuery := fmt.Sprintf("\"%s*\"", strings.ReplaceAll(query, "\"", ""))
	err = db.QueryRow(ftsQuery, matchQuery).Scan(&dbPath)
	if err == nil && verifyPath(db, dbPath) {
		incrementOpenCount(db, dbPath)
		return dbPath, nil
	}

	// Fallback de LIKE
	likeQuery := `
	SELECT path FROM path_entries 
	WHERE name LIKE ? AND exists_now = 1 
	ORDER BY open_count DESC LIMIT 1;
	`
	err = db.QueryRow(likeQuery, "%"+query+"%").Scan(&dbPath)
	if err == nil && verifyPath(db, dbPath) {
		incrementOpenCount(db, dbPath)
		return dbPath, nil
	}

	// 3. Búsqueda física recursiva en los roots permitidos
	var foundPath string
	for _, root := range allowedRoots {
		rootClean := filepath.Clean(root)
		if strings.HasPrefix(rootClean, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				rootClean = filepath.Join(home, rootClean[1:])
			}
		}

		_ = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			// Ignorar si está bloqueado
			for _, blocked := range blockedPaths {
				if strings.HasPrefix(path, blocked) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// Si el nombre coincide con la búsqueda
			if strings.EqualFold(d.Name(), query) || strings.Contains(strings.ToLower(d.Name()), strings.ToLower(query)) {
				foundPath = path
				return filepath.SkipAll // Detener búsqueda
			}
			return nil
		})

		if foundPath != "" {
			break
		}
	}

	// Registrar búsqueda en historial
	var resultFound int = 0
	if foundPath != "" {
		resultFound = 1
		// Registrar nueva entrada en BD para agilizar futuras consultas
		cachePathEntry(db, foundPath)
		return foundPath, nil
	}

	// Guardar historial de búsqueda fallida o exitosa
	_, _ = db.Exec(`
		INSERT INTO path_search_history (query, normalized_query, result_path, result_found, search_roots, duration_ms)
		VALUES (?, ?, ?, ?, ?, 0);
	`, query, strings.ToLower(query), foundPath, resultFound, strings.Join(allowedRoots, ","))

	return "", fmt.Errorf("no se encontró ningún archivo o carpeta con el nombre '%s'", query)
}

func verifyPath(db *sql.DB, path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		// La ruta ya no existe, marcar en base de datos como stale
		_, _ = db.Exec("UPDATE path_entries SET exists_now = 0, is_stale = 1 WHERE path = ?", path)
		return false
	}
	_ = info
	return true
}

func incrementOpenCount(db *sql.DB, path string) {
	_, _ = db.Exec("UPDATE path_entries SET open_count = open_count + 1, last_seen_at = datetime('now') WHERE path = ?", path)
}

func cachePathEntry(db *sql.DB, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	name := info.Name()
	ext := filepath.Ext(name)
	parent := filepath.Dir(path)
	size := info.Size()
	modTime := info.ModTime().Format(time.RFC3339)
	
	var pathType string
	if info.IsDir() {
		pathType = "directory"
	} else {
		pathType = "file"
	}

	query := `
	INSERT INTO path_entries (path, name, type, extension, parent_path, size_bytes, modified_at, last_seen_at, last_verified_at, exists_now, is_stale)
	VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'), 1, 0)
	ON CONFLICT(path) DO UPDATE SET
		exists_now = 1,
		is_stale = 0;
	`
	_, err = db.Exec(query, path, name, pathType, ext, parent, size, modTime)
	if err == nil {
		var entityID int64
		row := db.QueryRow("SELECT id FROM path_entries WHERE path = ?", path)
		if err := row.Scan(&entityID); err == nil {
			_, _ = db.Exec("INSERT OR REPLACE INTO search_index (entity_type, entity_id, title, body, path) VALUES ('path_entry', ?, ?, ?, ?)",
				entityID, name, pathType+" "+ext, path)
		}
	}
}
