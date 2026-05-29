package llm

import (
	"context"
	"fmt"
)

// ProviderAdapter adapta cualquier motor de inferencia (Ollama, OpenAI, compatible)
// basándose en sus capacidades y runtime configurados.
type ProviderAdapter struct {
	ProviderName string
	ActiveModel  string
	AuthMode     string
	BillingMode  string
	RuntimeMode  string
	Delegate     Provider
}

// Name devuelve el identificador de este proveedor.
func (a *ProviderAdapter) Name() string {
	return a.ProviderName
}

// ModelID devuelve el identificador del modelo activo.
func (a *ProviderAdapter) ModelID() string {
	return a.ActiveModel
}

// SetModel cambia el modelo activo.
func (a *ProviderAdapter) SetModel(modelID string) {
	a.ActiveModel = modelID
	if a.Delegate != nil {
		a.Delegate.SetModel(modelID)
	}
}

// Chat delega la ejecución al proveedor subyacente.
func (a *ProviderAdapter) Chat(ctx context.Context, messages []Message, tools []Tool, opts ChatOptions) (*Message, error) {
	if a.Delegate == nil {
		return nil, fmt.Errorf("proveedor %s no inicializado correctamente (el delegado es nulo)", a.ProviderName)
	}
	return a.Delegate.Chat(ctx, messages, tools, opts)
}

// ListModels delega la obtención de modelos al proveedor subyacente.
func (a *ProviderAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if a.Delegate == nil {
		return nil, fmt.Errorf("proveedor %s no inicializado correctamente (el delegado es nulo)", a.ProviderName)
	}
	return a.Delegate.ListModels(ctx)
}

// Ping delega la verificación al proveedor subyacente.
func (a *ProviderAdapter) Ping(ctx context.Context) error {
	if a.Delegate == nil {
		return fmt.Errorf("proveedor %s no inicializado correctamente (el delegado es nulo)", a.ProviderName)
	}
	return a.Delegate.Ping(ctx)
}
