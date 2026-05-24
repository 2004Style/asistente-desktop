package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-mcp", "echo", []string{"hello", "world"})

	if client.Name != "test-mcp" {
		t.Errorf("Expected Client Name to be 'test-mcp', got %q", client.Name)
	}

	if client.Command != "echo" {
		t.Errorf("Expected Client Command to be 'echo', got %q", client.Command)
	}

	if len(client.Args) != 2 || client.Args[0] != "hello" || client.Args[1] != "world" {
		t.Errorf("Expected Client Args to be ['hello', 'world'], got %v", client.Args)
	}
}

func TestServerManager_Bootstrap(t *testing.T) {
	// Create a temporary configuration JSON
	tempDir, err := os.MkdirTemp("", "rbot-mcp-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Usamos un comando interactivo bash para responder al handshake JSON-RPC inmediatamente
	configJSON := `{
		"mcpServers": {
			"dummy-server": {
				"command": "bash",
				"args": [
					"-c", 
					"read -r line; echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"mock-server\",\"version\":\"1.0.0\"}}}'; while true; do sleep 1; done", 
					"~/Documentos"
				]
			}
		}
	}`
	configPath := filepath.Join(tempDir, "mcp_config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write mock config: %v", err)
	}

	manager := NewServerManager()
	defer manager.CloseAll()

	// Bootstrap the manager (esto levantará el mock-server que responde al instante)
	err = manager.Bootstrap(configPath)
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	// Verify client registration
	client, exists := manager.Clients["dummy-server"]
	if !exists {
		t.Fatalf("Expected 'dummy-server' client to be registered in manager")
	}

	// Verify that path expansion was applied to args (e.g. ~/Documentos expanded to user's home Documents folder)
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, "Documentos")
	if len(client.Args) != 3 || client.Args[2] != expectedPath {
		t.Errorf("Expected expanded arg at index 2 to be %q, got %q", expectedPath, client.Args[2])
	}
}

func TestClient_Lifecycle(t *testing.T) {
	// Iniciamos un proceso bash que responde inmediatamente al handshake
	client := NewClient(
		"mock-bash",
		"bash",
		[]string{
			"-c",
			"read -r line; echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"mock-server\",\"version\":\"1.0.0\"}}}'; while true; do sleep 1; done",
		},
	)
	
	err := client.Start()
	if err != nil {
		t.Fatalf("Client failed to start: %v", err)
	}

	if !client.IsActive {
		t.Errorf("Expected client to be active after start")
	}

	// Close immediately and assert it stops cleanly and in non-blocking fashion
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stopChan := make(chan struct{})
	go func() {
		client.Stop()
		close(stopChan)
	}()

	select {
	case <-stopChan:
		// Stopped successfully in time
	case <-ctx.Done():
		t.Errorf("Client Stop() blocked and timed out")
	}

	if client.IsActive {
		t.Errorf("Expected client to be inactive after Stop()")
	}
}
