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
	Registry       *llm.Registry
	Active         string
	ActiveProfile  string
	ActiveModel    string
	ActiveAuthMode string
	Warnings       []string
}

func BuildRegistry(conf *config.Config, providers *config.ProvidersConfig, secretResolver *secrets.Manager) (*Result, error) {
	if providers == nil {
		providers = config.DefaultProvidersConfig()
	}
	if secretResolver == nil {
		secretResolver = secrets.NewManager()
	}
	providers.Normalize()

	registry := llm.NewRegistry()
	var warnings []string

	selection := providers.ResolveActiveSelection()

	for name, entry := range providers.Providers {
		if !entry.Enabled {
			continue
		}
		provider, err := buildProvider(name, entry, providers, selection, secretResolver)
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

	active := selection.ProviderName
	if active == "" {
		for _, p := range registry.List() {
			active = p.Name()
			break
		}
	}
	if active == "" {
		return nil, fmt.Errorf("no active provider could be selected")
	}
	if activeModel := selection.ModelID; activeModel != "" {
		if p, ok := registry.Get(active); ok {
			p.SetModel(activeModel)
		}
	}

	for _, warning := range warnings {
		log.Printf("[LLM Bootstrap] %s", warning)
	}
	return &Result{
		Registry:       registry,
		Active:         active,
		ActiveProfile:  selection.ProfileName,
		ActiveModel:    selection.ModelID,
		ActiveAuthMode: selection.AuthMode,
		Warnings:       warnings,
	}, nil
}

func buildProvider(name string, entry config.ProviderEntry, providers *config.ProvidersConfig, selection config.ActiveSelection, secretResolver *secrets.Manager) (llm.Provider, error) {
	providerType := strings.ToLower(strings.TrimSpace(entry.Type))
	if providerType == "" {
		providerType = strings.ToLower(name)
	}

	baseURL := strings.TrimSpace(entry.EffectiveBaseURL())
	modelID := entry.EffectiveModel()
	authMode := entry.EffectiveAuthMode()
	secretRef := entry.EffectiveSecretRef()
	if selection.ProviderName == name {
		if selection.ModelID != "" {
			modelID = selection.ModelID
		}
		if selection.AuthMode != "" {
			authMode = strings.ToLower(strings.TrimSpace(selection.AuthMode))
		}
		if selection.SecretRef != "" {
			secretRef = selection.SecretRef
		}
	}

	// Resolve capability attributes
	billingMode := "pay_as_you_go"
	runtimeMode := "direct_api"
	customHeader := ""

	for _, cap := range entry.Capabilities {
		if strings.EqualFold(cap.AuthMode, authMode) ||
			(cap.AuthMode == "browser_oauth" && authMode == "oauth_pkce") ||
			(cap.AuthMode == "oauth_pkce" && authMode == "browser_oauth") {
			billingMode = cap.BillingMode
			runtimeMode = cap.RuntimeMode
			customHeader = cap.Header
			if cap.BaseURL != "" {
				baseURL = cap.BaseURL
			}
			break
		}
	}

	if providerType == "ollama" || name == "ollama" {
		billingMode = "local"
		runtimeMode = "local_runtime"
	}

	apiKey, err := resolveAPIKey(authMode, secretRef, secretResolver)
	if err != nil {
		log.Printf("[LLM Bootstrap] Advertencia: no se pudo resolver secreto para el proveedor '%s' (%v). Registrando con credencial vacía temporal.", name, err)
		apiKey = ""
	}

	var delegate llm.Provider
	switch runtimeMode {
	case "local_runtime":
		delegate = ollama.NewProvider(baseURL, modelID, apiKey)
	default:
		if customHeader != "" {
			delegate = compatible.NewProviderWithHeader(name, baseURL, apiKey, modelID, customHeader)
		} else if providerType == "openai" {
			delegate = openai.NewProviderWithBaseURL(baseURL, apiKey, modelID)
		} else {
			delegate = compatible.NewProvider(name, baseURL, apiKey, modelID)
		}
	}

	return &llm.ProviderAdapter{
		ProviderName: name,
		ActiveModel:  modelID,
		AuthMode:     authMode,
		BillingMode:  billingMode,
		RuntimeMode:  runtimeMode,
		Delegate:     delegate,
	}, nil
}

func resolveAPIKey(authMode, secretRef string, secretResolver *secrets.Manager) (string, error) {
	authMode = strings.ToLower(strings.TrimSpace(authMode))
	switch authMode {
	case "", "none":
		return "", nil
	case "api_key":
		if secretRef == "" {
			return "", fmt.Errorf("auth mode %q requires secret_ref", authMode)
		}
		return secretResolver.Resolve(secretRef)
	case "browser_login", "browser_oauth", "oauth_pkce", "session", "oauth", "keyring", "adc", "service_account":
		// The runtime currently models these modes explicitly but only transports
		// API-key style credentials into the provider implementations.
		// Keep the selection visible and fail closed only when a secret is
		// required but missing in the active config.
		if secretRef == "" {
			return "", nil
		}
		return secretResolver.Resolve(secretRef)
	default:
		return "", fmt.Errorf("unsupported auth mode %q", authMode)
	}
}
