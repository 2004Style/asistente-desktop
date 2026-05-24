package security

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite" // SQLite driver
)

func TestIsPathBlocked(t *testing.T) {
	home, _ := os.UserHomeDir()
	blockedPaths := []string{
		"~/.ssh",
		"**/.env",
		"/tmp/blocked_dir",
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "Safe path",
			path:     "/home/user/documents/test.txt",
			expected: false,
		},
		{
			name:     "Blocked exact ssh path",
			path:     filepath.Join(home, ".ssh"),
			expected: true,
		},
		{
			name:     "Blocked sub ssh path",
			path:     filepath.Join(home, ".ssh/id_rsa"),
			expected: true,
		},
		{
			name:     "Blocked exact env wildcard",
			path:     "/projects/app/.env",
			expected: true,
		},
		{
			name:     "Blocked env suffix wildcard (non-matching)",
			path:     "/projects/app/.env.local",
			expected: false,
		},
		{
			name:     "Blocked absolute directory path",
			path:     "/tmp/blocked_dir/subfile.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathBlocked(tt.path, blockedPaths)
			if result != tt.expected {
				t.Errorf("IsPathBlocked(%q) = %v; expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsCommandCritical(t *testing.T) {
	tests := []struct {
		command  string
		expected bool
	}{
		{
			command:  "echo 'Hello World'",
			expected: false,
		},
		{
			command:  "go test ./...",
			expected: false,
		},
		{
			command:  "sudo apt-get update",
			expected: true,
		},
		{
			command:  "rm -rf /tmp/test",
			expected: true,
		},
		{
			command:  "systemctl stop lightdm",
			expected: true,
		},
		{
			command:  "cat file.txt | grep something",
			expected: true, // containing pipe
		},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := IsCommandCritical(tt.command)
			if result != tt.expected {
				t.Errorf("IsCommandCritical(%q) = %v; expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestValidateToolAction(t *testing.T) {
	// Create an in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp DB: %v", err)
	}
	defer db.Close()

	// Create tables needed
	_, err = db.Exec(`
		CREATE TABLE internal_tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			enabled INTEGER,
			requires_confirmation INTEGER,
			risk_level TEXT
		);
		CREATE TABLE mcp_tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			enabled INTEGER,
			requires_confirmation INTEGER,
			risk_level TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Insert test tools
	_, _ = db.Exec(`INSERT INTO internal_tools (name, enabled, requires_confirmation, risk_level) VALUES 
		('safe_tool', 1, 0, 'low'),
		('confirm_tool', 1, 1, 'medium'),
		('forbidden_tool', 1, 0, 'forbidden'),
		('disabled_tool', 0, 0, 'low')
	`)
	_, _ = db.Exec(`INSERT INTO mcp_tools (name, enabled, requires_confirmation, risk_level) VALUES 
		('mcp_high_risk_tool', 1, 0, 'high')
	`)

	blockedPaths := []string{"/blocked"}

	// 1. Test safe tool
	allowed, confirm, _ := ValidateToolAction(db, "safe_tool", "/allowed/path", blockedPaths)
	if !allowed || confirm {
		t.Errorf("safe_tool validation failed: allowed=%v, confirm=%v", allowed, confirm)
	}

	// 2. Test blocked path
	allowed, _, _ = ValidateToolAction(db, "safe_tool", "/blocked/file.txt", blockedPaths)
	if allowed {
		t.Errorf("safe_tool on blocked path should not be allowed")
	}

	// 3. Test confirm tool
	allowed, confirm, _ = ValidateToolAction(db, "confirm_tool", "", blockedPaths)
	if !allowed || !confirm {
		t.Errorf("confirm_tool validation failed: allowed=%v, confirm=%v", allowed, confirm)
	}

	// 4. Test forbidden tool
	allowed, _, _ = ValidateToolAction(db, "forbidden_tool", "", blockedPaths)
	if allowed {
		t.Errorf("forbidden_tool should not be allowed")
	}

	// 5. Test disabled tool
	allowed, _, _ = ValidateToolAction(db, "disabled_tool", "", blockedPaths)
	if allowed {
		t.Errorf("disabled_tool should not be allowed")
	}

	// 6. Test mcp high risk tool
	allowed, confirm, _ = ValidateToolAction(db, "mcp_high_risk_tool", "", blockedPaths)
	if !allowed || !confirm {
		t.Errorf("mcp_high_risk_tool validation failed: allowed=%v, confirm=%v", allowed, confirm)
	}

	// 7. Test delete file (explicitly destructive)
	allowed, confirm, _ = ValidateToolAction(db, "files.delete_file", "/some/path", blockedPaths)
	if !allowed || !confirm {
		t.Errorf("files.delete_file validation failed: allowed=%v, confirm=%v", allowed, confirm)
	}
}
