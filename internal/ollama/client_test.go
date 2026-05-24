package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c1 := NewClient("", "llama3")
	if c1.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected default BaseURL to be http://localhost:11434, got %s", c1.BaseURL)
	}
	if c1.Model != "llama3" {
		t.Errorf("Expected model to be llama3, got %s", c1.Model)
	}

	c2 := NewClient("http://other:11434", "mistral")
	if c2.BaseURL != "http://other:11434" {
		t.Errorf("Expected BaseURL http://other:11434, got %s", c2.BaseURL)
	}
}

func TestChatNoStream(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected path /api/chat, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Decode request to verify body
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model test-model, got %s", req.Model)
		}
		if req.Stream {
			t.Errorf("Expected stream false, got true")
		}

		// Respond with normal ChatResponse
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := ChatResponse{
			Model: "test-model",
			Message: Message{
				Role:    "assistant",
				Content: "Hello! How can I help you today?",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-model")
	messages := []Message{{Role: "user", Content: "Hi"}}
	msg, err := c.Chat(messages, nil)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if msg.Role != "assistant" || msg.Content != "Hello! How can I help you today?" {
		t.Errorf("Unexpected message returned: %+v", msg)
	}
}

func TestChatStream(t *testing.T) {
	// Create mock server for streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"model":"test-model","message":{"role":"assistant","content":"Hel"},"done":false}`,
			`{"model":"test-model","message":{"role":"assistant","content":"lo "},"done":false}`,
			`{"model":"test-model","message":{"role":"assistant","content":"world!"},"done":true}`,
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk + "\n"))
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-model")
	
	var receivedChunks []string
	c.OnTextChunk = func(chunk string) {
		receivedChunks = append(receivedChunks, chunk)
	}

	messages := []Message{{Role: "user", Content: "Hi"}}
	msg, err := c.Chat(messages, nil)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if msg.Content != "Hello world!" {
		t.Errorf("Expected final content to be 'Hello world!', got '%s'", msg.Content)
	}

	joinedChunks := strings.Join(receivedChunks, "")
	if joinedChunks != "Hello world!" {
		t.Errorf("Expected combined text chunks to be 'Hello world!', got '%s'", joinedChunks)
	}
	if len(receivedChunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(receivedChunks))
	}
}

func TestChatErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Model not found"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-model")
	messages := []Message{{Role: "user", Content: "Hi"}}
	_, err := c.Chat(messages, nil)
	if err == nil {
		t.Errorf("Expected error for HTTP 500 status code, got nil")
	} else if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("Expected error to contain status 500, got: %v", err)
	}
}
