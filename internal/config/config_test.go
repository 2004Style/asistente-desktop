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

func TestLoadConfigAppliesDefaultsToPartialYAML(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-config-partial-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "rbot.yaml")
	partialYAML := `
agent:
    name: "PartialAgent"
model:
    model: "test-model"
files:
    allowed_roots:
        - "."
`
	if err := os.WriteFile(configPath, []byte(partialYAML), 0644); err != nil {
		t.Fatalf("Failed to write partial YAML: %v", err)
	}

	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig error on partial file: %v", err)
	}

	if conf.Agent.Name != "PartialAgent" {
		t.Fatalf("expected YAML override to be preserved, got %q", conf.Agent.Name)
	}
	if conf.Model.Model != "test-model" {
		t.Fatalf("expected model override to be preserved, got %q", conf.Model.Model)
	}

	assertNonEmpty := func(name, value string) {
		t.Helper()
		if value == "" {
			t.Fatalf("expected %s to be defaulted, got empty", name)
		}
	}
	if conf.Providers.ConfigFile != filepath.Join(tempDir, "providers.yaml") {
		t.Fatalf("expected providers file path to resolve next to config, got %q", conf.Providers.ConfigFile)
	}
	if conf.Providers.ActiveProvider != "ollama" || conf.Providers.ActiveModel == "" {
		t.Fatalf("expected provider defaults, got %#v", conf.Providers)
	}
	assertNonEmpty("runtime.socket_path", conf.Runtime.SocketPath)
	assertNonEmpty("runtime.event_socket_path", conf.Runtime.EventSocketPath)
	assertNonEmpty("hud.event_socket_path", conf.Hud.EventSocketPath)
	assertNonEmpty("workspace.path", conf.Workspace.Path)
	assertNonEmpty("skills.workspace_skills_path", conf.Skills.WorkspaceSkillsPath)

	if conf.Scheduler.TickSeconds <= 0 {
		t.Fatalf("expected scheduler.tick_seconds default > 0, got %d", conf.Scheduler.TickSeconds)
	}
	if !conf.Workspace.Enabled || !conf.Workspace.AutoCreate {
		t.Fatalf("expected workspace defaults enabled/autocreate, got enabled=%v autocreate=%v", conf.Workspace.Enabled, conf.Workspace.AutoCreate)
	}
	if conf.Skills.RemoteInstallEnabled {
		t.Fatalf("remote skill install must be disabled by default")
	}

	requiredBlocked := []string{".ssh", ".gnupg", ".aws", ".config/gh", ".config/rclone", ".docker/config.json", "**/.env", "*.pem", "*.key", "*.p12"}
	for _, required := range requiredBlocked {
		if !containsPathFragment(conf.Security.BlockedPaths, required) {
			t.Fatalf("expected blocked paths to include %q, got %#v", required, conf.Security.BlockedPaths)
		}
	}
}

func TestLoadCheckedInConfigKeepsRuntimeDefaults(t *testing.T) {
	conf, err := LoadConfig(filepath.Join("..", "..", "config", "rbot.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig error on checked-in config: %v", err)
	}
	if conf.Runtime.SocketPath == "" || conf.Runtime.EventSocketPath == "" {
		t.Fatalf("checked-in config must not zero runtime socket defaults: %#v", conf.Runtime)
	}
	if conf.Hud.EventSocketPath == "" || conf.Workspace.Path == "" {
		t.Fatalf("checked-in config must not zero HUD/workspace defaults: hud=%#v workspace=%#v", conf.Hud, conf.Workspace)
	}
	if conf.Skills.RemoteInstallEnabled {
		t.Fatalf("checked-in config must keep remote skill install disabled by default")
	}
}

func containsPathFragment(paths []string, fragment string) bool {
	for _, path := range paths {
		if strings.Contains(path, fragment) {
			return true
		}
	}
	return false
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
