package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to retrieve user home dir: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Relative path untouched",
			input:    "./local/file.txt",
			expected: "./local/file.txt",
		},
		{
			name:     "Absolute path untouched",
			input:    "/usr/share/app",
			expected: "/usr/share/app",
		},
		{
			name:     "Tilde expansion",
			input:    "~/.local/share",
			expected: filepath.Join(home, ".local/share"),
		},
		{
			name:     "Just tilde",
			input:    "~",
			expected: home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInitDB(t *testing.T) {
	// Create a temporary dir for the test database
	tempDir, err := os.MkdirTemp("", "rbot-db-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "subdir/rbot_test.db")

	// Initialize the DB
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB returned error: %v", err)
	}
	defer database.Close()

	// Verify the database file was physically created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created at expected path: %s", dbPath)
	}

	// Verify that we can query one of the core tables created in Schema
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_memory'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database schema: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected user_memory table to exist, got count = %d", count)
	}

	// Verify FTS5 virtual table was created
	var ftsCount int
	err = database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='search_index'").Scan(&ftsCount)
	if err != nil {
		t.Fatalf("Failed to query search_index table: %v", err)
	}

	if ftsCount != 1 {
		t.Errorf("Expected search_index virtual table to exist, got count = %d", ftsCount)
	}
}
