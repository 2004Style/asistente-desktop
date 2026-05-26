package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"rbot/internal/llm"
)

// ollamaTagsResponse representa la respuesta de GET /api/tags de Ollama.
type ollamaTagsResponse struct {
	Models []ollamaModelEntry `json:"models"`
}

type ollamaModelEntry struct {
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	Size       int64     `json:"size"`
	Family     string    `json:"family,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
}

// ollamaChatRequest es el cuerpo de POST /api/chat de Ollama.
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Tools    []llm.Tool             `json:"tools,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// ollamaMessage es la representación interna de un mensaje en Ollama.
type ollamaMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
}

// ollamaChatResponse es la respuesta de POST /api/chat de Ollama.
type ollamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt time.Time     `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// Provider implementa llm.Provider para Ollama (local o remoto).
type Provider struct {
	name        string
	baseURL     string
	model       string
	apiKey      string // Bearer token para Ollama remoto (opcional)
	temperature float64
	httpClient  *http.Client
}

// NewProvider crea un nuevo proveedor Ollama.
func NewProvider(baseURL, model, apiKey string) *Provider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Provider{
		name:        "ollama",
		baseURL:     baseURL,
		model:       model,
		apiKey:      apiKey,
		temperature: 0.2,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name devuelve el identificador del proveedor.
func (p *Provider) Name() string { return p.name }

// ModelID devuelve el modelo actualmente configurado.
func (p *Provider) ModelID() string { return p.model }

// SetModel cambia el modelo activo.
func (p *Provider) SetModel(modelID string) { p.model = modelID }

// SetTemperature establece la temperatura de generación.
func (p *Provider) SetTemperature(temp float64) { p.temperature = temp }

// Chat envía mensajes al endpoint /api/chat de Ollama.
func (p *Provider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	streamEnabled := opts.OnTextChunk != nil

	// Convertir mensajes genéricos a formato Ollama
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, m := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		}
	}

	temp := p.temperature
	if opts.Temperature > 0 {
		temp = opts.Temperature
	}

	reqBody := ollamaChatRequest{
		Model:    p.model,
		Messages: ollamaMessages,
		Stream:   streamEnabled,
		Tools:    tools,
		Options: map[string]interface{}{
			"temperature": temp,
		},
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	url := fmt.Sprintf("%s/api/chat", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(rawBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to ollama at %s: %v", p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		return nil, fmt.Errorf("ollama returned status %d: %v", resp.StatusCode, errData)
	}

	if streamEnabled {
		scanner := bufio.NewScanner(resp.Body)
		var fullMessage llm.Message
		fullMessage.Role = "assistant"

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var chunk ollamaChatResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				return nil, fmt.Errorf("error decoding stream chunk: %v", err)
			}

			if chunk.Message.Content != "" {
				fullMessage.Content += chunk.Message.Content
				opts.OnTextChunk(chunk.Message.Content)
			}

			if len(chunk.Message.ToolCalls) > 0 {
				fullMessage.ToolCalls = append(fullMessage.ToolCalls, chunk.Message.ToolCalls...)
			}

			if chunk.Done {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("stream read error: %v", err)
		}
		return &fullMessage, nil
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &llm.Message{
		Role:      chatResp.Message.Role,
		Content:   chatResp.Message.Content,
		ToolCalls: chatResp.Message.ToolCalls,
	}, nil
}

// ListModels consulta GET /api/tags para obtener los modelos disponibles.
func (p *Provider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/tags returned status %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("error decoding /api/tags response: %v", err)
	}

	var models []llm.ModelInfo
	for _, m := range tagsResp.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		models = append(models, llm.ModelInfo{
			ID:       name,
			Name:     name,
			Provider: "ollama",
			Family:   m.Family,
			Size:     formatBytes(m.Size),
			Capabilities: llm.ModelCapabilities{
				ToolCalling:       true,
				Streaming:         true,
				Vision:            false,
				ConversationState: true,
			},
			ModifiedAt: m.ModifiedAt,
		})
	}

	return models, nil
}

// Ping verifica que Ollama esté accesible consultando /api/tags.
func (p *Provider) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama no disponible en %s: %v", p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama devolvió status %d", resp.StatusCode)
	}
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
