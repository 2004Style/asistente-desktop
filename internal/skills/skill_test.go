package skills

import (
	"archive/zip"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestValidator_ValidAndInvalid(t *testing.T) {
	// Callback de herramienta ficticia
	toolExists := func(name string) bool {
		return name == "desktop.open_app" || name == "input.hotkey"
	}

	val := NewValidator(toolExists)

	// Habilidad válida
	valid := &SkillMetadata{
		Name:        "test-valid",
		Description: "Descripción válida",
		RiskLevel:   "medium",
		Status:      "disabled",
		Tools:       []string{"desktop.open_app"},
		VoiceTriggers: []string{
			"abre el navegador",
		},
	}
	if err := val.Validate(valid); err != nil {
		t.Errorf("Expected valid skill to pass, got: %v", err)
	}

	// Habilidad sin nombre
	noName := &SkillMetadata{
		Description: "Descripción válida",
	}
	if err := val.Validate(noName); err == nil {
		t.Errorf("Expected skill without name to fail")
	}

	// Habilidad con trigger genérico
	generic := &SkillMetadata{
		Name:        "test-generic",
		Description: "Descripción",
		RiskLevel:   "low",
		Status:      "enabled",
		VoiceTriggers: []string{
			"abre",
		},
	}
	if err := val.Validate(generic); err == nil {
		t.Errorf("Expected skill with generic trigger 'abre' to fail")
	}

	// Habilidad con herramienta inexistente
	badTool := &SkillMetadata{
		Name:        "test-badtool",
		Description: "Descripción",
		RiskLevel:   "low",
		Status:      "enabled",
		Tools:       []string{"bad.tool"},
	}
	if err := val.Validate(badTool); err == nil {
		t.Errorf("Expected skill with nonexistent tool to fail")
	}
}

func TestPermissions(t *testing.T) {
	// Válido con coincidencia exacta
	if err := ValidatePermissions([]string{"desktop.open_app"}, []string{"desktop.open_app"}); err != nil {
		t.Errorf("Exact match permissions should pass: %v", err)
	}

	// Válido con asterisco/categoría
	if err := ValidatePermissions([]string{"desktop:*"}, []string{"desktop.open_app"}); err != nil {
		t.Errorf("Category wildcard permissions should pass: %v", err)
	}

	// Válido con wildcard global
	if err := ValidatePermissions([]string{"*"}, []string{"desktop.open_app", "input.hotkey"}); err != nil {
		t.Errorf("Global wildcard permissions should pass: %v", err)
	}

	// Inválido (permisos insuficientes)
	if err := ValidatePermissions([]string{"media:*"}, []string{"desktop.open_app"}); err == nil {
		t.Errorf("Expected permissions check to fail on mismatch")
	}
}

func TestQuarantine(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	_, _ = db.Exec(`
		CREATE TABLE skills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			description TEXT,
			version TEXT,
			path TEXT,
			skill_md_path TEXT,
			frontmatter_json TEXT,
			permissions_json TEXT,
			risk_level TEXT,
			priority INTEGER DEFAULT 0,
			category TEXT,
			exclusive INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 0,
			trusted INTEGER DEFAULT 0,
			status TEXT DEFAULT 'disabled',
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE workspace_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT UNIQUE,
			value TEXT
		);
	`)

	_, _ = db.Exec("INSERT INTO skills (name, description, status, enabled) VALUES ('broken-skill', 'desc', 'enabled', 1)")

	// Registrar primer y segundo fallos
	quarantined, err := RecordFailure(db, "broken-skill", 3)
	if err != nil {
		t.Fatalf("RecordFailure failed: %v", err)
	}
	if quarantined {
		t.Errorf("Expected quarantined to be false after 1 failure")
	}

	_, _ = RecordFailure(db, "broken-skill", 3)

	// Registrar tercer fallo -> Debe entrar en cuarentena
	quarantined, err = RecordFailure(db, "broken-skill", 3)
	if err != nil {
		t.Fatalf("RecordFailure failed: %v", err)
	}
	if !quarantined {
		t.Errorf("Expected quarantined to be true after 3 failures")
	}

	// Verificar estado final
	var status string
	var enabled int
	_ = db.QueryRow("SELECT status, enabled FROM skills WHERE name = 'broken-skill'").Scan(&status, &enabled)
	if status != "quarantined" || enabled != 0 {
		t.Errorf("Expected skill to be quarantined and disabled, got status=%s, enabled=%d", status, enabled)
	}
}

func TestInstaller_ZipSlipAndValid(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-install-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	skillsDir := filepath.Join(tempDir, "installed_skills")
	installer := NewInstaller(skillsDir, NewValidator(nil))

	// 1. Crear archivo ZIP válido en memoria y guardarlo a disco
	validZipPath := filepath.Join(tempDir, "valid.zip")
	createTestZip(t, validZipPath, "SKILL.md", `---
name: my-installable-skill
description: "Habilidad instalada desde ZIP"
risk_level: low
status: disabled
voice_triggers:
  - "abre mi skill"
---
# Cuerpo
`)

	meta, err := installer.InstallZip(validZipPath)
	if err != nil {
		t.Fatalf("Expected valid ZIP to install successfully, got: %v", err)
	}

	if meta.Name != "my-installable-skill" {
		t.Errorf("Expected name 'my-installable-skill', got %s", meta.Name)
	}

	// 2. Crear archivo ZIP malicioso (Zip Slip)
	maliciousZipPath := filepath.Join(tempDir, "malicious.zip")
	createTestZip(t, maliciousZipPath, "../escaped.txt", "contenido")

	_, err = installer.InstallZip(maliciousZipPath)
	if err == nil {
		t.Errorf("Expected Zip Slip to be detected and installation to fail")
	} else if !strings.Contains(err.Error(), "Zip Slip") {
		t.Errorf("Expected error to mention 'Zip Slip', got: %v", err)
	}
}

func createTestZip(t *testing.T, zipPath string, filename string, content string) {
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	f, err := zipWriter.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create file inside zip: %v", err)
	}

	_, err = f.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write to file inside zip: %v", err)
	}
}
