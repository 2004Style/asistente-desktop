package skills

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SkillMetadata struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Author        string   `json:"author"`
	Permissions   []string `json:"permissions"`
	RiskLevel     string   `json:"risk_level"`
	VoiceTriggers []string `json:"voice_triggers"`
}

// ScanSkills escanea el directorio de habilidades buscando archivos SKILL.md y los indexa en SQLite.
func ScanSkills(db *sql.DB, skillsDir string) error {
	if _, err := os.Stat(skillsDir); err != nil {
		// Crear carpeta de skills si no existe
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			return fmt.Errorf("error al crear directorio de skills: %v", err)
		}
		return nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderPath := filepath.Join(skillsDir, entry.Name())
		skillMdPath := filepath.Join(folderPath, "SKILL.md")
		
		if _, err := os.Stat(skillMdPath); err != nil {
			continue // No tiene SKILL.md
		}

		meta, err := parseSkillMd(skillMdPath)
		if err != nil {
			continue // Error al parsear metadata
		}

		if meta.Name == "" {
			meta.Name = entry.Name()
		}
		if meta.RiskLevel == "" {
			meta.RiskLevel = "medium"
		}

		frontmatterJSON, _ := json.Marshal(meta)
		permsJSON, _ := json.Marshal(meta.Permissions)

		// Guardar en la base de datos (deshabilitada por defecto)
		query := `
		INSERT INTO skills (name, description, version, path, skill_md_path, frontmatter_json, permissions_json, risk_level, enabled, trusted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			version = excluded.version,
			frontmatter_json = excluded.frontmatter_json,
			permissions_json = excluded.permissions_json,
			risk_level = excluded.risk_level;
		`

		_, err = db.Exec(query, meta.Name, meta.Description, meta.Version, folderPath, skillMdPath, string(frontmatterJSON), string(permsJSON), meta.RiskLevel)
		if err == nil {
			// Añadir al search_index FTS5
			var entityID int64
			row := db.QueryRow("SELECT id FROM skills WHERE name = ?", meta.Name)
			if err := row.Scan(&entityID); err == nil {
				_, _ = db.Exec("DELETE FROM search_index WHERE entity_type = 'skill' AND entity_id = ?", entityID)
				_, _ = db.Exec("INSERT INTO search_index (entity_type, entity_id, title, body, path) VALUES ('skill', ?, ?, ?, ?)",
					entityID, meta.Name, meta.Description, folderPath)
			}
		}
	}

	return nil
}

// EnableSkill activa una habilidad por su nombre.
func EnableSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET enabled = 1 WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

// DisableSkill desactiva una habilidad por su nombre.
func DisableSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET enabled = 0 WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

// FindMatchingSkills busca habilidades habilitadas que coincidan con la intención del usuario.
// Prioriza coincidencias en voice_triggers, y luego usa FTS5 sobre la descripción/nombre.
func FindMatchingSkills(db *sql.DB, userInput string) ([]SkillMetadata, error) {
	userInputLower := strings.ToLower(strings.TrimSpace(userInput))
	var matched []SkillMetadata
	matchedNames := make(map[string]bool)

	// 1. Obtener todas las habilidades habilitadas
	rows, err := db.Query("SELECT name, frontmatter_json FROM skills WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, frontmatterJSON string
		if err := rows.Scan(&name, &frontmatterJSON); err == nil {
			var meta SkillMetadata
			if err := json.Unmarshal([]byte(frontmatterJSON), &meta); err == nil {
				// Comprobar voice triggers
				for _, trigger := range meta.VoiceTriggers {
					triggerLower := strings.ToLower(trigger)
					if triggerLower != "" && strings.Contains(userInputLower, triggerLower) {
						if !matchedNames[meta.Name] {
							matched = append(matched, meta)
							matchedNames[meta.Name] = true
						}
						break
					}
				}
			}
		}
	}

	// 2. Coincidencia FTS5 como fallback o complemento
	words := strings.Fields(userInputLower)
	if len(words) > 0 {
		ftsQuery := strings.Join(words, " OR ")
		ftsRows, err := db.Query(`
			SELECT s.name, s.frontmatter_json 
			FROM skills s
			JOIN search_index idx ON idx.entity_id = s.id AND idx.entity_type = 'skill'
			WHERE s.enabled = 1 AND search_index MATCH ?
		`, ftsQuery)
		if err == nil {
			defer ftsRows.Close()
			for ftsRows.Next() {
				var name, frontmatterJSON string
				if err := ftsRows.Scan(&name, &frontmatterJSON); err == nil {
					if !matchedNames[name] {
						var meta SkillMetadata
						if err := json.Unmarshal([]byte(frontmatterJSON), &meta); err == nil {
							matched = append(matched, meta)
							matchedNames[name] = true
						}
					}
				}
			}
		}
	}

	return matched, nil
}

// LoadSkillBody lee y retorna el contenido completo de un SKILL.md de una habilidad.
func LoadSkillBody(db *sql.DB, name string) (string, error) {
	var skillMdPath string
	err := db.QueryRow("SELECT skill_md_path FROM skills WHERE name = ?", name).Scan(&skillMdPath)
	if err != nil {
		return "", fmt.Errorf("habilidad '%s' no encontrada: %v", name, err)
	}

	content, err := os.ReadFile(skillMdPath)
	if err != nil {
		return "", fmt.Errorf("error al leer archivo de habilidad '%s' en %s: %v", name, skillMdPath, err)
	}

	return string(content), nil
}

func parseSkillMd(path string) (*SkillMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	meta := &SkillMetadata{
		Permissions:   []string{},
		VoiceTriggers: []string{},
	}

	inFrontmatter := false
	dashCount := 0
	inPermissions := false
	inVoiceTriggers := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			dashCount++
			if dashCount == 1 {
				inFrontmatter = true
				continue
			} else if dashCount == 2 {
				inFrontmatter = false
				break // Termina de leer metadatos
			}
		}

		if !inFrontmatter {
			continue
		}

		// Detectar inicio de bloque permissions:
		if strings.HasPrefix(trimmed, "permissions:") {
			inPermissions = true
			inVoiceTriggers = false
			continue
		}

		// Detectar inicio de bloque voice_triggers:
		if strings.HasPrefix(trimmed, "voice_triggers:") {
			inVoiceTriggers = true
			inPermissions = false
			continue
		}

		// Si estamos en el bloque de permisos, leer elementos de lista
		if inPermissions {
			if strings.HasPrefix(trimmed, "-") {
				perm := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				meta.Permissions = append(meta.Permissions, perm)
				continue
			} else if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
				inPermissions = false
			}
		}

		// Si estamos en el bloque de voice_triggers, leer elementos de lista
		if inVoiceTriggers {
			if strings.HasPrefix(trimmed, "-") {
				trigger := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				trigger = strings.Trim(trigger, `"'`) // Quitar comillas
				meta.VoiceTriggers = append(meta.VoiceTriggers, trigger)
				continue
			} else if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
				inVoiceTriggers = false
			}
		}

		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`) // Quitar comillas simples/dobles

			switch key {
			case "name":
				meta.Name = val
			case "description":
				meta.Description = val
			case "version":
				meta.Version = val
			case "author":
				meta.Author = val
			case "risk_level":
				meta.RiskLevel = val
			}
		}
	}

	return meta, scanner.Err()
}
