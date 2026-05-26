package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

// YouTubePlayTool busca y abre directamente el primer vídeo de YouTube que coincida con la búsqueda.
type YouTubePlayTool struct{}

func NewYouTubePlayTool() *YouTubePlayTool {
	return &YouTubePlayTool{}
}

func (t *YouTubePlayTool) Name() string { return "browser.youtube_play" }
func (t *YouTubePlayTool) Description() string {
	return "Busca y reproduce inmediatamente el primer vídeo coincidente en YouTube para la consulta dada."
}
func (t *YouTubePlayTool) Category() string  { return "browser" }
func (t *YouTubePlayTool) RiskLevel() string { return "low" }
func (t *YouTubePlayTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Nombre de la canción, artista o vídeo a reproducir en YouTube.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *YouTubePlayTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("el argumento 'query' es obligatorio")
	}

	targetURL := getFirstYouTubeVideo(query)
	err := desktop.OpenURL(targetURL)
	if err != nil {
		return nil, err
	}

	textMsg := fmt.Sprintf("Buscando e intentando reproducir '%s' en YouTube.", query)
	if strings.Contains(targetURL, "/watch?v=") {
		textMsg = fmt.Sprintf("Reproduciendo '%s' en YouTube.", query)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       textMsg,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// YouTubeSearchTool abre la página de resultados de búsqueda de YouTube.
type YouTubeSearchTool struct{}

func NewYouTubeSearchTool() *YouTubeSearchTool {
	return &YouTubeSearchTool{}
}

func (t *YouTubeSearchTool) Name() string { return "browser.youtube_search" }
func (t *YouTubeSearchTool) Description() string {
	return "Busca una consulta en la web de YouTube y muestra la lista de resultados de vídeos."
}
func (t *YouTubeSearchTool) Category() string  { return "browser" }
func (t *YouTubeSearchTool) RiskLevel() string { return "low" }
func (t *YouTubeSearchTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Términos o palabras clave a buscar en YouTube.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *YouTubeSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("el argumento 'query' es obligatorio")
	}

	targetURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))
	err := desktop.OpenURL(targetURL)
	if err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Buscando resultados de '%s' en la web de YouTube.", query),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

func getFirstYouTubeVideo(query string) string {
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", strings.ReplaceAll(query, " ", "+"))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return searchURL
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return searchURL
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return searchURL
	}

	bodyStr := string(bodyBytes)
	re := regexp.MustCompile(`/watch\?v=[a-zA-Z0-9_-]{11}`)
	match := re.FindString(bodyStr)
	if match != "" {
		return "https://www.youtube.com" + match
	}

	return searchURL
}
