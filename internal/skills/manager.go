package skills

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SkillMetadata struct {
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Version          string              `json:"version"`
	Author           string              `json:"author"`
	Permissions      []string            `json:"permissions"`
	RiskLevel        string              `json:"risk_level"`
	Priority         int                 `json:"priority"`
	Category         string              `json:"category"`
	Exclusive        bool                `json:"exclusive"`
	Intents          []string            `json:"intents"`
	Tools            []string            `json:"tools"`
	VoiceTriggers    []string            `json:"voice_triggers"`
	NegativeTriggers []string            `json:"negative_triggers"`
	RequiredSlots    map[string][]string `json:"required_slots"`
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
		INSERT INTO skills (name, description, version, path, skill_md_path, frontmatter_json, permissions_json, risk_level, priority, category, exclusive, enabled, trusted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			version = excluded.version,
			frontmatter_json = excluded.frontmatter_json,
			permissions_json = excluded.permissions_json,
			risk_level = excluded.risk_level,
			priority = excluded.priority,
			category = excluded.category,
			exclusive = excluded.exclusive;
		`

		exclusiveInt := 0
		if meta.Exclusive {
			exclusiveInt = 1
		}

		_, err = db.Exec(query, meta.Name, meta.Description, meta.Version, folderPath, skillMdPath, string(frontmatterJSON), string(permsJSON), meta.RiskLevel, meta.Priority, meta.Category, exclusiveInt)
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

// GetAllEnabledSkills obtiene todas las habilidades que están habilitadas en la base de datos.
// La lógica de coincidencia (scoring) será manejada por el IntentRouter.
func GetAllEnabledSkills(db *sql.DB) ([]SkillMetadata, error) {
	var enabledSkills []SkillMetadata

	rows, err := db.Query("SELECT frontmatter_json FROM skills WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var frontmatterJSON string
		if err := rows.Scan(&frontmatterJSON); err == nil {
			var meta SkillMetadata
			if err := json.Unmarshal([]byte(frontmatterJSON), &meta); err == nil {
				enabledSkills = append(enabledSkills, meta)
			}
		}
	}

	return enabledSkills, nil
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
	var frontmatterLines []string
	dashCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			dashCount++
			if dashCount == 2 {
				break
			}
			continue
		}

		if dashCount == 1 {
			frontmatterLines = append(frontmatterLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	yamlContent := strings.Join(frontmatterLines, "\n")
	
	meta := &SkillMetadata{
		Permissions:      []string{},
		VoiceTriggers:    []string{},
		NegativeTriggers: []string{},
		Intents:          []string{},
		Tools:            []string{},
		RequiredSlots:    make(map[string][]string),
	}

	if err := yaml.Unmarshal([]byte(yamlContent), meta); err != nil {
		return nil, fmt.Errorf("error parseando YAML en %s: %v", path, err)
	}

	return meta, nil
}
