package compatible

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

// chatRequest sigue el formato OpenAI-compatible de /v1/chat/completions.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Tools       []llm.Tool      `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
}

type chatResponse struct {
	ID      string   `json:"id"`
	Choices []choice `json:"choices"`
}

type choice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta,omitempty"`
	FinishReason string      `json:"finish_reason"`
}

type streamChunk struct {
	ID      string   `json:"id"`
	Choices []choice `json:"choices"`
}

type modelsResponse struct {
	Data []modelEntry `json:"data"`
}

type modelEntry struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// Provider implementa llm.Provider para cualquier endpoint OpenAI-compatible
// (Groq, DeepSeek, LocalAI, LM Studio, vLLM, etc.)
type Provider struct {
	providerName string
	baseURL      string
	apiKey       string
	model        string
	customHeader string
	httpClient   *http.Client
}

// NewProvider crea un nuevo proveedor compatible.
// providerName: nombre identificador (ej: "groq", "deepseek", "localai")
// baseURL: URL base del endpoint (ej: "https://api.groq.com/openai")
// apiKey: clave API (puede estar vacía para endpoints locales)
// model: modelo por defecto
func NewProvider(providerName, baseURL, apiKey, model string) *Provider {
	return NewProviderWithHeader(providerName, baseURL, apiKey, model, "")
}

// NewProviderWithHeader crea un nuevo proveedor compatible con cabecera de autenticación personalizada.
func NewProviderWithHeader(providerName, baseURL, apiKey, model, customHeader string) *Provider {
	return &Provider{
		providerName: providerName,
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		model:        model,
		customHeader: customHeader,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (p *Provider) Name() string      { return p.providerName }
func (p *Provider) ModelID() string    { return p.model }
func (p *Provider) SetModel(id string) { p.model = id }

// Chat envía mensajes al endpoint /v1/chat/completions compatible.
func (p *Provider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	streamEnabled := opts.OnTextChunk != nil

	cMessages := make([]chatMessage, len(messages))
	for i, m := range messages {
		cMessages[i] = chatMessage{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		}
	}

	reqBody := chatRequest{
		Model:    p.model,
		Messages: cMessages,
		Stream:   streamEnabled,
	}

	if opts.Temperature > 0 {
		reqBody.Temperature = &opts.Temperature
	}
	if opts.MaxTokens > 0 {
		reqBody.MaxTokens = &opts.MaxTokens
	}
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
	if p.apiKey != "" {
		if p.customHeader != "" {
			req.Header.Set(p.customHeader, p.apiKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to %s: %v", p.providerName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s returned status %d: %s", p.providerName, resp.StatusCode, string(bodyBytes))
	}

	if streamEnabled {
		return p.readSSEStream(resp, opts.OnTextChunk)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("%s returned empty choices", p.providerName)
	}

	ch := chatResp.Choices[0]
	return &llm.Message{
		Role:      ch.Message.Role,
		Content:   ch.Message.Content,
		ToolCalls: ch.Message.ToolCalls,
	}, nil
}

func (p *Provider) readSSEStream(resp *http.Response, onChunk func(string)) (*llm.Message, error) {
	scanner := bufio.NewScanner(resp.Body)
	var fullMessage llm.Message
	fullMessage.Role = "assistant"

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
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

// ListModels consulta GET /v1/models.
func (p *Provider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	url := p.formatURL("/v1/models")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if p.apiKey != "" {
		if p.customHeader != "" {
			req.Header.Set(p.customHeader, p.apiKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to %s: %v", p.providerName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s /v1/models returned status %d", p.providerName, resp.StatusCode)
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("error decoding /v1/models response: %v", err)
	}

	var models []llm.ModelInfo
	for _, m := range modelsResp.Data {
		models = append(models, llm.ModelInfo{
			ID:       m.ID,
			Name:     m.ID,
			Provider: p.providerName,
			Capabilities: llm.ModelCapabilities{
				ToolCalling:       true,
				Streaming:         true,
				Vision:            false,
				ConversationState: true,
			},
		})
	}

	return models, nil
}

// Ping verifica que el endpoint esté accesible.
func (p *Provider) Ping(ctx context.Context) error {
	url := p.formatURL("/v1/models")
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if p.apiKey != "" {
		if p.customHeader != "" {
			req.Header.Set(p.customHeader, p.apiKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s no disponible en %s: %v", p.providerName, p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("API key inválida para %s", p.providerName)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s devolvió status %d", p.providerName, resp.StatusCode)
	}
	return nil
}

func (p *Provider) formatURL(path string) string {
	baseURL := strings.TrimRight(p.baseURL, "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") && strings.HasPrefix(strings.ToLower(path), "/v1") {
		path = path[3:]
	}
	return baseURL + path
}
