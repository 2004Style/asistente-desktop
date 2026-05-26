package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

// ─── browser.open_or_reuse ────────────────────────────────────────────────────

// OpenOrReuseTool intenta reutilizar una ventana de navegador existente o abre una nueva URL.
type OpenOrReuseTool struct {
	listWindows WindowListFunc
	focusWindow WindowFocusFunc
}

func NewOpenOrReuseTool(listWindows WindowListFunc, focusWindow WindowFocusFunc) *OpenOrReuseTool {
	return &OpenOrReuseTool{
		listWindows: listWindows,
		focusWindow: focusWindow,
	}
}

func (t *OpenOrReuseTool) Name() string        { return "browser.open_or_reuse" }
func (t *OpenOrReuseTool) Category() string    { return "browser" }
func (t *OpenOrReuseTool) RiskLevel() string   { return "low" }
func (t *OpenOrReuseTool) Description() string {
	return "Abre una URL en el navegador reutilizando una ventana existente si es posible, o abriendo una nueva."
}
func (t *OpenOrReuseTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL a abrir.",
			},
			"title_hint": map[string]interface{}{
				"type":        "string",
				"description": "Pista del título de ventana a buscar (opcional).",
			},
		},
		"required": []string{"url"},
	}
}

func (t *OpenOrReuseTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	targetURL, _ := args["url"].(string)
	if targetURL == "" {
		return nil, fmt.Errorf("el argumento 'url' es obligatorio")
	}

	titleHint, _ := args["title_hint"].(string)

	// Extraer dominio como pista si no hay title_hint
	if titleHint == "" {
		if parsed, err := url.Parse(targetURL); err == nil {
			titleHint = parsed.Hostname()
			// Simplificar: quitar www.
			titleHint = strings.TrimPrefix(titleHint, "www.")
		}
	}

	listFn := t.listWindows
	if listFn == nil {
		listFn = defaultWindowListFunc
	}
	focusFn := t.focusWindow
	if focusFn == nil {
		focusFn = defaultWindowFocusFunc
	}

	match, err := FindBestBrowserWindow(listFn, titleHint, "")
	if err == nil && match != nil && match.Confidence >= 0.70 {
		// Enfocar ventana existente
		if focusErr := focusFn(ctx, match.Window.Address); focusErr == nil {
			return &executor.ToolResult{
				Success: true,
				Text: fmt.Sprintf("Navegador encontrado y enfocado (confianza: %.0f%%, razón: %s). Navega a: %s",
					match.Confidence*100, match.Reason, targetURL),
				Data: map[string]interface{}{
					"reused":     true,
					"window":     match.Window.Title,
					"confidence": match.Confidence,
					"url":        targetURL,
				},
				StartedAt:  started,
				FinishedAt: time.Now(),
			}, nil
		}
	}

	// Abrir nueva URL
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	if err := desktop.OpenURL(targetURL); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success: true,
		Text:    fmt.Sprintf("URL '%s' abierta en el navegador.", targetURL),
		Data: map[string]interface{}{
			"reused": false,
			"url":    targetURL,
		},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── browser.youtube_open_or_reuse ───────────────────────────────────────────

// YouTubeOpenOrReuseTool busca una ventana de YouTube abierta y la enfoca, o abre YouTube.
type YouTubeOpenOrReuseTool struct {
	listWindows WindowListFunc
	focusWindow WindowFocusFunc
}

func NewYouTubeOpenOrReuseTool(listWindows WindowListFunc, focusWindow WindowFocusFunc) *YouTubeOpenOrReuseTool {
	return &YouTubeOpenOrReuseTool{
		listWindows: listWindows,
		focusWindow: focusWindow,
	}
}

func (t *YouTubeOpenOrReuseTool) Name() string        { return "browser.youtube_open_or_reuse" }
func (t *YouTubeOpenOrReuseTool) Category() string    { return "browser" }
func (t *YouTubeOpenOrReuseTool) RiskLevel() string   { return "low" }
func (t *YouTubeOpenOrReuseTool) Description() string {
	return "Busca una ventana de YouTube abierta y la enfoca, o abre YouTube (con o sin consulta de búsqueda)."
}
func (t *YouTubeOpenOrReuseTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Consulta de búsqueda en YouTube (opcional).",
			},
		},
	}
}

func (t *YouTubeOpenOrReuseTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)

	listFn := t.listWindows
	if listFn == nil {
		listFn = defaultWindowListFunc
	}
	focusFn := t.focusWindow
	if focusFn == nil {
		focusFn = defaultWindowFocusFunc
	}

	// Buscar ventana de YouTube
	match, err := FindBestBrowserWindow(listFn, "YouTube", "")
	if err == nil && match != nil && match.Confidence >= 0.70 {
		if focusErr := focusFn(ctx, match.Window.Address); focusErr == nil {
			return &executor.ToolResult{
				Success: true,
				Text:    "YouTube encontrado y enfocado.",
				Data: map[string]interface{}{
					"reused":     true,
					"window":     match.Window.Title,
					"confidence": match.Confidence,
				},
				StartedAt:  started,
				FinishedAt: time.Now(),
			}, nil
		}
	}

	// Abrir YouTube (con o sin búsqueda)
	targetURL := "https://www.youtube.com"
	if query != "" {
		targetURL = fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))
	}

	if err := desktop.OpenURL(targetURL); err != nil {
		return nil, err
	}

	text := "YouTube abierto en el navegador."
	if query != "" {
		text = fmt.Sprintf("Buscando '%s' en YouTube.", query)
	}

	return &executor.ToolResult{
		Success: true,
		Text:    text,
		Data: map[string]interface{}{
			"reused": false,
			"url":    targetURL,
		},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
