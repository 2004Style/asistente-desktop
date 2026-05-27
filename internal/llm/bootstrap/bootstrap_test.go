package bootstrap

import (
	"testing"

	"rbot/internal/config"
	"rbot/internal/secrets"
)

func TestBuildRegistryRegistersEnabledProviders(t *testing.T) {
	t.Setenv("RBOT_TEST_OPENAI_KEY", "test-key")
	providers := config.DefaultProvidersConfig()
	providers.Providers["openai"] = config.ProviderEntry{
		Enabled:   true,
		Type:      "openai",
		AuthMode:  "api_key",
		SecretRef: "env:RBOT_TEST_OPENAI_KEY",
		Model:     "gpt-test",
	}

	result, err := BuildRegistry(config.DefaultConfig(), providers, secrets.NewManager())
	if err != nil {
		t.Fatalf("BuildRegistry failed: %v", err)
	}
	if _, ok := result.Registry.Get("ollama"); !ok {
		t.Fatal("expected ollama provider registered")
	}
	if _, ok := result.Registry.Get("openai"); !ok {
		t.Fatal("expected openai provider registered")
	}
}

func TestBuildRegistrySkipsMissingSecretProvider(t *testing.T) {
	providers := config.DefaultProvidersConfig()
	providers.Providers["openai"] = config.ProviderEntry{
		Enabled:   true,
		Type:      "openai",
		AuthMode:  "api_key",
		SecretRef: "env:RBOT_MISSING_OPENAI_KEY",
		Model:     "gpt-test",
	}

	result, err := BuildRegistry(config.DefaultConfig(), providers, secrets.NewManager())
	if err != nil {
		t.Fatalf("BuildRegistry failed: %v", err)
	}
	if _, ok := result.Registry.Get("openai"); ok {
		t.Fatal("expected openai provider skipped when secret is missing")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for missing secret")
	}
}

func TestBuildRegistryActiveModelOverride(t *testing.T) {
	providers := config.DefaultProvidersConfig()
	providers.ActiveProvider = "ollama"
	providers.ActiveModel = "qwen-custom"

	result, err := BuildRegistry(config.DefaultConfig(), providers, secrets.NewManager())
	if err != nil {
		t.Fatalf("BuildRegistry failed: %v", err)
	}
	p, ok := result.Registry.Get("ollama")
	if !ok {
		t.Fatal("expected ollama provider registered")
	}
	if p.ModelID() != "qwen-custom" {
		t.Fatalf("expected active model override, got %q", p.ModelID())
	}
}

func TestBuildRegistryRequiresEnabledProviders(t *testing.T) {
	providers := config.DefaultProvidersConfig()
	for name, entry := range providers.Providers {
		entry.Enabled = false
		providers.Providers[name] = entry
	}

	if _, err := BuildRegistry(config.DefaultConfig(), providers, secrets.NewManager()); err == nil {
		t.Fatal("expected BuildRegistry to fail when no providers are enabled")
	}
}
