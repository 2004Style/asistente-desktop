package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"rbot/internal/llm"
)

// openaiChatRequest es el cuerpo de POST /v1/chat/completions.
type openaiChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Tools       []llm.Tool      `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
}

// openaiChatResponse es la respuesta de POST /v1/chat/completions.
type openaiChatResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Choices []openaiChoice         `json:"choices"`
	Usage   map[string]interface{} `json:"usage,omitempty"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	Delta        openaiMessage `json:"delta,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

// openaiStreamChunk representa una línea de Server-Sent Events (SSE).
type openaiStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []openaiChoice `json:"choices"`
}

// openaiModelsResponse es la respuesta de GET /v1/models.
type openaiModelsResponse struct {
	Data []openaiModelEntry `json:"data"`
}

type openaiModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// Provider implementa llm.Provider para OpenAI.
type Provider struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewProvider crea un nuevo proveedor OpenAI.
func NewProvider(apiKey, model string) *Provider {
	return NewProviderWithBaseURL("https://api.openai.com", apiKey, model)
}

// NewProviderWithBaseURL crea un proveedor OpenAI con URL base explícita.
func NewProviderWithBaseURL(baseURL, apiKey, model string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &Provider{
		name:    "openai",
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// Name devuelve el identificador del proveedor.
func (p *Provider) Name() string { return p.name }

// ModelID devuelve el modelo actualmente configurado.
func (p *Provider) ModelID() string { return p.model }

// SetModel cambia el modelo activo.
func (p *Provider) SetModel(modelID string) { p.model = modelID }

// Chat envía mensajes al endpoint /v1/chat/completions de OpenAI.
func (p *Provider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	streamEnabled := opts.OnTextChunk != nil

	oMessages := make([]openaiMessage, len(messages))
	for i, m := range messages {
		oMessages[i] = openaiMessage{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		}
	}

	reqBody := openaiChatRequest{
		Model:    p.model,
		Messages: oMessages,
		Stream:   streamEnabled,
	}

	temp := opts.Temperature
	if temp > 0 {
		reqBody.Temperature = &temp
	}
	if opts.MaxTokens > 0 {
		reqBody.MaxTokens = &opts.MaxTokens
	}

	// Solo incluir tools si hay alguna
	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	url := p.formatURL("/v1/chat/completions")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(rawBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to OpenAI: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if streamEnabled {
		return p.readSSEStream(resp, opts.OnTextChunk)
	}

	var chatResp openaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned empty choices")
	}

	choice := chatResp.Choices[0]
	return &llm.Message{
		Role:      choice.Message.Role,
		Content:   choice.Message.Content,
		ToolCalls: choice.Message.ToolCalls,
	}, nil
}

// readSSEStream lee un stream de Server-Sent Events (SSE) de OpenAI.
func (p *Provider) readSSEStream(resp *http.Response, onChunk func(string)) (*llm.Message, error) {
	scanner := bufio.NewScanner(resp.Body)
	var fullMessage llm.Message
	fullMessage.Role = "assistant"

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {json}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			fullMessage.Content += delta.Content
			onChunk(delta.Content)
		}

		if len(delta.ToolCalls) > 0 {
			fullMessage.ToolCalls = append(fullMessage.ToolCalls, delta.ToolCalls...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("SSE stream read error: %v", err)
	}

	return &fullMessage, nil
}

// ListModels consulta GET /v1/models para listar modelos disponibles.
func (p *Provider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	url := p.formatURL("/v1/models")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to OpenAI: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI /v1/models returned status %d", resp.StatusCode)
	}

	var modelsResp openaiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("error decoding /v1/models response: %v", err)
	}

	var models []llm.ModelInfo
	for _, m := range modelsResp.Data {
		// Filtrar solo modelos de chat/completions (gpt, o-, chatgpt)
		if !isRelevantModel(m.ID) {
			continue
		}
		models = append(models, llm.ModelInfo{
			ID:       m.ID,
			Name:     m.ID,
			Provider: "openai",
			Capabilities: llm.ModelCapabilities{
				ToolCalling:       true,
				Streaming:         true,
				Vision:            strings.Contains(m.ID, "vision") || strings.Contains(m.ID, "4o"),
				ConversationState: true,
			},
		})
	}

	return models, nil
}

// Ping verifica que OpenAI esté accesible.
func (p *Provider) Ping(ctx context.Context) error {
	url := p.formatURL("/v1/models")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("OpenAI no disponible: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("API key de OpenAI inválida o expirada")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenAI devolvió status %d", resp.StatusCode)
	}
	return nil
}

func isRelevantModel(id string) bool {
	prefixes := []string{"gpt-", "o1", "o3", "o4", "chatgpt-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func (p *Provider) formatURL(path string) string {
	baseURL := strings.TrimRight(p.baseURL, "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") && strings.HasPrefix(strings.ToLower(path), "/v1") {
		path = path[3:]
	}
	return baseURL + path
}
