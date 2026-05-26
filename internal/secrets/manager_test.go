package secrets

import (
	"strings"
	"testing"
)

func TestManagerResolveEnvSecret(t *testing.T) {
	t.Setenv("RBOT_TEST_SECRET", "secret-value")
	mgr := NewManager()

	value, err := mgr.Resolve("env:RBOT_TEST_SECRET")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if value != "secret-value" {
		t.Fatalf("expected secret-value, got %q", value)
	}
}

func TestManagerResolveMissingEnvSecret(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.Resolve("env:RBOT_MISSING_SECRET")
	if err == nil {
		t.Fatal("expected missing environment secret error")
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaked secret value: %v", err)
	}
}

func TestRedactDoesNotExposeRawSecret(t *testing.T) {
	if got := Redact("raw-secret"); got != "<redacted>" {
		t.Fatalf("expected raw value redacted, got %q", got)
	}
	if got := Redact("env:OPENAI_API_KEY"); got != "env:OPENAI_API_KEY" {
		t.Fatalf("expected env reference preserved, got %q", got)
	}
}
