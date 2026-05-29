package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rbot/internal/llm"

	"gopkg.in/yaml.v3"
)

type ModelEntry struct {
	ID           string                `yaml:"id" json:"id"`
	Label        string                `yaml:"label,omitempty" json:"label,omitempty"`
	Enabled      bool                  `yaml:"enabled" json:"enabled"`
	Default      bool                  `yaml:"default" json:"default"`
	Pricing      string                `yaml:"pricing,omitempty" json:"pricing,omitempty"`
	Capabilities llm.ModelCapabilities `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type AuthModeEntry struct {
	Type       string `yaml:"type" json:"type"`
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	SecretRef  string `yaml:"secret_ref,omitempty" json:"secret_ref,omitempty"`
	SessionRef string `yaml:"session_ref,omitempty" json:"session_ref,omitempty"`
	Notes      string `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type DiscoveryConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	Strategy        string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	CacheTTLSeconds int    `yaml:"cache_ttl_seconds,omitempty" json:"cache_ttl_seconds,omitempty"`
}

type ProviderConnection struct {
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}

type ProfileEntry struct {
	Provider    string `yaml:"provider" json:"provider"`
	Model       string `yaml:"model" json:"model"`
	AuthMode    string `yaml:"auth_mode" json:"auth_mode"`
	BillingMode string `yaml:"billing_mode,omitempty" json:"billing_mode,omitempty"`
	RuntimeMode string `yaml:"runtime_mode,omitempty" json:"runtime_mode,omitempty"`
	SecretRef   string `yaml:"secret_ref,omitempty" json:"secret_ref,omitempty"`
	SessionRef  string `yaml:"session_ref,omitempty" json:"session_ref,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
}

type ProviderEntry struct {
	Enabled       bool                     `yaml:"enabled" json:"enabled"`
	Type          string                   `yaml:"type" json:"type"`
	Category      string                   `yaml:"category,omitempty" json:"category,omitempty"`
	ProviderType  string                   `yaml:"provider_type,omitempty" json:"provider_type,omitempty"`
	DisplayName   string                   `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Connection    ProviderConnection       `yaml:"connection,omitempty" json:"connection,omitempty"`
	BaseURL       string                   `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	AuthMode      string                   `yaml:"auth_mode,omitempty" json:"auth_mode,omitempty"`
	BillingMode   string                   `yaml:"billing_mode,omitempty" json:"billing_mode,omitempty"`
	RuntimeMode   string                   `yaml:"runtime_mode,omitempty" json:"runtime_mode,omitempty"`
	SecretRef     string                   `yaml:"secret_ref,omitempty" json:"secret_ref,omitempty"`
	SessionRef    string                   `yaml:"session_ref,omitempty" json:"session_ref,omitempty"`
	DefaultModel  string                   `yaml:"default_model,omitempty" json:"default_model,omitempty"`
	Model         string                   `yaml:"model,omitempty" json:"model,omitempty"` // legacy compatibility
	Compatibility []string                 `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	Supports      map[string]bool          `yaml:"supports,omitempty" json:"supports,omitempty"`
	Models        map[string]ModelEntry    `yaml:"models,omitempty" json:"models,omitempty"`
	Discovery     DiscoveryConfig          `yaml:"discovery,omitempty" json:"discovery,omitempty"`
	AuthModes     map[string]AuthModeEntry `yaml:"auth_modes,omitempty" json:"auth_modes,omitempty"`
	APIVersion    string                   `yaml:"api_version,omitempty" json:"api_version,omitempty"`
	Capabilities  []llm.ProviderCapability `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type ActiveConfig struct {
	Provider    string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model       string `yaml:"model,omitempty" json:"model,omitempty"`
	AuthProfile string `yaml:"auth_profile,omitempty" json:"auth_profile,omitempty"`
}

type DefaultsConfig struct {
	TimeoutMS   int     `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	Stream      bool    `yaml:"stream,omitempty" json:"stream,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
}

type AuthModeDetail struct {
	InternalName string `yaml:"internal_name" json:"internal_name"`
	DisplayName  string `yaml:"display_name" json:"display_name"`
	Flow         string `yaml:"flow,omitempty" json:"flow,omitempty"`
	CallbackHost string `yaml:"callback_host,omitempty" json:"callback_host,omitempty"`
	CallbackPath string `yaml:"callback_path,omitempty" json:"callback_path,omitempty"`
	TokenStore   string `yaml:"token_store,omitempty" json:"token_store,omitempty"`
}

type BillingModeDetail struct {
	Description string `yaml:"description" json:"description"`
}

type RuntimeModeDetail struct {
	Description string `yaml:"description" json:"description"`
}

type ProvidersConfig struct {
	Version        int                          `yaml:"version,omitempty" json:"version,omitempty"`
	Active         ActiveConfig                 `yaml:"active,omitempty" json:"active,omitempty"`
	Defaults       DefaultsConfig               `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	ActiveProfile  string                       `yaml:"active_profile,omitempty" json:"active_profile,omitempty"`
	ActiveProvider string                       `yaml:"active_provider,omitempty" json:"active_provider,omitempty"`
	ActiveModel    string                       `yaml:"active_model,omitempty" json:"active_model,omitempty"`
	ActiveAuthMode string                       `yaml:"active_auth_mode,omitempty" json:"active_auth_mode,omitempty"`
	AuthModes      map[string]AuthModeDetail    `yaml:"auth_modes,omitempty" json:"auth_modes,omitempty"`
	BillingModes   map[string]BillingModeDetail `yaml:"billing_modes,omitempty" json:"billing_modes,omitempty"`
	RuntimeModes   map[string]RuntimeModeDetail `yaml:"runtime_modes,omitempty" json:"runtime_modes,omitempty"`
	Providers      map[string]ProviderEntry     `yaml:"providers" json:"providers"`
	Profiles       map[string]ProfileEntry      `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	lastActive     ActiveConfig                 `yaml:"-" json:"-"`
}

func (c *ProvidersConfig) UnmarshalYAML(value *yaml.Node) error {
	type Alias ProvidersConfig
	aux := &struct {
		*Alias `yaml:",inline"`
	}{
		Alias: (*Alias)(c),
	}

	if err := value.Decode(aux); err != nil {
		return err
	}

	// Verificar si el nodo contiene la clave "active"
	hasActive := false
	for i := 0; i < len(value.Content); i += 2 {
		if i+1 < len(value.Content) && value.Content[i].Value == "active" {
			hasActive = true
			break
		}
	}

	// Si no vino explícitamente en el YAML, limpiar ActiveConfig para
	// que no pise los campos planos durante Normalize
	if !hasActive {
		c.Active = ActiveConfig{}
	}

	return nil
}

type ActiveSelection struct {
	ProfileName  string
	ProviderName string
	ModelID      string
	AuthMode     string
	SecretRef    string
	SessionRef   string
}

func DefaultProvidersConfig() *ProvidersConfig {
	conf := &ProvidersConfig{
		Version:        2,
		ActiveProfile:  "local_fast",
		ActiveProvider: "ollama",
		ActiveModel:    "qwen2.5:7b",
		ActiveAuthMode: "none",
		AuthModes: map[string]AuthModeDetail{
			"browser_oauth": {
				InternalName: "oauth_pkce",
				DisplayName:  "Iniciar sesión con navegador",
				Flow:         "authorization_code_pkce",
				CallbackHost: "127.0.0.1",
				CallbackPath: "/auth/callback",
				TokenStore:   "system_keychain",
			},
			"device_code": {
				InternalName: "device_code",
				DisplayName:  "Iniciar sesión con código",
				TokenStore:   "system_keychain",
			},
			"api_key": {
				InternalName: "api_key",
				DisplayName:  "Usar API Key",
				TokenStore:   "system_keychain",
			},
			"adc": {
				InternalName: "application_default_credentials",
				DisplayName:  "Usar credenciales de Google Cloud",
			},
			"service_account": {
				InternalName: "service_account",
				DisplayName:  "Usar cuenta de servicio",
			},
			"none": {
				InternalName: "none",
				DisplayName:  "Sin autenticación",
			},
		},
		BillingModes: map[string]BillingModeDetail{
			"subscription":  {Description: "Uso asociado a una suscripción del usuario"},
			"pay_as_you_go": {Description: "Uso facturado por consumo directo de API"},
			"credits":       {Description: "Uso descontado de créditos prepagados"},
			"cloud_project": {Description: "Uso facturado a un proyecto cloud"},
			"local":         {Description: "Ejecución local sin facturación externa"},
		},
		RuntimeModes: map[string]RuntimeModeDetail{
			"official_cli_session": {Description: "Uso mediante sesión gestionada por CLI oficial"},
			"direct_api":           {Description: "Uso directo contra la API del proveedor"},
			"gateway_api":          {Description: "Uso mediante gateway compatible como OpenRouter"},
			"local_runtime":        {Description: "Uso mediante runtime local como Ollama"},
		},
		Providers: map[string]ProviderEntry{
			"ollama": {
				Enabled:      true,
				Type:         "ollama",
				DisplayName:  "Ollama",
				AuthMode:     "none",
				BaseURL:      "http://localhost:11434",
				Connection:   ProviderConnection{BaseURL: "http://localhost:11434"},
				DefaultModel: "qwen2.5:7b",
				Model:        "qwen2.5:7b",
				Discovery:    DiscoveryConfig{Enabled: true, Strategy: "ollama_api", CacheTTLSeconds: 60},
				AuthModes: map[string]AuthModeEntry{
					"none": {Type: "none", Enabled: true},
				},
				Models: map[string]ModelEntry{
					"qwen2.5:7b": {
						ID:      "qwen2.5:7b",
						Label:   "Qwen 2.5 7B",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            false,
							ConversationState: true,
						},
					},
					"qwen2.5-coder:7b": {
						ID:      "qwen2.5-coder:7b",
						Label:   "Qwen 2.5 Coder 7B",
						Enabled: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            false,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:    "none",
						BillingMode: "local",
						RuntimeMode: "local_runtime",
						BaseURL:     "http://localhost:11434",
					},
				},
			},
			"openai": {
				Enabled:      false,
				Type:         "openai",
				DisplayName:  "OpenAI",
				AuthMode:     "api_key",
				BaseURL:      "https://api.openai.com/v1",
				Connection:   ProviderConnection{BaseURL: "https://api.openai.com/v1"},
				DefaultModel: "gpt-4o-mini",
				Model:        "gpt-4o-mini",
				SecretRef:    "env:OPENAI_API_KEY",
				Discovery:    DiscoveryConfig{Enabled: true, Strategy: "openai_models_api", CacheTTLSeconds: 3600},
				AuthModes: map[string]AuthModeEntry{
					"api_key":       {Type: "api_key", Enabled: true, SecretRef: "env:OPENAI_API_KEY"},
					"browser_oauth": {Type: "browser_oauth", Enabled: true, SessionRef: "system_keyring:openai_session"},
				},
				Models: map[string]ModelEntry{
					"gpt-4o-mini": {
						ID:      "gpt-4o-mini",
						Label:   "GPT-4o mini",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            true,
							ConversationState: true,
						},
					},
					"gpt-4o": {
						ID:      "gpt-4o",
						Label:   "GPT-4o",
						Enabled: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            true,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:    "browser_oauth",
						BillingMode: "subscription",
						RuntimeMode: "official_cli_session",
						Usage:       "ChatGPT / Codex subscription",
					},
					{
						AuthMode:    "api_key",
						BillingMode: "pay_as_you_go",
						RuntimeMode: "direct_api",
						EnvKey:      "OPENAI_API_KEY",
						BaseURL:     "https://api.openai.com/v1",
					},
				},
			},
			"anthropic": {
				Enabled:      false,
				Type:         "compatible",
				DisplayName:  "Anthropic Claude",
				AuthMode:     "api_key",
				BaseURL:      "https://api.anthropic.com",
				Connection:   ProviderConnection{BaseURL: "https://api.anthropic.com"},
				DefaultModel: "claude-3-5-sonnet-latest",
				Model:        "claude-3-5-sonnet-latest",
				SecretRef:    "env:ANTHROPIC_API_KEY",
				Discovery:    DiscoveryConfig{Enabled: false, Strategy: "compatible_models_api", CacheTTLSeconds: 3600},
				AuthModes: map[string]AuthModeEntry{
					"api_key":       {Type: "api_key", Enabled: true, SecretRef: "env:ANTHROPIC_API_KEY"},
					"browser_oauth": {Type: "browser_oauth", Enabled: true},
				},
				Models: map[string]ModelEntry{
					"claude-3-5-sonnet-latest": {
						ID:      "claude-3-5-sonnet-latest",
						Label:   "Claude 3.5 Sonnet",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            true,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:    "browser_oauth",
						BillingMode: "subscription",
						RuntimeMode: "official_cli_session",
						Usage:       "Claude subscription / Claude Code",
					},
					{
						AuthMode:    "api_key",
						BillingMode: "pay_as_you_go",
						RuntimeMode: "direct_api",
						EnvKey:      "ANTHROPIC_API_KEY",
						Header:      "x-api-key",
					},
				},
			},
			"google_gemini": {
				Enabled:      false,
				Type:         "compatible",
				DisplayName:  "Google Gemini",
				AuthMode:     "api_key",
				BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
				Connection:   ProviderConnection{BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai"},
				DefaultModel: "gemini-2.5-flash",
				Model:        "gemini-2.5-flash",
				SecretRef:    "env:GEMINI_API_KEY",
				Discovery:    DiscoveryConfig{Enabled: false, Strategy: "compatible_models_api", CacheTTLSeconds: 3600},
				AuthModes: map[string]AuthModeEntry{
					"api_key":         {Type: "api_key", Enabled: true, SecretRef: "env:GEMINI_API_KEY"},
					"browser_oauth":   {Type: "browser_oauth", Enabled: true},
					"adc":             {Type: "adc", Enabled: true},
					"service_account": {Type: "service_account", Enabled: true},
				},
				Models: map[string]ModelEntry{
					"gemini-2.5-flash": {
						ID:      "gemini-2.5-flash",
						Label:   "Gemini 2.5 Flash",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            true,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:    "browser_oauth",
						BillingMode: "subscription",
						RuntimeMode: "official_cli_session",
						Usage:       "Login with Google / Gemini CLI",
					},
					{
						AuthMode:    "api_key",
						BillingMode: "pay_as_you_go",
						RuntimeMode: "direct_api",
						EnvKey:      "GEMINI_API_KEY",
					},
					{
						AuthMode:    "adc",
						BillingMode: "cloud_project",
						RuntimeMode: "direct_api",
						Usage:       "Vertex AI with Google Cloud project",
					},
					{
						AuthMode:    "service_account",
						BillingMode: "cloud_project",
						RuntimeMode: "direct_api",
						Usage:       "Vertex AI service account",
					},
				},
			},
			"openrouter": {
				Enabled:      true,
				Type:         "compatible",
				DisplayName:  "OpenRouter",
				AuthMode:     "api_key",
				BaseURL:      "https://openrouter.ai/api/v1",
				Connection:   ProviderConnection{BaseURL: "https://openrouter.ai/api/v1"},
				DefaultModel: "google/gemini-2.5-flash",
				Model:        "google/gemini-2.5-flash",
				SecretRef:    "env:OPENROUTER_API_KEY",
				Discovery:    DiscoveryConfig{Enabled: false, Strategy: "compatible_models_api", CacheTTLSeconds: 3600},
				AuthModes: map[string]AuthModeEntry{
					"api_key": {Type: "api_key", Enabled: true, SecretRef: "env:OPENROUTER_API_KEY"},
				},
				Models: map[string]ModelEntry{
					"google/gemini-2.5-flash": {
						ID:      "google/gemini-2.5-flash",
						Label:   "Gemini 2.5 Flash (via OpenRouter)",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            false,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:    "api_key",
						BillingMode: "credits",
						RuntimeMode: "gateway_api",
						EnvKey:      "OPENROUTER_API_KEY",
						BaseURL:     "https://openrouter.ai/api/v1",
					},
				},
			},
			"deepseek": {
				Enabled:      false,
				Type:         "compatible",
				DisplayName:  "DeepSeek",
				AuthMode:     "api_key",
				BaseURL:      "https://api.deepseek.com",
				Connection:   ProviderConnection{BaseURL: "https://api.deepseek.com"},
				DefaultModel: "deepseek-chat",
				Model:        "deepseek-chat",
				SecretRef:    "env:DEEPSEEK_API_KEY",
				Discovery:    DiscoveryConfig{Enabled: false, Strategy: "compatible_models_api", CacheTTLSeconds: 3600},
				AuthModes: map[string]AuthModeEntry{
					"api_key": {Type: "api_key", Enabled: true, SecretRef: "env:DEEPSEEK_API_KEY"},
				},
				Models: map[string]ModelEntry{
					"deepseek-chat": {
						ID:      "deepseek-chat",
						Label:   "DeepSeek Chat",
						Enabled: true,
						Default: true,
						Capabilities: llm.ModelCapabilities{
							ToolCalling:       true,
							Streaming:         true,
							Vision:            false,
							ConversationState: true,
						},
					},
				},
				Capabilities: []llm.ProviderCapability{
					{
						AuthMode:      "api_key",
						BillingMode:   "pay_as_you_go",
						RuntimeMode:   "direct_api",
						EnvKey:        "DEEPSEEK_API_KEY",
						BaseURL:       "https://api.deepseek.com",
						Compatibility: []string{"openai", "anthropic"},
					},
				},
			},
		},
		Profiles: map[string]ProfileEntry{
			"local_fast": {
				Provider: "ollama",
				Model:    "qwen2.5:7b",
				AuthMode: "none",
				Enabled:  true,
			},
			"local_code": {
				Provider: "ollama",
				Model:    "qwen2.5-coder:7b",
				AuthMode: "none",
				Enabled:  true,
			},
			"openai_api": {
				Provider:  "openai",
				Model:     "gpt-4o-mini",
				AuthMode:  "api_key",
				SecretRef: "env:OPENAI_API_KEY",
				Enabled:   true,
			},
			"openai_oauth": {
				Provider:   "openai",
				Model:      "gpt-4o-mini",
				AuthMode:   "browser_oauth",
				SessionRef: "system_keyring:openai_session",
				Enabled:    true,
			},
			"openrouter_profile": {
				Provider:  "openrouter",
				Model:     "google/gemini-2.5-flash",
				AuthMode:  "api_key",
				SecretRef: "env:OPENROUTER_API_KEY",
				Enabled:   true,
			},
		},
	}
	conf.Normalize()
	return conf
}

func (c *ProvidersConfig) Normalize() {
	if c == nil {
		return
	}
	if c.Version == 0 {
		c.Version = 2
	}
	if c.Providers == nil {
		c.Providers = map[string]ProviderEntry{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]ProfileEntry{}
	}

	// Detectar si el usuario modificó la estructura anidada o la plana en Go
	nestedChanged := c.Active.Provider != c.lastActive.Provider ||
		c.Active.Model != c.lastActive.Model ||
		c.Active.AuthProfile != c.lastActive.AuthProfile

	flatChanged := c.ActiveProvider != c.lastActive.Provider ||
		c.ActiveModel != c.lastActive.Model ||
		c.ActiveProfile != c.lastActive.AuthProfile

	if nestedChanged && !flatChanged {
		// El bloque anidado fue modificado, actualizar planos
		c.ActiveProvider = c.Active.Provider
		c.ActiveModel = c.Active.Model
		c.ActiveProfile = c.Active.AuthProfile
	} else if flatChanged {
		// Los campos planos fueron modificados (o ambos), actualizar anidado
		c.Active.Provider = c.ActiveProvider
		c.Active.Model = c.ActiveModel
		c.Active.AuthProfile = c.ActiveProfile
	}

	for name, entry := range c.Providers {
		c.Providers[name] = normalizeProviderEntry(name, entry)
	}

	// Backfill legacy provider-wide configuration into a profile when no profile exists yet.
	if len(c.Profiles) == 0 {
		for name, entry := range c.Providers {
			profileName := suggestedProfileName(name, entry)
			c.Profiles[profileName] = ProfileEntry{
				Provider:   name,
				Model:      entry.EffectiveModel(),
				AuthMode:   entry.EffectiveAuthMode(),
				SecretRef:  entry.EffectiveSecretRef(),
				SessionRef: entry.EffectiveSessionRef(),
				Enabled:    entry.Enabled,
			}
		}
	}

	selection := c.resolveActiveSelectionInternal()
	if selection.ProfileName != "" {
		c.ActiveProfile = selection.ProfileName
	}
	if selection.ProviderName != "" {
		c.ActiveProvider = selection.ProviderName
	}
	if selection.ModelID != "" {
		c.ActiveModel = selection.ModelID
	}
	if selection.AuthMode != "" {
		c.ActiveAuthMode = selection.AuthMode
	}

	// Sincronizar y guardar para la próxima llamada
	c.Active.Provider = c.ActiveProvider
	c.Active.Model = c.ActiveModel
	c.Active.AuthProfile = c.ActiveProfile
	c.lastActive = c.Active
}

func (c *ProvidersConfig) ResolveActiveSelection() ActiveSelection {
	if c == nil {
		return ActiveSelection{}
	}
	c.Normalize()
	return c.resolveActiveSelectionInternal()
}

func (c *ProvidersConfig) resolveActiveSelectionInternal() ActiveSelection {
	selection := ActiveSelection{}

	if c.ActiveProfile != "" {
		if profile, ok := c.Profiles[c.ActiveProfile]; ok && profile.Enabled {
			if c.ActiveProvider == "" || profile.Provider == c.ActiveProvider {
				if c.ActiveModel == "" || c.ActiveModel == profile.Model {
					if c.ActiveAuthMode == "" || strings.EqualFold(c.ActiveAuthMode, profile.AuthMode) {
						selection.ProfileName = c.ActiveProfile
						selection.ProviderName = profile.Provider
						selection.ModelID = profile.Model
						selection.AuthMode = profile.AuthMode
						selection.SecretRef = profile.SecretRef
						selection.SessionRef = profile.SessionRef
					}
				}
			}
		}
	}

	if selection.ProviderName == "" && c.ActiveProvider != "" {
		if provider, ok := c.Providers[c.ActiveProvider]; ok {
			selection.ProviderName = c.ActiveProvider
			selection.ModelID = c.ActiveModel
			selection.AuthMode = c.ActiveAuthMode
			selection.ProfileName = c.profileNameForProvider(c.ActiveProvider)
			if selection.ModelID == "" || (selection.ModelID == "qwen2.5:7b" && strings.ToLower(strings.TrimSpace(c.ActiveProvider)) != "ollama") {
				selection.ModelID = provider.EffectiveModel()
			}
			if selection.AuthMode == "" {
				selection.AuthMode = provider.EffectiveAuthMode()
			}
			selection.SecretRef = provider.EffectiveSecretRef()
			selection.SessionRef = provider.EffectiveSessionRef()
		}
	}

	if selection.ProviderName == "" {
		for profileName, profile := range c.Profiles {
			if !profile.Enabled {
				continue
			}
			selection.ProfileName = profileName
			selection.ProviderName = profile.Provider
			selection.ModelID = profile.Model
			selection.AuthMode = profile.AuthMode
			selection.SecretRef = profile.SecretRef
			selection.SessionRef = profile.SessionRef
			break
		}
	}

	if selection.ProviderName == "" {
		for name, provider := range c.Providers {
			if !provider.Enabled {
				continue
			}
			selection.ProviderName = name
			selection.ProfileName = c.profileNameForProvider(name)
			selection.ModelID = provider.EffectiveModel()
			selection.AuthMode = provider.EffectiveAuthMode()
			selection.SecretRef = provider.EffectiveSecretRef()
			selection.SessionRef = provider.EffectiveSessionRef()
			break
		}
	}

	if selection.ProviderName == "" {
		return selection
	}

	provider := c.Providers[selection.ProviderName]
	if selection.ModelID == "" {
		selection.ModelID = provider.EffectiveModel()
	}
	if selection.AuthMode == "" {
		selection.AuthMode = provider.EffectiveAuthMode()
	}
	if selection.SecretRef == "" {
		selection.SecretRef = provider.EffectiveSecretRef()
	}
	if selection.SessionRef == "" {
		selection.SessionRef = provider.EffectiveSessionRef()
	}
	if selection.ProfileName == "" {
		selection.ProfileName = c.profileNameForProvider(selection.ProviderName)
	}

	return selection
}

func (c *ProvidersConfig) profileNameForProvider(providerName string) string {
	if providerName == "" {
		return ""
	}
	if profile, ok := c.Profiles[providerName]; ok && profile.Enabled {
		return providerName
	}
	provider, ok := c.Providers[providerName]
	if !ok {
		return ""
	}
	if profileName := suggestedProfileName(providerName, provider); profileName != "" {
		if _, ok := c.Profiles[profileName]; ok {
			return profileName
		}
	}
	return providerName
}

func normalizeProviderEntry(name string, entry ProviderEntry) ProviderEntry {
	if entry.Type == "" {
		entry.Type = strings.ToLower(strings.TrimSpace(name))
	}
	if entry.DisplayName == "" {
		entry.DisplayName = titleize(name)
	}
	if entry.Connection.BaseURL == "" && entry.BaseURL != "" {
		entry.Connection.BaseURL = entry.BaseURL
	}
	if entry.BaseURL == "" && entry.Connection.BaseURL != "" {
		entry.BaseURL = entry.Connection.BaseURL
	}
	if entry.AuthMode == "" {
		entry.AuthMode = inferAuthMode(entry)
	}
	if entry.AuthModes == nil {
		entry.AuthModes = map[string]AuthModeEntry{}
	}
	if len(entry.Capabilities) > 0 {
		for _, cap := range entry.Capabilities {
			mode := cap.AuthMode
			if _, ok := entry.AuthModes[mode]; !ok {
				entry.AuthModes[mode] = AuthModeEntry{
					Type:       mode,
					Enabled:    true,
					SecretRef:  entry.SecretRef,
					SessionRef: entry.SessionRef,
				}
			}
		}
	}
	if entry.AuthMode != "" {
		mode := strings.ToLower(strings.TrimSpace(entry.AuthMode))
		if _, ok := entry.AuthModes[mode]; !ok {
			entry.AuthModes[mode] = AuthModeEntry{
				Type:       mode,
				Enabled:    true,
				SecretRef:  entry.SecretRef,
				SessionRef: entry.SessionRef,
			}
		}
	}
	if entry.DefaultModel == "" {
		entry.DefaultModel = entry.Model
	}
	if entry.Model == "" {
		entry.Model = entry.DefaultModel
	}
	if entry.Models == nil {
		entry.Models = map[string]ModelEntry{}
	}
	if len(entry.Models) == 0 && entry.Model != "" {
		entry.Models[entry.Model] = ModelEntry{
			ID:           entry.Model,
			Label:        entry.Model,
			Enabled:      true,
			Default:      true,
			Capabilities: defaultCapabilitiesForProvider(entry.Type),
		}
	}
	if entry.DefaultModel == "" {
		entry.DefaultModel = entry.EffectiveModel()
	}
	if entry.Discovery.Strategy == "" {
		entry.Discovery.Strategy = defaultDiscoveryStrategy(entry.Type)
	}
	if entry.Discovery.CacheTTLSeconds == 0 {
		switch strings.ToLower(strings.TrimSpace(entry.Type)) {
		case "ollama":
			entry.Discovery.CacheTTLSeconds = 60
		case "openai":
			entry.Discovery.CacheTTLSeconds = 3600
		default:
			entry.Discovery.CacheTTLSeconds = 3600
		}
	}
	return entry
}

func defaultCapabilitiesForProvider(providerType string) llm.ModelCapabilities {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai":
		return llm.ModelCapabilities{ToolCalling: true, Streaming: true, Vision: true, ConversationState: true}
	default:
		return llm.ModelCapabilities{ToolCalling: true, Streaming: true, Vision: false, ConversationState: true}
	}
}

func defaultDiscoveryStrategy(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "ollama":
		return "ollama_api"
	case "openai":
		return "openai_models_api"
	default:
		return "compatible_models_api"
	}
}

func inferAuthMode(entry ProviderEntry) string {
	if entry.SecretRef != "" {
		return "api_key"
	}
	if entry.SessionRef != "" {
		return "browser_login"
	}
	if entry.Type == "openai" {
		return "api_key"
	}
	return "none"
}

func suggestedProfileName(providerName string, entry ProviderEntry) string {
	switch strings.ToLower(strings.TrimSpace(entry.Type)) {
	case "ollama":
		return "local_fast"
	case "openai":
		return "openai_api"
	case "compatible":
		return "compatible"
	default:
		if providerName != "" {
			return providerName
		}
		return ""
	}
}

func titleize(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func (p ProviderEntry) EffectiveBaseURL() string {
	if strings.TrimSpace(p.Connection.BaseURL) != "" {
		return strings.TrimSpace(p.Connection.BaseURL)
	}
	return strings.TrimSpace(p.BaseURL)
}

func (p ProviderEntry) EffectiveModel() string {
	if strings.TrimSpace(p.DefaultModel) != "" {
		return strings.TrimSpace(p.DefaultModel)
	}
	if strings.TrimSpace(p.Model) != "" {
		return strings.TrimSpace(p.Model)
	}
	if len(p.Models) == 0 {
		return ""
	}
	keys := make([]string, 0, len(p.Models))
	for key, model := range p.Models {
		if model.Enabled || model.Default {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		for key := range p.Models {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys[0]
}

func (p ProviderEntry) EffectiveAuthMode() string {
	if strings.TrimSpace(p.AuthMode) != "" {
		return strings.ToLower(strings.TrimSpace(p.AuthMode))
	}
	return inferAuthMode(p)
}

func (p ProviderEntry) EffectiveSecretRef() string {
	if strings.TrimSpace(p.SecretRef) != "" {
		return strings.TrimSpace(p.SecretRef)
	}
	mode := p.EffectiveAuthMode()
	if modeEntry, ok := p.AuthModes[mode]; ok && strings.TrimSpace(modeEntry.SecretRef) != "" {
		return strings.TrimSpace(modeEntry.SecretRef)
	}
	return ""
}

func (p ProviderEntry) EffectiveSessionRef() string {
	if strings.TrimSpace(p.SessionRef) != "" {
		return strings.TrimSpace(p.SessionRef)
	}
	mode := p.EffectiveAuthMode()
	if modeEntry, ok := p.AuthModes[mode]; ok && strings.TrimSpace(modeEntry.SessionRef) != "" {
		return strings.TrimSpace(modeEntry.SessionRef)
	}
	return ""
}

func SaveProvidersConfig(path string, conf *ProvidersConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if conf != nil {
		conf.Normalize()
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
	conf.Normalize()
	return conf, nil
}

func mustHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
