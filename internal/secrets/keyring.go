package secrets

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

// KeyringResolver resuelve secretos usando el almacén de credenciales del sistema.
// Referencia: keyring:<name>
// name puede incluir un separador '/' para designar service/name; por defecto service='rbot'.
type KeyringResolver struct{}

func (KeyringResolver) Resolve(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty keyring reference")
	}
	// Allow either name or service/name
	service := "rbot"
	name := ref
	if strings.Contains(ref, "/") {
		parts := strings.SplitN(ref, "/", 2)
		service = parts[0]
		name = parts[1]
	}
	val, err := keyring.Get(service, name)
	if err != nil {
		return "", fmt.Errorf("keyring resolver: %w", err)
	}
	if val == "" {
		return "", fmt.Errorf("keyring: empty value for %s/%s", service, name)
	}
	return val, nil
}
