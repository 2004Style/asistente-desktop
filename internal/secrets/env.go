package secrets

import (
	"fmt"
	"os"
	"strings"
)

type EnvResolver struct{}

func (EnvResolver) Resolve(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty environment secret name")
	}
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", fmt.Errorf("environment secret %s is not set", name)
	}
	return value, nil
}
