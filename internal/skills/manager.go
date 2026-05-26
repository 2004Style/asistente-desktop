package skills

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SkillMetadata struct {
	Name             string              `yaml:"name" json:"name"`
	Description      string              `yaml:"description" json:"description"`
	Version          string              `yaml:"version" json:"version"`
	Author           string              `yaml:"author" json:"author"`
	Permissions      []string            `yaml:"permissions" json:"permissions"`
	RiskLevel        string              `yaml:"risk_level" json:"risk_level"`
	Priority         int                 `yaml:"priority" json:"priority"`
	Category         string              `yaml:"category" json:"category"`
	Exclusive        bool                `yaml:"exclusive" json:"exclusive"`
	Status           string              `yaml:"status" json:"status"` // disabled, experimental, enabled, trusted, quarantined
	Intents          []string            `yaml:"intents" json:"intents"`
	Tools            []string            `yaml:"tools" json:"tools"`
	VoiceTriggers    []string            `yaml:"voice_triggers" json:"voice_triggers"`
	NegativeTriggers []string            `yaml:"negative_triggers" json:"negative_triggers"`
	RequiredSlots    map[string][]string `yaml:"required_slots" json:"required_slots"`
	ExamplesPositive []ExamplePositive   `yaml:"examples_positive" json:"examples_positive"`
	ExamplesNegative []ExampleNegative   `yaml:"examples_negative" json:"examples_negative"`
}

func ScanSkills(db *sql.DB, skillsDir string) error {
	if _, err := os.Stat(skillsDir); err != nil {
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
			continue // Sin SKILL.md
		}

		meta, _, err := ParseSkillMd(skillMdPath)
		if err != nil {
			continue // Ignorar si no se puede parsear
		}

		if meta.Name == "" {
			meta.Name = entry.Name()
		}

		// Validar de forma básica (no estricta en auto-discover para evitar bloqueos)
		val := NewValidator(nil)
		validationErrStr := ""
		if err := val.Validate(meta); err != nil {
			validationErrStr = err.Error()
		}

		hash, _ := CalculateDirHash(folderPath)

		frontmatterJSON, _ := json.Marshal(meta)
		permsJSON, _ := json.Marshal(meta.Permissions)

		// Guardar en la base de datos manteniendo el status existente si ya existe
		var existingStatus string
		err = db.QueryRow("SELECT status FROM skills WHERE name = ?", meta.Name).Scan(&existingStatus)
		if err == sql.ErrNoRows {
			// Si es nueva y tiene riesgo alto, forzar disabled
			if meta.RiskLevel == "high" || meta.RiskLevel == "critical" {
				meta.Status = "disabled"
			} else {
				meta.Status = "disabled" // Por seguridad inicial, por defecto disabled
			}
		} else if err == nil {
			meta.Status = existingStatus
		}

		query := `
		INSERT INTO skills (name, description, version, path, skill_md_path, frontmatter_json, permissions_json, risk_level, priority, category, exclusive, status, hash, last_validated_at, validation_errors)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			version = excluded.version,
			frontmatter_json = excluded.frontmatter_json,
			permissions_json = excluded.permissions_json,
			risk_level = excluded.risk_level,
			priority = excluded.priority,
			category = excluded.category,
			exclusive = excluded.exclusive,
			hash = excluded.hash,
			last_validated_at = excluded.last_validated_at,
			validation_errors = excluded.validation_errors;
		`

		exclusiveInt := 0
		if meta.Exclusive {
			exclusiveInt = 1
		}

		_, err = db.Exec(query,
			meta.Name,
			meta.Description,
			meta.Version,
			folderPath,
			skillMdPath,
			string(frontmatterJSON),
			string(permsJSON),
			meta.RiskLevel,
			meta.Priority,
			meta.Category,
			exclusiveInt,
			meta.Status,
			hash,
			time.Now().Format(time.RFC3339),
			validationErrStr,
		)
		if err != nil {
			continue
		}

		// Obtener ID para guardar triggers y permisos asociados
		var skillID int64
		if err := db.QueryRow("SELECT id FROM skills WHERE name = ?", meta.Name).Scan(&skillID); err == nil {
			// Limpiar triggers antiguos
			_, _ = db.Exec("DELETE FROM skill_triggers WHERE skill_id = ?", skillID)
			for _, trig := range meta.VoiceTriggers {
				_, _ = db.Exec("INSERT OR IGNORE INTO skill_triggers (skill_id, trigger, trigger_type) VALUES (?, ?, 'positive')", skillID, trig)
			}
			for _, trig := range meta.NegativeTriggers {
				_, _ = db.Exec("INSERT OR IGNORE INTO skill_triggers (skill_id, trigger, trigger_type) VALUES (?, ?, 'negative')", skillID, trig)
			}

			// Limpiar permisos antiguos
			_, _ = db.Exec("DELETE FROM skill_permissions WHERE skill_id = ?", skillID)
			for _, perm := range meta.Permissions {
				_, _ = db.Exec("INSERT OR IGNORE INTO skill_permissions (skill_id, permission) VALUES (?, ?)", skillID, perm)
			}

			// Actualizar FTS5
			_, _ = db.Exec("DELETE FROM search_index WHERE entity_type = 'skill' AND entity_id = ?", skillID)
			_, _ = db.Exec("INSERT INTO search_index (entity_type, entity_id, title, body, path) VALUES ('skill', ?, ?, ?, ?)",
				skillID, meta.Name, meta.Description, folderPath)
		}
	}

	return nil
}

func EnableSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET enabled = 1, status = 'enabled' WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

func DisableSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET enabled = 0, status = 'disabled' WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

func TrustSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET status = 'trusted' WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

func QuarantineSkill(db *sql.DB, name string) error {
	res, err := db.Exec("UPDATE skills SET status = 'quarantined', enabled = 0 WHERE name = ?", name)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no se encontró ninguna habilidad con el nombre '%s'", name)
	}
	return nil
}

func GetAllEnabledSkills(db *sql.DB) ([]SkillMetadata, error) {
	var enabledSkills []SkillMetadata

	// Habilidades habilitadas son aquellas que tienen status = 'enabled' o 'trusted' o enabled = 1
	rows, err := db.Query("SELECT frontmatter_json, COALESCE(status, '') FROM skills WHERE status IN ('enabled', 'trusted') OR enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var frontmatterJSON string
		var status string
		if err := rows.Scan(&frontmatterJSON, &status); err == nil {
			var meta SkillMetadata
			if err := json.Unmarshal([]byte(frontmatterJSON), &meta); err == nil {
				meta.Status = status
				enabledSkills = append(enabledSkills, meta)
			}
		}
	}

	return enabledSkills, nil
}

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
