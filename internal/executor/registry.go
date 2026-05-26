package executor

import (
	"fmt"
	"sync"

	"rbot/internal/llm"
)

// Registry mantiene un mapa seguro ante accesos concurrentes de todas las herramientas disponibles.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]ToolHandler
}

// NewRegistry crea e inicializa un nuevo registro de herramientas.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolHandler),
	}
}

// Register almacena una nueva herramienta. Retorna error si ya existe otra con el mismo nombre.
func (r *Registry) Register(tool ToolHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("intento de registrar una herramienta sin nombre")
	}

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("herramienta ya registrada previamente: %s", name)
	}

	r.tools[name] = tool
	return nil
}

// RegisterOrReplace registra una herramienta, sobrescribiéndola si ya existe.
func (r *Registry) RegisterOrReplace(tool ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Name()
	if name != "" {
		r.tools[name] = tool
	}
}

// Get recupera una herramienta por su nombre registrado.
func (r *Registry) Get(name string) (ToolHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List devuelve una copia plana de todas las herramientas registradas.
func (r *Registry) List() []ToolHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ToolHandler, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, tool)
	}
	return out
}

// GetOllamaTools mantiene compatibilidad con código existente, delegando a GetLLMTools.
// Deprecated: usar GetLLMTools en su lugar.
func (r *Registry) GetOllamaTools() []llm.Tool {
	return r.GetLLMTools()
}

// GetLLMTools convierte las herramientas registradas a definiciones compatibles con LLM Tool Calling.
func (r *Registry) GetLLMTools() []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []llm.Tool
	for _, tool := range r.tools {
		params := llm.Parameters{
			Type:       "object",
			Properties: make(map[string]interface{}),
		}

		schema := tool.Schema()
		if schema != nil {
			if t, ok := schema["type"].(string); ok {
				params.Type = t
			}
			if props, ok := schema["properties"].(map[string]interface{}); ok {
				params.Properties = props
			}
			if req, ok := schema["required"].([]string); ok {
				params.Required = req
			}
			// En caso de que required venga como []interface{} (debido a parsing)
			if req, ok := schema["required"].([]interface{}); ok {
				var reqStrings []string
				for _, rItem := range req {
					if rStr, ok := rItem.(string); ok {
						reqStrings = append(reqStrings, rStr)
					}
				}
				params.Required = reqStrings
			}
		}

		out = append(out, llm.Tool{
			Type: "function",
			Function: llm.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}
	return out
}

