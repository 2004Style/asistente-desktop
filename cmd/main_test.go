package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"rbot/internal/db"
	"rbot/internal/mcp"
	"strings"
	"testing"
)

func TestCleanCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"o abre firefox", "abre firefox"},
		{"o, abre youtube", "abre youtube"},
		{"o", ""},
		{"  hola, señor. ", "hola, señor"},
		{"abre la terminal", "abre la terminal"},
	}

	for _, tt := range tests {
		res := cleanCommand(tt.input)
		if res != tt.expected {
			t.Errorf("cleanCommand(%q) = %q; expected %q", tt.input, res, tt.expected)
		}
	}
}

func TestIsWhisperHallucination(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"gracias por ver", true},
		{"Subtítulos por Amara.org", true},
		{"hola", true},
		{"amara.org", true},
		{"abre el navegador", false},
		{"reproduce rock", false},
	}

	for _, tt := range tests {
		res := isWhisperHallucination(tt.input)
		if res != tt.expected {
			t.Errorf("isWhisperHallucination(%q) = %t; expected %t", tt.input, res, tt.expected)
		}
	}
}

func TestPrintUsage(t *testing.T) {
	// Redirect stdout to capture output
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()
	w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "Uso: rbot <comando> [argumentos]") {
		t.Errorf("Expected usage text to contain 'Uso: rbot <comando> [argumentos]', got: %s", output)
	}
}

func TestListSkills(t *testing.T) {
	// Create temporary db
	tempDir, err := os.MkdirTemp("", "rbot-main-skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	// Insert dummy skill
	_, _ = database.Exec("INSERT INTO skills (name, description, path, skill_md_path, enabled) VALUES ('test-skill', 'test description', 'path', 'path_md', 1)")

	// Capture stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	listSkills(database)
	w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "test-skill") {
		t.Errorf("Expected output to contain 'test-skill', got: %s", output)
	}
}

func TestListMcpTools(t *testing.T) {
	mcpManager := mcp.NewServerManager()
	defer mcpManager.CloseAll()

	// Capture stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	listMcpTools(context.Background(), mcpManager)
	w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "SERVIDORES Y HERRAMIENTAS MCP") {
		t.Errorf("Expected output to contain 'SERVIDORES Y HERRAMIENTAS MCP', got: %s", output)
	}
}
