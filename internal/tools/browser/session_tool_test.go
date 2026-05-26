package browser

import (
	"context"
	"testing"
)

func TestYouTubeOpenOrReuseTool_Reuse(t *testing.T) {
	// Mock: Simular que hay una ventana de YouTube abierta
	mockListWindows := func(ctx context.Context) ([]WindowInfoBasic, error) {
		return []WindowInfoBasic{
			{
				ID:      "0x123456",
				Address: "0x123456",
				Title:   "Lo-Fi Beats for Coding - YouTube",
				Class:   "Brave-browser",
				Focused: false,
			},
		}, nil
	}

	focusCalled := false
	focusedAddress := ""
	mockFocusWindow := func(ctx context.Context, address string) error {
		focusCalled = true
		focusedAddress = address
		return nil
	}

	tool := NewYouTubeOpenOrReuseTool(mockListWindows, mockFocusWindow)

	// Ejecutar sin query
	args := map[string]interface{}{}
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Unexpected error executing tool: %v", err)
	}

	if !res.Success {
		t.Errorf("Expected execution to be successful")
	}

	reused, ok := res.Data["reused"].(bool)
	if !ok || !reused {
		t.Errorf("Expected reused to be true, got %v", res.Data["reused"])
	}

	if !focusCalled {
		t.Errorf("Expected focusWindow callback to be called")
	}

	if focusedAddress != "0x123456" {
		t.Errorf("Expected focusWindow to be called with '0x123456', got %q", focusedAddress)
	}
}

func TestYouTubeOpenOrReuseTool_NoReuse(t *testing.T) {
	// Mock: Simular que no hay ninguna ventana abierta de YouTube, solo otra cosa
	mockListWindows := func(ctx context.Context) ([]WindowInfoBasic, error) {
		return []WindowInfoBasic{
			{
				ID:      "0x99999",
				Address: "0x99999",
				Title:   "Go Documentation",
				Class:   "firefox",
				Focused: true,
			},
		}, nil
	}

	focusCalled := false
	mockFocusWindow := func(ctx context.Context, address string) error {
		focusCalled = true
		return nil
	}

	tool := NewYouTubeOpenOrReuseTool(mockListWindows, mockFocusWindow)

	// Ejecutar con query (intentará abrir un navegador de verdad mediante desktop.OpenURL)
	// Si falla porque no hay navegador ni GUI en el entorno de pruebas, es aceptable, 
	// pero nos aseguramos de que focusWindow nunca se llamó (es decir, no reutilizó).
	args := map[string]interface{}{"query": "chill beats"}
	res, err := tool.Execute(context.Background(), args)
	
	if focusCalled {
		t.Errorf("Expected focusWindow not to be called since no YouTube window exists")
	}

	if err != nil {
		// El error de no tener navegador/GUI es esperable en entornos headless de CI/CD
		t.Log("Execution failed as expected (no browser/GUI):", err)
		return
	}

	if res != nil {
		reused, ok := res.Data["reused"].(bool)
		if ok && reused {
			t.Errorf("Expected reused to be false, got true")
		}
	}
}
