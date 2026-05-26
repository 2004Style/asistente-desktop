package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ProviderEntry struct {
	Enabled    bool   `yaml:"enabled"`
	Type       string `yaml:"type"`
	AuthMode   string `yaml:"auth_mode"`
	BaseURL    string `yaml:"base_url"`
	SecretRef  string `yaml:"secret_ref"`
	Model      string `yaml:"model"`
	APIVersion string `yaml:"api_version,omitempty"`
}

type ProvidersConfig struct {
	ActiveProvider string                   `yaml:"active_provider"`
	ActiveModel    string                   `yaml:"active_model"`
	Providers      map[string]ProviderEntry `yaml:"providers"`
}

func DefaultProvidersConfig() *ProvidersConfig {
	return &ProvidersConfig{
		ActiveProvider: "ollama",
		ActiveModel:    "qwen2.5:7b",
		Providers: map[string]ProviderEntry{
			"ollama": {
				Enabled:  true,
				Type:     "ollama",
				AuthMode: "none",
				BaseURL:  "http://localhost:11434",
				Model:    "qwen2.5:7b",
			},
			"openai": {
				Enabled:   false,
				Type:      "openai",
				AuthMode:  "api_key",
				BaseURL:   "https://api.openai.com",
				SecretRef: "env:OPENAI_API_KEY",
				Model:     "gpt-4o-mini",
			},
			"compatible": {
				Enabled:   false,
				Type:      "compatible",
				AuthMode:  "api_key",
				BaseURL:   "",
				SecretRef: "",
				Model:     "",
			},
		},
	}
}

func SaveProvidersConfig(path string, conf *ProvidersConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadProvidersConfig(path string) (*ProvidersConfig, error) {
	conf := DefaultProvidersConfig()
	if path == "" {
		return conf, nil
	}
	path = expandPath(path, mustHomeDir())

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := SaveProvidersConfig(path, conf); err != nil {
			return nil, err
		}
		return conf, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, err
	}
	if conf.Providers == nil {
		conf.Providers = map[string]ProviderEntry{}
	}
	return conf, nil
}

func mustHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
