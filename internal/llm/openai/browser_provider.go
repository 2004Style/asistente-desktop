package openai

import (
	"context"
	"fmt"
	"rbot/internal/llm"
)

// BrowserProvider representa un proveedor de OpenAI que simula el uso mediante cuenta/suscripción
// a través del navegador o una sesión guardada, en lugar de una API Key de desarrollador.
type BrowserProvider struct {
	name       string
	model      string
	sessionRef string
}

// NewBrowserProvider crea un nuevo BrowserProvider.
func NewBrowserProvider(name, model, sessionRef string) *BrowserProvider {
	if model == "" {
		model = "auto"
	}
	return &BrowserProvider{
		name:       name,
		model:      model,
		sessionRef: sessionRef,
	}
}

// Name devuelve el identificador de este proveedor.
func (p *BrowserProvider) Name() string { return p.name }

// ModelID devuelve el identificador del modelo activo.
func (p *BrowserProvider) ModelID() string { return p.model }

// SetModel cambia el modelo activo.
func (p *BrowserProvider) SetModel(modelID string) { p.model = modelID }

// Chat devuelve un error indicando las limitaciones de automatizar la sesión debido a Cloudflare.
func (p *BrowserProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	if p.sessionRef == "" {
		return nil, fmt.Errorf("el modo browser_login requiere ingresar tu token de sesión de ChatGPT (__Secure-next-auth.session-token). Abre Ajustes en la UI e ingrésalo.")
	}
	return nil, fmt.Errorf("se cargó tu Session Token de ChatGPT (longitud: %d bytes). Sin embargo, el acceso web automatizado de ChatGPT está protegido contra bots por Cloudflare Turnstile. Se recomienda cambiar al modo de autenticación por API KEY para interactuar sin restricciones.", len(p.sessionRef))
}

// ListModels lista los modelos virtuales disponibles para este modo.
func (p *BrowserProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{
		{
			ID:       "auto",
			Name:     "Modelo Automático (Suscripción)",
			Provider: p.name,
			Capabilities: llm.ModelCapabilities{
				ToolCalling:       true,
				Streaming:         true,
				Vision:            true,
				ConversationState: true,
			},
		},
	}, nil
}

// Ping devuelve un error informando sobre el estado de la sesión.
func (p *BrowserProvider) Ping(ctx context.Context) error {
	if p.sessionRef == "" {
		return fmt.Errorf("el modo browser_login no contiene ningún session token guardado")
	}
	return fmt.Errorf("session token leído con éxito (longitud: %d bytes), pero el acceso requiere API KEY debido a protecciones de red", len(p.sessionRef))
}
