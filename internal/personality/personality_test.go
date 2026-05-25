package personality

import (
	"errors"
	"strings"
	"testing"
)

func TestComposeResponse(t *testing.T) {
	tests := []struct {
		name     string
		ctx      ResponseContext
		expected []string // Uno de estos strings debe ser retornado o contener el substring
	}{
		{
			name: "High risk confirming",
			ctx: ResponseContext{
				State: StateConfirming,
				Risk:  RiskHigh,
			},
			expected: []string{"No continuaré sin confirmación. Esta acción puede modificar elementos importantes."},
		},
		{
			name: "Ambiguous request",
			ctx: ResponseContext{
				State:     StateObserving,
				Ambiguous: true,
			},
			expected: []string{"Encontré varias posibilidades. Te mostraré candidatos antes de actuar."},
		},
		{
			name: "State Observing",
			ctx: ResponseContext{
				State: StateObserving,
			},
			expected: []string{"Estoy revisando el entorno."},
		},
		{
			name: "State Error with specific error",
			ctx: ResponseContext{
				State: StateError,
				Error: errors.New("denegado por política"),
			},
			expected: []string{"He encontrado un inconveniente: denegado por política"},
		},
		{
			name: "State Done for App Open",
			ctx: ResponseContext{
				State:    StateDone,
				ToolName: "desktop.open_app",
				Target:   "firefox",
			},
			expected: []string{"Listo. Aplicación firefox lanzada."},
		},
		{
			name: "State Done generic",
			ctx: ResponseContext{
				State: StateDone,
			},
			expected: []string{
				"Hecho.",
				"Todo quedó en orden.",
				"Operación completada.",
				"Listo.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComposeResponse(tt.ctx)
			
			match := false
			for _, exp := range tt.expected {
				if strings.Contains(result, exp) {
					match = true
					break
				}
			}
			
			if !match {
				t.Errorf("ComposeResponse() returned %q, but expected one of %v", result, tt.expected)
			}
		})
	}
}

func TestFormatTarget(t *testing.T) {
	res := formatTarget("code")
	if res != "VS Code" {
		t.Errorf("Expected 'VS Code', got %s", res)
	}

	res2 := formatTarget("/usr/bin/nautilus")
	if res2 != "/usr/bin/nautilus" {
		t.Errorf("Expected unchanged target, got %s", res2)
	}
}
