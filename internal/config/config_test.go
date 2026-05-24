package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeConfig(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Could not get home directory for testing: %v", err)
	}

	conf := &Config{}
	conf.Files.AllowedRoots = []string{
		".",
		"~/Documentos",
		"~/Descargas",
	}
	conf.Security.BlockedPaths = []string{
		"~/.ssh",
		"**/.env",
	}

	NormalizeConfig(conf)

	// Verify CWD dot (.) resolution
	wd, err := os.Getwd()
	if err == nil && wd != "" {
		expectedWd := filepath.Clean(wd)
		if conf.Files.AllowedRoots[0] != expectedWd {
			t.Errorf("Expected AllowedRoots[0] to be CWD: %q, got %q", expectedWd, conf.Files.AllowedRoots[0])
		}
	}

	// Verify tilde (~) documents resolution
	expectedDocs := filepath.Clean(filepath.Join(home, "Documentos"))
	if conf.Files.AllowedRoots[1] != expectedDocs {
		t.Errorf("Expected AllowedRoots[1] to be expanded home docs: %q, got %q", expectedDocs, conf.Files.AllowedRoots[1])
	}

	// Verify blocked path ssh resolution
	expectedSsh := filepath.Clean(filepath.Join(home, ".ssh"))
	if conf.Security.BlockedPaths[0] != expectedSsh {
		t.Errorf("Expected BlockedPaths[0] to be expanded home ssh: %q, got %q", expectedSsh, conf.Security.BlockedPaths[0])
	}

	// Verify wildcards are kept clean but untouched regarding absolute path expansion
	if conf.Security.BlockedPaths[1] != "**/.env" {
		t.Errorf("Expected BlockedPaths[1] to keep wildcard signature: '**/.env', got %q", conf.Security.BlockedPaths[1])
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary file path
	tempDir, err := os.MkdirTemp("", "rbot-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "rbot.yaml")

	// 1. Test loading non-existent config generates default
	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig error on generating default: %v", err)
	}

	if conf.Agent.Name != "RBot" {
		t.Errorf("Expected default Agent Name to be 'RBot', got %q", conf.Agent.Name)
	}

	// Verify file was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Expected default config file to be written on disk, but it does not exist")
	}

	// 2. Test loading existing custom config
	customYAML := `
agent:
    name: "CustomAgent"
model:
    model: "test-model"
files:
    allowed_roots:
        - "."
`
	err = os.WriteFile(configPath, []byte(customYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write custom YAML: %v", err)
	}

	conf, err = LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig error on custom file: %v", err)
	}

	if conf.Agent.Name != "CustomAgent" {
		t.Errorf("Expected loaded custom Agent Name to be 'CustomAgent', got %q", conf.Agent.Name)
	}

	if conf.Model.Model != "test-model" {
		t.Errorf("Expected model to be 'test-model', got %q", conf.Model.Model)
	}

	// Verify dynamic resolution was applied on load
	wd, _ := os.Getwd()
	if !strings.Contains(conf.Files.AllowedRoots[0], "~") && conf.Files.AllowedRoots[0] != "." {
		expectedWd := filepath.Clean(wd)
		if conf.Files.AllowedRoots[0] != expectedWd {
			t.Errorf("Expected AllowedRoots[0] to be normalized to absolute path: %q, got %q", expectedWd, conf.Files.AllowedRoots[0])
		}
	}
}
