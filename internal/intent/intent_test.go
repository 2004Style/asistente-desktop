package intent_test

import (
	"rbot/internal/intent"
	"testing"
)

func TestNormalize(t *testing.T) {
	wakeWords := []string{"oye ronaldo", "ey ronaldo", "rbot"}
	
	tests := []struct {
		input    string
		expected string
	}{
		{"Oye Ronaldo abre bray", "abre brave"},
		{"ey ronaldo preciona ele", "presiona l"},
		{"Rbot colcame musica", "colócame musica"},
		{"  gracias por ver   ", ""}, // Whisper hallucination should be empty or handled
		{"ronal abre bray", "ronal abre brave"}, // "ronal" is not in wakeWords but fonetic bray -> brave is checked
		{"Habré el archivo", "abre el archivo"},
	}

	for _, tc := range tests {
		got := intent.Normalize(tc.input, wakeWords)
		// Si es una alucinación descartable
		if intent.IsWhisperHallucination(tc.input) {
			got = ""
		}
		if got != tc.expected {
			t.Errorf("Normalize(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSplitMultiIntent(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"abre brave y busca documentación de go", []string{"abre brave", "busca documentación de go"}},
		{"busca música de rock y metal", []string{"busca música de rock y metal"}}, // "metal" is not an action verb
		{"lee el archivo y luego elimina la carpeta build", []string{"lee el archivo", "elimina la carpeta build"}},
		{"corre go test, crea el archivo main.go", []string{"corre go test", "crea el archivo main.go"}},
	}

	for _, tc := range tests {
		got := intent.SplitMultiIntent(tc.input)
		if len(got) != len(tc.expected) {
			t.Fatalf("SplitMultiIntent(%q) returned %d parts, want %d: %v", tc.input, len(got), len(tc.expected), got)
		}
		for i, part := range got {
			if part != tc.expected[i] {
				t.Errorf("SplitMultiIntent(%q)[%d] = %q; want %q", tc.input, i, part, tc.expected[i])
			}
		}
	}
}

func TestSlotsExtraction(t *testing.T) {
	tests := []struct {
		intentName string
		input      string
		slotKey    string
		expected   interface{}
	}{
		{"browser-open", "abre brave y navega a google.com", "app", "brave"},
		{"browser-open", "navega a google.com", "url", "google.com"},
		{"file-reader", "lee el archivo README.md", "file_name", "README.md"},
		{"system-run", "ejecuta go test", "command", "go test"},
		{"media", "reproduce música de linkin park", "query", "linkin park"},
		{"input", "presiona ctrl+c", "keys", "ctrl+c"},
		{"input", "escribe hola mundo", "text", "hola mundo"},
	}

	for _, tc := range tests {
		slots := intent.ExtractSlots(tc.intentName, tc.input)
		val, exists := slots[tc.slotKey]
		if !exists {
			t.Errorf("For input %q, slot %q not found", tc.input, tc.slotKey)
			continue
		}
		if val != tc.expected {
			t.Errorf("For input %q, slot %q = %v; want %v", tc.input, tc.slotKey, val, tc.expected)
		}
	}
}
