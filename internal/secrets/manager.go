package secrets

import (
	"fmt"
	"strings"
)

type Resolver interface {
	Resolve(ref string) (string, error)
}

type Manager struct {
	resolvers map[string]Resolver
}

func NewManager() *Manager {
	m := &Manager{resolvers: map[string]Resolver{}}
	m.Register("env", EnvResolver{})
	return m
}

func (m *Manager) Register(scheme string, resolver Resolver) {
	if m.resolvers == nil {
		m.resolvers = map[string]Resolver{}
	}
	m.resolvers[strings.ToLower(strings.TrimSpace(scheme))] = resolver
}

func (m *Manager) Resolve(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid secret reference %q; expected scheme:name", Redact(ref))
	}
	resolver, ok := m.resolvers[strings.ToLower(parts[0])]
	if !ok {
		return "", fmt.Errorf("unsupported secret reference scheme %q", parts[0])
	}
	return resolver.Resolve(parts[1])
}

func Redact(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "env:") {
		return value
	}
	return "<redacted>"
}
