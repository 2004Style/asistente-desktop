package llm

import (
	"context"
	"time"
)

// Message representa un mensaje genérico de conversación, agnóstico del proveedor.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall describe una invocación de herramienta solicitada por el modelo.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall describe la función específica a invocar y sus argumentos.
type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Tool describe una herramienta disponible para que el modelo la invoque.
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition describe el esquema de una función invocable.
type FunctionDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

// Parameters describe los parámetros de entrada de una función.
type Parameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// ModelInfo contiene metadatos sobre un modelo disponible.
type ModelInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Provider     string            `json:"provider"`
	Size         string            `json:"size,omitempty"`
	Family       string            `json:"family,omitempty"`
	Capabilities ModelCapabilities `json:"capabilities"`
	ModifiedAt   time.Time         `json:"modified_at,omitempty"`
}

// ModelCapabilities describe qué funcionalidades soporta un modelo.
type ModelCapabilities struct {
	ToolCalling       bool `json:"tool_calling"`
	Streaming         bool `json:"streaming"`
	Vision            bool `json:"vision"`
	ConversationState bool `json:"conversation_state"`
}

// ChatOptions permite configurar opciones por llamada al proveedor.
type ChatOptions struct {
	Temperature float64
	MaxTokens   int
	Stream      bool
	OnTextChunk func(string) // Callback para recibir chunks de texto en streaming.
}

// ProviderConfig describe la configuración necesaria para instanciar un proveedor.
type ProviderConfig struct {
	Name          string `json:"name"`
	Type          string `json:"type"` // "ollama", "openai", "compatible"
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key,omitempty"`
	Model         string `json:"model"`
	ActiveProfile string `json:"active_profile,omitempty"`
	IsActive      bool   `json:"is_active"`
}

// Provider define la interfaz que todo proveedor de LLM debe implementar.
type Provider interface {
	// Name devuelve el identificador del proveedor (ej: "ollama", "openai").
	Name() string

	// Chat envía un conjunto de mensajes y herramientas al modelo y retorna la respuesta.
	Chat(ctx context.Context, messages []Message, tools []Tool, opts ChatOptions) (*Message, error)

	// ListModels devuelve la lista de modelos disponibles en el proveedor.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Ping verifica que el proveedor está accesible y operativo.
	Ping(ctx context.Context) error

	// ModelID devuelve el modelo actualmente configurado.
	ModelID() string

	// SetModel cambia el modelo activo en caliente.
	SetModel(modelID string)
}

// ProviderCapability define de forma explícita las capacidades y métodos de autenticación de un proveedor.
type ProviderCapability struct {
	AuthMode      string   `json:"auth_mode" yaml:"auth_mode"`
	BillingMode   string   `json:"billing_mode" yaml:"billing_mode"`
	RuntimeMode   string   `json:"runtime_mode" yaml:"runtime_mode"`
	Usage         string   `json:"usage,omitempty" yaml:"usage,omitempty"`
	EnvKey        string   `json:"env_key,omitempty" yaml:"env_key,omitempty"`
	BaseURL       string   `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Header        string   `json:"header,omitempty" yaml:"header,omitempty"`
	Compatibility []string `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
}
