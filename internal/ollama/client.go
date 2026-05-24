package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Parameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Stream   bool                   `json:"stream"`
	Tools    []Tool                 `json:"tools,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ChatResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Message   Message   `json:"message"`
	Done      bool      `json:"done"`
}

type Client struct {
	BaseURL     string
	Model       string
	Temperature float64
	OnTextChunk func(string)
	HTTPClient  *http.Client
}

func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{
		BaseURL:     baseURL,
		Model:       model,
		Temperature: 0.2,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Chat(messages []Message, tools []Tool) (*Message, error) {
	streamEnabled := c.OnTextChunk != nil

	reqBody := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   streamEnabled,
		Tools:    tools,
		Options: map[string]interface{}{
			"temperature": c.Temperature,
		},
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(rawBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errData map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errData)
		return nil, fmt.Errorf("ollama returned status %d: %v", resp.StatusCode, errData)
	}

	if streamEnabled {
		scanner := bufio.NewScanner(resp.Body)
		var fullMessage Message
		fullMessage.Role = "assistant"

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var chunk ChatResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				return nil, fmt.Errorf("error decoding stream chunk: %v", err)
			}

			if chunk.Message.Content != "" {
				fullMessage.Content += chunk.Message.Content
				c.OnTextChunk(chunk.Message.Content)
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

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &chatResp.Message, nil
}
