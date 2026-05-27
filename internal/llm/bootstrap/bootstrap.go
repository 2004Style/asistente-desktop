package bootstrap

import (
	"fmt"
	"log"
	"strings"

	"rbot/internal/config"
	"rbot/internal/llm"
	"rbot/internal/llm/compatible"
	"rbot/internal/llm/ollama"
	"rbot/internal/llm/openai"
	"rbot/internal/secrets"
)

type Result struct {
	Registry *llm.Registry
	Active   string
	Warnings []string
}

func BuildRegistry(conf *config.Config, providers *config.ProvidersConfig, secretResolver *secrets.Manager) (*Result, error) {
	if providers == nil {
		providers = config.DefaultProvidersConfig()
	}
	if secretResolver == nil {
		secretResolver = secrets.NewManager()
	}

	registry := llm.NewRegistry()
	var warnings []string

	for name, entry := range providers.Providers {
		if !entry.Enabled {
			continue
		}
		provider, err := buildProvider(name, entry, secretResolver)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if err := registry.Register(provider); err != nil {
			warnings = append(warnings, err.Error())
		}
	}

	if len(registry.List()) == 0 {
		return nil, fmt.Errorf("no enabled LLM providers could be registered from providers.yaml")
	}

	active := providers.ActiveProvider
	if active == "" {
		for _, p := range registry.List() {
			active = p.Name()
			break
		}
	}
	if active == "" {
		return nil, fmt.Errorf("no active provider could be selected")
	}
	if activeModel := providers.ActiveModel; activeModel != "" {
		if p, ok := registry.Get(active); ok {
			p.SetModel(activeModel)
		}
	}

	for _, warning := range warnings {
		log.Printf("[LLM Bootstrap] %s", warning)
	}
	return &Result{Registry: registry, Active: active, Warnings: warnings}, nil
}

func buildProvider(name string, entry config.ProviderEntry, secretResolver *secrets.Manager) (llm.Provider, error) {
	providerType := strings.ToLower(strings.TrimSpace(entry.Type))
	if providerType == "" {
		providerType = strings.ToLower(name)
	}

	apiKey, err := resolveAPIKey(entry, secretResolver)
	if err != nil {
		return nil, fmt.Errorf("provider %s skipped: %w", name, err)
	}

	switch providerType {
	case "ollama":
		return ollama.NewProvider(entry.BaseURL, entry.Model, apiKey), nil
	case "openai":
		return openai.NewProviderWithBaseURL(entry.BaseURL, apiKey, entry.Model), nil
	case "compatible":
		if entry.BaseURL == "" {
			return nil, fmt.Errorf("provider %s skipped: compatible provider requires base_url", name)
		}
		return compatible.NewProvider(name, entry.BaseURL, apiKey, entry.Model), nil
	default:
		return nil, fmt.Errorf("provider %s skipped: unsupported type %q", name, providerType)
	}
}

func resolveAPIKey(entry config.ProviderEntry, secretResolver *secrets.Manager) (string, error) {
	authMode := strings.ToLower(strings.TrimSpace(entry.AuthMode))
	if authMode == "" || authMode == "none" {
		return "", nil
	}
	if entry.SecretRef == "" {
		return "", fmt.Errorf("auth mode %q requires secret_ref", entry.AuthMode)
	}
	return secretResolver.Resolve(entry.SecretRef)
}
