package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"rbot/internal/db"
	"rbot/internal/mcp"
	"rbot/internal/ollama"
	"strings"
	"testing"
)

func TestOrchestratorBuildSystemPrompt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-orchestrator-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	// Insert memories, apps, and skills into db
	_, _ = database.Exec("INSERT INTO user_memory (key, value, category) VALUES ('nombre', 'Juan', 'personal')")
	_, _ = database.Exec("INSERT INTO app_launchers (name, display_name, executable, command, is_available) VALUES ('firefox', 'Firefox Web Browser', 'firefox', 'firefox', 1)")
	_, _ = database.Exec("INSERT INTO skills (name, description, path, skill_md_path, enabled) VALUES ('music-skill', 'Reproducir musica', 'path', 'path_md', 1)")

	o := NewOrchestrator(database, nil, mcp.NewServerManager(), nil, nil, "RBot")
	prompt := o.BuildSystemPrompt([]string{"Contexto especial"})

	if !strings.Contains(prompt, "Juan") {
		t.Errorf("Expected prompt to contain remembered user name 'Juan', got: %s", prompt)
	}
	if !strings.Contains(prompt, "Firefox") {
		t.Errorf("Expected prompt to contain app 'Firefox', got: %s", prompt)
	}
	if !strings.Contains(prompt, "Contexto especial") {
		t.Errorf("Expected prompt to contain skill context, got: %s", prompt)
	}
}

func TestOrchestratorGetAvailableTools(t *testing.T) {
	o := NewOrchestrator(nil, nil, mcp.NewServerManager(), nil, nil, "RBot")
	tools := o.GetAvailableTools(context.Background())

	foundOpenApp := false
	foundReadFile := false
	for _, tool := range tools {
		if tool.Function.Name == "desktop.open_app" {
			foundOpenApp = true
		}
		if tool.Function.Name == "files.read_file" {
			foundReadFile = true
		}
	}

	if !foundOpenApp {
		t.Errorf("Expected tools to include 'desktop.open_app'")
	}
	if !foundReadFile {
		t.Errorf("Expected tools to include 'files.read_file'")
	}
}

func TestOrchestratorDetectDirectIntents(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-orchestrator-detect")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	// Insert app firefox to db so findBestAppMatch works
	_, _ = database.Exec("INSERT INTO app_launchers (name, display_name, executable, command, is_available) VALUES ('firefox', 'Firefox Web Browser', 'firefox', 'firefox', 1)")

	o := NewOrchestrator(database, nil, mcp.NewServerManager(), nil, nil, "RBot")

	home, _ := os.UserHomeDir()
	expectedDescargasPath := "descargas"
	if home != "" {
		expectedDescargasPath = filepath.Join(home, "Descargas")
	}

	tests := []struct {
		input             string
		expectedToolName  string
		expectedArgValue  string
		expectedArgName   string
		shouldReturnEmpty bool
	}{
		{
			input:            "abre firefox",
			expectedToolName: "desktop.open_app",
			expectedArgName:  "app",
			expectedArgValue: "firefox",
		},
		{
			input:            "reproduce rock",
			expectedToolName: "browser.youtube_play",
			expectedArgName:  "query",
			expectedArgValue: "rock",
		},
		{
			input:            "abre la carpeta descargas",
			expectedToolName: "desktop.open_folder",
			expectedArgName:  "path",
			expectedArgValue: expectedDescargasPath,
		},
		{
			input:             "dame un resumen de doc.txt",
			shouldReturnEmpty: true, // Should bypass and go to LLM
		},
		{
			input:             "lee el archivo notas.txt",
			shouldReturnEmpty: true, // Should bypass and go to LLM
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actions := o.detectDirectIntents(tt.input)
			if tt.shouldReturnEmpty {
				if len(actions) > 0 {
					t.Errorf("Expected empty actions for %q, got: %+v", tt.input, actions)
				}
			} else {
				if len(actions) == 0 {
					t.Fatalf("Expected action for %q, got empty", tt.input)
				}
				if actions[0].ToolName != tt.expectedToolName {
					t.Errorf("Expected tool %q, got %q", tt.expectedToolName, actions[0].ToolName)
				}
				val, ok := actions[0].Args[tt.expectedArgName].(string)
				if !ok || val != tt.expectedArgValue {
					t.Errorf("Expected arg %q to be %q, got %v", tt.expectedArgName, tt.expectedArgValue, actions[0].Args[tt.expectedArgName])
				}
			}
		})
	}
}

func TestOrchestratorExecuteTool(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-exec-tool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	o := NewOrchestrator(database, nil, mcp.NewServerManager(), nil, []string{tempDir}, "RBot")

	// 1. Test memory.remember
	args := map[string]interface{}{
		"key":      "color",
		"value":    "azul",
		"category": "personal",
	}
	res, err := o.executeTool(context.Background(), "memory.remember", args)
	if err != nil {
		t.Fatalf("memory.remember failed: %v", err)
	}
	if !strings.Contains(res, "azul") {
		t.Errorf("Expected output to contain 'azul', got %q", res)
	}

	var storedVal string
	err = database.QueryRow("SELECT value FROM user_memory WHERE key = 'color'").Scan(&storedVal)
	if err != nil {
		t.Fatalf("Failed to retrieve stored value: %v", err)
	}
	if storedVal != "azul" {
		t.Errorf("Expected stored value 'azul', got %q", storedVal)
	}

	// 2. Test files.create_file
	filePath := filepath.Join(tempDir, "hello.txt")
	args = map[string]interface{}{
		"path":    filePath,
		"content": "hello orchestrator",
	}
	res, err = o.executeTool(context.Background(), "files.create_file", args)
	if err != nil {
		t.Fatalf("files.create_file failed: %v", err)
	}
	if !strings.Contains(res, "creado") {
		t.Errorf("Expected output to contain 'creado', got %q", res)
	}

	// Verify file was written
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}
	if string(data) != "hello orchestrator" {
		t.Errorf("Expected file content 'hello orchestrator', got %q", string(data))
	}

	// 3. Test files.read_file
	args = map[string]interface{}{
		"path": filePath,
	}
	res, err = o.executeTool(context.Background(), "files.read_file", args)
	if err != nil {
		t.Fatalf("files.read_file failed: %v", err)
	}
	if res != "hello orchestrator" {
		t.Errorf("Expected file read content 'hello orchestrator', got %q", res)
	}

	// 4. Test files.list_directory
	args = map[string]interface{}{
		"path": tempDir,
	}
	res, err = o.executeTool(context.Background(), "files.list_directory", args)
	if err != nil {
		t.Fatalf("files.list_directory failed: %v", err)
	}
	if !strings.Contains(res, "hello.txt") {
		t.Errorf("Expected directory listing to contain 'hello.txt', got %q", res)
	}

	// 5. Test files.delete_file
	args = map[string]interface{}{
		"path": filePath,
	}
	res, err = o.executeTool(context.Background(), "files.delete_file", args)
	if err != nil {
		t.Fatalf("files.delete_file failed: %v", err)
	}
	if !strings.Contains(res, "eliminado") {
		t.Errorf("Expected output to contain 'eliminado', got %q", res)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted, but it still exists")
	}
}

func TestOrchestratorChatEndToEnd(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-chat-e2e")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	// Setup mock HTTP server for Ollama LLM responses
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		callCount++
		if callCount == 1 {
			// First call: LLM decides to call tool 'memory.remember'
			resp := ollama.ChatResponse{
				Model: "llama3",
				Message: ollama.Message{
					Role: "assistant",
					ToolCalls: []ollama.ToolCall{
						{
							Type: "function",
							Function: ollama.FunctionCall{
								Name: "memory.remember",
								Arguments: map[string]interface{}{
									"key":      "color",
									"value":    "azul",
									"category": "personal",
								},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			// Second call: LLM receives tool execution results and replies with final text
			resp := ollama.ChatResponse{
				Model: "llama3",
				Message: ollama.Message{
					Role:    "assistant",
					Content: "He recordado que su color favorito es el azul, señor.",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	ollamaClient := ollama.NewClient(server.URL, "llama3")
	mcpManager := mcp.NewServerManager()
	
	o := NewOrchestrator(database, ollamaClient, mcpManager, nil, nil, "RBot")
	
	ctx := context.Background()
	reply, err := o.Chat(ctx, "recuerda que mi color favorito es el azul", nil)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if reply != "He recordado que su color favorito es el azul, señor." {
		t.Errorf("Unexpected reply: %q", reply)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls to Ollama client, got %d", callCount)
	}

	// Verify that the memory was actually inserted in the DB during the process
	var val string
	err = database.QueryRow("SELECT value FROM user_memory WHERE key = 'color'").Scan(&val)
	if err != nil {
		t.Fatalf("Failed to verify memory insertion in DB: %v", err)
	}
	if val != "azul" {
		t.Errorf("Expected DB color value 'azul', got %q", val)
	}
}
