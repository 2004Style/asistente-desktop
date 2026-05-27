package onboarding

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rbot/internal/config"
)

func TestRunOpenAIOnboardingWritesProviderFiles(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "rbot.yaml")
	input := strings.NewReader("2\n\n\n\n")
	var output bytes.Buffer

	if err := Run(context.Background(), Options{ConfigPath: configPath, In: input, Out: &output}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if conf.Providers.ActiveProvider != "openai" {
		t.Fatalf("expected openai active provider, got %q", conf.Providers.ActiveProvider)
	}
	if conf.Model.Provider != "openai" {
		t.Fatalf("expected model provider openai, got %q", conf.Model.Provider)
	}
	if conf.Providers.ConfigFile != filepath.Join(tempDir, "providers.yaml") {
		t.Fatalf("expected providers file in temp dir, got %q", conf.Providers.ConfigFile)
	}

	providersConf, err := config.LoadProvidersConfig(filepath.Join(tempDir, "providers.yaml"))
	if err != nil {
		t.Fatalf("LoadProvidersConfig failed: %v", err)
	}
	if !providersConf.Providers["openai"].Enabled {
		t.Fatal("expected openai enabled")
	}
	if providersConf.Providers["openai"].SecretRef != "env:OPENAI_API_KEY" {
		t.Fatalf("expected default secret ref, got %q", providersConf.Providers["openai"].SecretRef)
	}
	if !strings.Contains(output.String(), "Onboarding completado") {
		t.Fatalf("expected completion message, got %q", output.String())
	}
}

func TestRunCompatibleOnboardingWritesCustomProvider(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "rbot.yaml")
	input := strings.NewReader("3\nmy-compatible\nhttps://llm.local\nmy-model\nenv:LLM_API_KEY\n")
	var output bytes.Buffer

	if err := Run(context.Background(), Options{ConfigPath: configPath, In: input, Out: &output}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	providersConf, err := config.LoadProvidersConfig(filepath.Join(tempDir, "providers.yaml"))
	if err != nil {
		t.Fatalf("LoadProvidersConfig failed: %v", err)
	}
	entry, ok := providersConf.Providers["my-compatible"]
	if !ok {
		t.Fatal("expected custom compatible provider saved")
	}
	if entry.Type != "compatible" || !entry.Enabled {
		t.Fatalf("unexpected provider entry: %#v", entry)
	}
	if entry.SecretRef != "env:LLM_API_KEY" {
		t.Fatalf("expected env secret ref, got %q", entry.SecretRef)
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if conf.Providers.ActiveProvider != "my-compatible" {
		t.Fatalf("expected active provider updated, got %q", conf.Providers.ActiveProvider)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "providers.yaml")); err != nil {
		t.Fatalf("expected providers file to exist: %v", err)
	}
}
