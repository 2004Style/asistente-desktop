package llm

import (
	"fmt"
	"sync"
)

// Registry mantiene un registro en memoria de todos los proveedores LLM disponibles.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry crea un nuevo registro vacío de proveedores.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register añade un nuevo proveedor al registro. Retorna error si ya existe.
func (r *Registry) Register(p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if name == "" {
		return fmt.Errorf("intento de registrar un proveedor sin nombre")
	}

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("proveedor ya registrado: %s", name)
	}

	r.providers[name] = p
	return nil
}

// RegisterOrReplace registra un proveedor, reemplazándolo si ya existe.
func (r *Registry) RegisterOrReplace(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if name != "" {
		r.providers[name] = p
	}
}

// Get devuelve el proveedor con el nombre dado, si existe.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	return p, ok
}

// List devuelve una copia de todos los proveedores registrados.
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}

// Names devuelve los nombres de todos los proveedores registrados.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.providers))
	for name := range r.providers {
		out = append(out, name)
	}
	return out
}

// Remove elimina un proveedor del registro por su nombre.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}
