package files

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// IndexRoots camina recursivamente por las rutas permitidas e indexa archivos/directorios en la base de datos.
// Admite indexación incremental y un límite de profundidad para optimizar las escrituras.
func IndexRoots(db *sql.DB, allowedRoots []string, blockedPaths []string, ignoreList []string, maxDepth int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de indexación: %v", err)
	}
	defer tx.Rollback()

	// Marcar inicialmente todos los archivos existentes en los roots a escanear como no existentes temporalmente,
	// para poder limpiar los que hayan sido eliminados.
	for _, root := range allowedRoots {
		rootClean := filepath.Clean(root)
		if strings.HasPrefix(rootClean, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				rootClean = filepath.Join(home, rootClean[1:])
			}
		}
		// Asegurar el separador para no coincidir con prefijos similares (ej. /home/user/Doc vs /home/user/Documentos)
		prefix := rootClean
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		_, err = tx.Exec("UPDATE path_entries SET exists_now = 0, is_stale = 1 WHERE path = ? OR path LIKE ?", rootClean, prefix+"%")
		if err != nil {
			return fmt.Errorf("error al limpiar estado previo para %s: %v", rootClean, err)
		}
	}

	// Preparar sentencias SQL para optimizar las transacciones masivas
	stmtCheck, err := tx.Prepare("SELECT id, modified_at, size_bytes FROM path_entries WHERE path = ?")
	if err != nil {
		return fmt.Errorf("error preparando stmtCheck: %v", err)
	}
	defer stmtCheck.Close()

	stmtInsertEntry, err := tx.Prepare(`
		INSERT INTO path_entries (path, name, type, extension, parent_path, size_bytes, modified_at, last_seen_at, last_verified_at, exists_now, is_stale)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'), 1, 0)
	`)
	if err != nil {
		return fmt.Errorf("error preparando stmtInsertEntry: %v", err)
	}
	defer stmtInsertEntry.Close()

	stmtUpdateEntry, err := tx.Prepare(`
		UPDATE path_entries SET 
			name = ?, type = ?, extension = ?, parent_path = ?, size_bytes = ?, modified_at = ?,
			last_seen_at = datetime('now'), last_verified_at = datetime('now'), exists_now = 1, is_stale = 0
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("error preparando stmtUpdateEntry: %v", err)
	}
	defer stmtUpdateEntry.Close()

	stmtUpdateSeen, err := tx.Prepare(`
		UPDATE path_entries SET 
			last_seen_at = datetime('now'), last_verified_at = datetime('now'), exists_now = 1, is_stale = 0
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("error preparando stmtUpdateSeen: %v", err)
	}
	defer stmtUpdateSeen.Close()

	stmtDeleteFTS, err := tx.Prepare("DELETE FROM search_index WHERE entity_type = 'path_entry' AND entity_id = ?")
	if err != nil {
		return fmt.Errorf("error preparando stmtDeleteFTS: %v", err)
	}
	defer stmtDeleteFTS.Close()

	stmtInsertFTS, err := tx.Prepare("INSERT INTO search_index (entity_type, entity_id, title, body, path) VALUES ('path_entry', ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("error preparando stmtInsertFTS: %v", err)
	}
	defer stmtInsertFTS.Close()

	totalProcessed := 0
	totalAdded := 0
	totalUpdated := 0
	totalUnchanged := 0

	for _, root := range allowedRoots {
		// Expandir tildes de home si aplica
		rootClean := filepath.Clean(root)
		if strings.HasPrefix(rootClean, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				rootClean = filepath.Join(home, rootClean[1:])
			}
		}
		
		// Verificar si el root existe y es accesible
		info, err := os.Stat(rootClean)
		if err != nil {
			continue // Omitir raíces que no existen físicamente en este momento
		}
		
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil // Ignorar errores de acceso y continuar
			}

			// Límite de profundidad
			rel, err := filepath.Rel(rootClean, path)
			if err == nil {
				depth := 0
				if rel != "." {
					depth = len(strings.Split(rel, string(filepath.Separator)))
				}
				if maxDepth > 0 && depth > maxDepth {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// Verificar si la ruta está bloqueada
			for _, blocked := range blockedPaths {
				if strings.HasPrefix(path, blocked) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// Verificar si coincide con algún término del ignoreList
			for _, ignore := range ignoreList {
				if strings.Contains(path, ignore) {
					if d.IsDir() && strings.HasSuffix(path, ignore) {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// Extraer metadatos
			fileInfo, err := d.Info()
			if err != nil {
				return nil
			}

			name := fileInfo.Name()
			ext := filepath.Ext(name)
			parent := filepath.Dir(path)
			size := fileInfo.Size()
			modTime := fileInfo.ModTime().Format(time.RFC3339)
			
			var pathType string
			if d.IsDir() {
				pathType = "directory"
			} else {
				pathType = "file"
			}

			totalProcessed++
			if totalProcessed%5000 == 0 {
				log.Printf("Indexando: %d archivos procesados...", totalProcessed)
			}

			// Consultar si ya existe en BD para determinar indexación incremental
			var existingID int64
			var existingModTime string
			var existingSize int64

			rowErr := stmtCheck.QueryRow(path).Scan(&existingID, &existingModTime, &existingSize)
			if rowErr == sql.ErrNoRows {
				// No existe, insertar nuevo
				res, err := stmtInsertEntry.Exec(path, name, pathType, ext, parent, size, modTime)
				if err != nil {
					return nil // Omitir errores individuales
				}
				newID, err := res.LastInsertId()
				if err == nil {
					_, _ = stmtInsertFTS.Exec(newID, name, pathType+" "+ext, path)
				}
				totalAdded++
			} else if rowErr == nil {
				// Existe, comparar si hubo modificaciones
				if existingModTime == modTime && existingSize == size {
					// No ha cambiado: solo actualizar marca de presencia
					_, _ = stmtUpdateSeen.Exec(existingID)
					totalUnchanged++
				} else {
					// Ha cambiado: actualizar metadatos y re-indexar en FTS
					_, _ = stmtUpdateEntry.Exec(name, pathType, ext, parent, size, modTime, existingID)
					_, _ = stmtDeleteFTS.Exec(existingID)
					_, _ = stmtInsertFTS.Exec(existingID, name, pathType+" "+ext, path)
					totalUpdated++
				}
			}

			return nil
		})
		
		if err != nil {
			return fmt.Errorf("error al indexar ruta %s: %v", rootClean, err)
		}
	}

	// Limpiar de search_index aquellas entradas cuyos paths ya no existen en disco (exists_now = 0)
	_, _ = tx.Exec(`
		DELETE FROM search_index 
		WHERE entity_type = 'path_entry' 
		AND entity_id IN (SELECT id FROM path_entries WHERE exists_now = 0)
	`)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al comprometer transacción de indexación: %v", err)
	}

	log.Printf("Resumen de indexación de rutas: Procesados: %d | Añadidos: %d | Actualizados: %d | Sin cambios: %d",
		totalProcessed, totalAdded, totalUpdated, totalUnchanged)

	return nil
}
