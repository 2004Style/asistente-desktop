package skills

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSkillsManagement(t *testing.T) {
	// 1. Create temporary directory structure for skills
	tempDir, err := os.MkdirTemp("", "rbot-skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	skill1Dir := filepath.Join(tempDir, "music-test")
	if err := os.MkdirAll(skill1Dir, 0755); err != nil {
		t.Fatalf("Failed to create skill1 directory: %v", err)
	}

	skill1Content := `---
name: youtube-music-test
description: "Habilidad para reproducir música en youtube"
version: 1.0.0
author: tester
risk_level: low
voice_triggers:
  - "pon música"
  - "reproducir tema"
permissions:
  - exec:xdg-open
---

# Instrucciones
Reproduce música de YouTube.
`
	if err := os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644); err != nil {
		t.Fatalf("Failed to write skill1 markdown: %v", err)
	}

	skill2Dir := filepath.Join(tempDir, "sys-test")
	if err := os.MkdirAll(skill2Dir, 0755); err != nil {
		t.Fatalf("Failed to create skill2 directory: %v", err)
	}

	skill2Content := `---
name: system-off-test
description: "Habilidad para apagar el sistema"
version: 2.1.0
author: tester
risk_level: high
voice_triggers:
  - "apagar computadora"
permissions:
  - exec:shutdown
---

# Instrucciones
Apaga el sistema.
`
	if err := os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2Content), 0644); err != nil {
		t.Fatalf("Failed to write skill2 markdown: %v", err)
	}

	// 2. Initialize in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
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
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE search_index (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_type TEXT,
			entity_id INTEGER,
			title TEXT,
			body TEXT,
			path TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create mock tables: %v", err)
	}

	// 3. Test ScanSkills
	err = ScanSkills(db, tempDir)
	if err != nil {
		t.Fatalf("ScanSkills returned error: %v", err)
	}

	// Verify both skills are registered (disabled by default)
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 skills to be registered, got %d", count)
	}

	// 4. Test EnableSkill
	err = EnableSkill(db, "youtube-music-test")
	if err != nil {
		t.Fatalf("EnableSkill returned error: %v", err)
	}

	var enabled int
	_ = db.QueryRow("SELECT enabled FROM skills WHERE name = 'youtube-music-test'").Scan(&enabled)
	if enabled != 1 {
		t.Errorf("Expected youtube-music-test to be enabled (1), got %d", enabled)
	}

	// 5. Test DisableSkill
	err = DisableSkill(db, "youtube-music-test")
	if err != nil {
		t.Fatalf("DisableSkill returned error: %v", err)
	}

	_ = db.QueryRow("SELECT enabled FROM skills WHERE name = 'youtube-music-test'").Scan(&enabled)
	if enabled != 0 {
		t.Errorf("Expected youtube-music-test to be disabled (0), got %d", enabled)
	}

	// 6. Test GetAllEnabledSkills
	// Enable both skills for searching tests
	_, _ = db.Exec("UPDATE skills SET enabled = 1")

	enabledSkills, err := GetAllEnabledSkills(db)
	if err != nil {
		t.Fatalf("GetAllEnabledSkills returned error: %v", err)
	}

	if len(enabledSkills) != 2 {
		t.Fatalf("Expected 2 matches, got %d", len(enabledSkills))
	}

	foundYoutube := false
	for _, s := range enabledSkills {
		if s.Name == "youtube-music-test" {
			foundYoutube = true
		}
	}
	if !foundYoutube {
		t.Errorf("Expected to find youtube-music-test among enabled skills")
	}

	// 7. Test LoadSkillBody
	body, err := LoadSkillBody(db, "system-off-test")
	if err != nil {
		t.Fatalf("LoadSkillBody returned error: %v", err)
	}
	if !strings.Contains(body, "Apaga el sistema") {
		t.Errorf("Expected body to contain 'Apaga el sistema', got: %q", body)
	}
}
