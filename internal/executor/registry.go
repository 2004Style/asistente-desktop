package executor

import (
	"context"
	"fmt"

	"rbot/internal/ollama"
)

type ToolResult struct {
	Output string
	Error  error
}

type ToolHandler interface {
	Name() string
	Definition() ollama.Tool
	Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error)
	Risk(args map[string]interface{}) string // Retorna 'low', 'medium', 'high', 'critical'
}

type Registry struct {
	tools map[string]ToolHandler
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolHandler),
	}
}

func (r *Registry) Register(tool ToolHandler) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (ToolHandler, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) GetAllDefinitions() []ollama.Tool {
	var defs []ollama.Tool
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return ToolResult{}, fmt.Errorf("tool desconocida: %s", name)
	}
	return tool.Execute(ctx, args)
}
