package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"rbot/internal/executor"
)

// ReadURLTool lee y limpia el contenido textual de una página web.
type ReadURLTool struct{}

func NewReadURLTool() *ReadURLTool {
	return &ReadURLTool{}
}

func (t *ReadURLTool) Name() string { return "browser.read_url" }
func (t *ReadURLTool) Description() string {
	return "Lee y extrae el contenido de texto limpio de una página web o dirección URL para poder resumirla o responder preguntas."
}
func (t *ReadURLTool) Category() string  { return "browser" }
func (t *ReadURLTool) RiskLevel() string { return "low" }
func (t *ReadURLTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "La dirección URL del sitio web a leer.",
			},
		},
		"required": []string{"url"},
	}
}

func (t *ReadURLTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	targetURL, _ := args["url"].(string)
	if targetURL == "" {
		return nil, fmt.Errorf("el argumento 'url' es obligatorio")
	}

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error al crear petición: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al conectar con la página web: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la página devolvió un código de estado de error: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer contenido: %v", err)
	}

	htmlStr := string(bodyBytes)
	reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	htmlStr = reScript.ReplaceAllString(htmlStr, "")
	reStyle := regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	htmlStr = reStyle.ReplaceAllString(htmlStr, "")
	reTags := regexp.MustCompile(`<.*?>`)
	textStr := reTags.ReplaceAllString(htmlStr, " ")
	reSpaces := regexp.MustCompile(`\s+`)
	textStr = reSpaces.ReplaceAllString(textStr, " ")
	textStr = strings.TrimSpace(textStr)

	if len(textStr) > 4000 {
		textStr = textStr[:4000] + "... [Contenido truncado]"
	}

	if textStr == "" {
		return nil, fmt.Errorf("no se pudo extraer texto legible de la página")
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       textStr,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
