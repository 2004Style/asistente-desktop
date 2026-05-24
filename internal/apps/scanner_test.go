package apps

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestParseDesktopFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-desktop-parse-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "test.desktop")
	desktopContent := `[Desktop Entry]
Name=Test Application
Exec=testapp --verbose %U
Icon=test-icon
Categories=Utility;Development;
Type=Application
`
	if err := os.WriteFile(filePath, []byte(desktopContent), 0644); err != nil {
		t.Fatalf("Failed to write mock desktop file: %v", err)
	}

	app, err := parseDesktopFile(filePath)
	if err != nil {
		t.Fatalf("parseDesktopFile returned error: %v", err)
	}

	if app.DisplayName != "Test Application" {
		t.Errorf("Expected DisplayName 'Test Application', got %q", app.DisplayName)
	}
	if app.Command != "testapp --verbose %U" {
		t.Errorf("Expected Command 'testapp --verbose %%U', got %q", app.Command)
	}
	if app.Icon != "test-icon" {
		t.Errorf("Expected Icon 'test-icon', got %q", app.Icon)
	}
	if app.Categories != "Utility;Development;" {
		t.Errorf("Expected Categories 'Utility;Development;', got %q", app.Categories)
	}
}

func TestVerifyApplications(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE app_launchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			executable TEXT,
			is_available INTEGER,
			last_verified_at TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create mock tables: %v", err)
	}

	// Insert mock applications (one absolute path that exists, one absolute that doesn't, one binary in PATH, one binary missing)
	_, _ = db.Exec(`INSERT INTO app_launchers (id, name, executable, is_available) VALUES 
		(1, 'existing-abs', '/etc/hosts', 0),
		(2, 'nonexisting-abs', '/invalid/path/to/binary', 1),
		(3, 'available-cmd', 'ls', 0),
		(4, 'missing-cmd', 'invalidbinarycmdname', 1)
	`)

	// Mock execLookPath helper
	oldLookPath := execLookPath
	defer func() { execLookPath = oldLookPath }()

	execLookPath = func(file string) (string, error) {
		if file == "ls" {
			return "/bin/ls", nil
		}
		return "", fmt.Errorf("not found")
	}

	// Run validation
	VerifyApplications(db)

	var isAvail1, isAvail2, isAvail3, isAvail4 int
	_ = db.QueryRow("SELECT is_available FROM app_launchers WHERE id = 1").Scan(&isAvail1)
	_ = db.QueryRow("SELECT is_available FROM app_launchers WHERE id = 2").Scan(&isAvail2)
	_ = db.QueryRow("SELECT is_available FROM app_launchers WHERE id = 3").Scan(&isAvail3)
	_ = db.QueryRow("SELECT is_available FROM app_launchers WHERE id = 4").Scan(&isAvail4)

	if isAvail1 != 1 {
		t.Errorf("Expected existing absolute path to be marked available (1), got %d", isAvail1)
	}
	if isAvail2 != 0 {
		t.Errorf("Expected non-existing absolute path to be marked unavailable (0), got %d", isAvail2)
	}
	if isAvail3 != 1 {
		t.Errorf("Expected mock available binary to be marked available (1), got %d", isAvail3)
	}
	if isAvail4 != 0 {
		t.Errorf("Expected mock missing binary to be marked unavailable (0), got %d", isAvail4)
	}
}

func TestScanApplications(t *testing.T) {
	// Create mock database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE app_launchers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			display_name TEXT,
			executable TEXT,
			desktop_file TEXT,
			command TEXT,
			categories TEXT,
			icon TEXT,
			source TEXT,
			is_available INTEGER,
			last_verified_at TEXT,
			UNIQUE(name, executable)
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
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Create a mock desktop file in the local user directory (~/.local/share/applications) if accessible
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Skipping scan test, could not resolve home dir")
	}

	targetDir := filepath.Join(home, ".local/share/applications")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Skip("Skipping scan test, could not create user applications directory")
	}

	testFile := filepath.Join(targetDir, "rbot-test-scan-temp.desktop")
	content := `[Desktop Entry]
Name=RBot Scanner Test App
Exec=rbot-test-scan-exec %F
Icon=rbot-scanner-icon
Categories=System;Utility;
Type=Application
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Skip("Skipping scan test, could not write test desktop file")
	}
	defer os.Remove(testFile)

	// Run ScanApplications
	err = ScanApplications(db)
	if err != nil {
		t.Fatalf("ScanApplications returned error: %v", err)
	}

	// Verify it was correctly inserted in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM app_launchers WHERE name = 'rbot scanner test app'").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 app launcher to be scanned and indexed, got %d", count)
	}

	// Verify command cleaned up `%F`
	var cmd, exe string
	err = db.QueryRow("SELECT command, executable FROM app_launchers WHERE name = 'rbot scanner test app'").Scan(&cmd, &exe)
	if err != nil {
		t.Fatalf("QueryRow command failed: %v", err)
	}

	if cmd != "rbot-test-scan-exec" {
		t.Errorf("Expected cleaned command 'rbot-test-scan-exec', got %q", cmd)
	}
	if exe != "rbot-test-scan-exec" {
		t.Errorf("Expected cleaned executable 'rbot-test-scan-exec', got %q", exe)
	}
}
