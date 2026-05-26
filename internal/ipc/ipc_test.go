package ipc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rbot/internal/runtime"
)

func TestIPCServerCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbot-ipc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "rbot.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(req RequestJSONRPC) ResponseJSONRPC {
		if req.Method == "ping" {
			return NewResponseJSONRPC(req.ID, "pong")
		}
		return NewErrorResponseJSONRPC(req.ID, MethodNotFoundCode, "Method not found", nil)
	}

	err = StartIPCServer(ctx, sockPath, handler)
	if err != nil {
		t.Fatalf("Failed to start IPC server: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verificar que el archivo del socket tiene permisos estrictos (0600)
	sockInfo, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("Failed to stat socket file: %v", err)
	}
	mode := sockInfo.Mode()
	if mode&0077 != 0 {
		t.Errorf("Unexpected socket permissions: %v (expected owner-only)", mode)
	}

	resp, err := SendCommandRPC(sockPath, "ping", nil, "req-1")
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}

	if resp.JSONRPC != "2.0" || resp.ID != "req-1" || resp.Result != "pong" {
		t.Errorf("Unexpected response: %+v", resp)
	}
}

func TestIPCServerEventsNDJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rbot-ipc-events-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "events.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := runtime.NewEventBus()

	err = StartEventIPCServer(ctx, sockPath, bus)
	if err != nil {
		t.Fatalf("Failed to start event IPC server: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("Failed to dial event socket: %v", err)
	}
	defer conn.Close()

	// Dar tiempo al servidor para registrar la suscripción
	time.Sleep(50 * time.Millisecond)

	bus.Publish(runtime.Event{
		Type:    "test.ndjson.event",
		Payload: map[string]interface{}{"data": "value"},
	})

	decoder := json.NewDecoder(conn)
	var ev runtime.Event
	err = decoder.Decode(&ev)
	if err != nil {
		t.Fatalf("Failed to decode NDJSON event: %v", err)
	}

	if ev.Type != "test.ndjson.event" || ev.Payload["data"].(string) != "value" {
		t.Errorf("Unexpected event received: %+v", ev)
	}
}
