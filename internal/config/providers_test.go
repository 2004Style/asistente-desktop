package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProvidersConfigCreatesSafeDefaults(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "providers.yaml")

	conf, err := LoadProvidersConfig(path)
	if err != nil {
		t.Fatalf("LoadProvidersConfig failed: %v", err)
	}
	if conf.ActiveProfile != "local_fast" {
		t.Fatalf("expected local_fast active profile, got %q", conf.ActiveProfile)
	}
	if conf.ActiveProvider != "ollama" {
		t.Fatalf("expected ollama active provider, got %q", conf.ActiveProvider)
	}
	if !conf.Providers["ollama"].Enabled {
		t.Fatal("expected ollama enabled by default")
	}
	if conf.Providers["openai"].Enabled {
		t.Fatal("openai must be disabled until explicitly configured")
	}
	if conf.Providers["openai"].SecretRef != "env:OPENAI_API_KEY" {
		t.Fatalf("expected secret ref, got %q", conf.Providers["openai"].SecretRef)
	}
	if _, ok := conf.Profiles["openai_api"]; !ok {
		t.Fatal("expected openai_api profile to exist")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected providers config to be written: %v", err)
	}
}

func TestLoadProvidersConfigOverlaysPartialYAML(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "providers.yaml")
	data := []byte(`
active_provider: openai
providers:
  openai:
    enabled: true
    type: openai
    auth_mode: api_key
    secret_ref: env:RBOT_TEST_KEY
    model: gpt-test
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write providers config: %v", err)
	}

	conf, err := LoadProvidersConfig(path)
	if err != nil {
		t.Fatalf("LoadProvidersConfig failed: %v", err)
	}
	if conf.ActiveProfile != "openai_api" {
		t.Fatalf("expected openai_api active profile, got %q", conf.ActiveProfile)
	}
	if conf.ActiveProvider != "openai" {
		t.Fatalf("expected openai active provider, got %q", conf.ActiveProvider)
	}
	if conf.ActiveModel != "gpt-test" {
		t.Fatalf("expected active model to preserve legacy override, got %q", conf.ActiveModel)
	}
	if !conf.Providers["openai"].Enabled {
		t.Fatal("expected openai enabled")
	}
	if _, ok := conf.Providers["ollama"]; !ok {
		t.Fatal("expected default ollama provider retained when YAML is partial")
	}
	if _, ok := conf.Profiles["local_fast"]; !ok {
		t.Fatal("expected default local_fast profile retained when YAML is partial")
	}
}
