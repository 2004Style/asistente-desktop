package media

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/executor"
)

var globalPlayer = newPlayerController()
var globalVolume = newVolumeController()

// ─── media.play ──────────────────────────────────────────────────────────────

type PlayTool struct{}

func NewPlayTool() *PlayTool { return &PlayTool{} }

func (t *PlayTool) Name() string        { return "media.play" }
func (t *PlayTool) Category() string    { return "media" }
func (t *PlayTool) RiskLevel() string   { return "low" }
func (t *PlayTool) Description() string {
	return "Inicia la reproducción multimedia. Si se proporciona una consulta, busca y reproduce en YouTube."
}
func (t *PlayTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Canción, artista o vídeo a reproducir (opcional). Si se omite, reanuda la reproducción actual.",
			},
		},
	}
}

func (t *PlayTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	query, _ := args["query"].(string)

	if query == "" {
		// Sin consulta: reproducir con playerctl
		if err := globalPlayer.Play(ctx); err != nil {
			return nil, fmt.Errorf("no hay reproductor activo: %v", err)
		}
		return &executor.ToolResult{
			Success:    true,
			Text:       "Reproducción iniciada.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	// Con consulta: abrir en YouTube
	targetURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))
	if err := desktop.OpenURL(targetURL); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Buscando '%s' en YouTube.", query),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.pause ─────────────────────────────────────────────────────────────

type PauseTool struct{}

func NewPauseTool() *PauseTool { return &PauseTool{} }

func (t *PauseTool) Name() string        { return "media.pause" }
func (t *PauseTool) Category() string    { return "media" }
func (t *PauseTool) RiskLevel() string   { return "low" }
func (t *PauseTool) Description() string { return "Pausa la reproducción multimedia actual." }
func (t *PauseTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *PauseTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalPlayer.Pause(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Reproducción pausada.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.resume ────────────────────────────────────────────────────────────

type ResumeTool struct{}

func NewResumeTool() *ResumeTool { return &ResumeTool{} }

func (t *ResumeTool) Name() string        { return "media.resume" }
func (t *ResumeTool) Category() string    { return "media" }
func (t *ResumeTool) RiskLevel() string   { return "low" }
func (t *ResumeTool) Description() string { return "Reanuda la reproducción multimedia pausada." }
func (t *ResumeTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *ResumeTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalPlayer.Play(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Reproducción reanudada.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.toggle ────────────────────────────────────────────────────────────

type ToggleTool struct{}

func NewToggleTool() *ToggleTool { return &ToggleTool{} }

func (t *ToggleTool) Name() string        { return "media.toggle" }
func (t *ToggleTool) Category() string    { return "media" }
func (t *ToggleTool) RiskLevel() string   { return "low" }
func (t *ToggleTool) Description() string {
	return "Alterna entre reproducción y pausa del reproductor multimedia activo."
}
func (t *ToggleTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *ToggleTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalPlayer.Toggle(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Reproducción alternada (play/pause).",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.next ──────────────────────────────────────────────────────────────

type NextTool struct{}

func NewNextTool() *NextTool { return &NextTool{} }

func (t *NextTool) Name() string        { return "media.next" }
func (t *NextTool) Category() string    { return "media" }
func (t *NextTool) RiskLevel() string   { return "low" }
func (t *NextTool) Description() string { return "Salta a la siguiente pista en el reproductor activo." }
func (t *NextTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *NextTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalPlayer.Next(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Saltando a la siguiente pista.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.previous ──────────────────────────────────────────────────────────

type PreviousTool struct{}

func NewPreviousTool() *PreviousTool { return &PreviousTool{} }

func (t *PreviousTool) Name() string        { return "media.previous" }
func (t *PreviousTool) Category() string    { return "media" }
func (t *PreviousTool) RiskLevel() string   { return "low" }
func (t *PreviousTool) Description() string { return "Regresa a la pista anterior en el reproductor activo." }
func (t *PreviousTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *PreviousTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalPlayer.Previous(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Regresando a la pista anterior.",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.volume_up ─────────────────────────────────────────────────────────

type VolumeUpTool struct{}

func NewVolumeUpTool() *VolumeUpTool { return &VolumeUpTool{} }

func (t *VolumeUpTool) Name() string        { return "media.volume_up" }
func (t *VolumeUpTool) Category() string    { return "media" }
func (t *VolumeUpTool) RiskLevel() string   { return "low" }
func (t *VolumeUpTool) Description() string { return "Sube el volumen del sistema." }
func (t *VolumeUpTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"percent": map[string]interface{}{
				"type":        "integer",
				"description": "Porcentaje a subir el volumen (por defecto: 5).",
			},
		},
	}
}

func (t *VolumeUpTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	percent := 5
	if v, ok := args["percent"].(float64); ok && v > 0 {
		percent = int(v)
	}
	if err := globalVolume.Up(ctx, percent); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Volumen subido un %d%%.", percent),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.volume_down ───────────────────────────────────────────────────────

type VolumeDownTool struct{}

func NewVolumeDownTool() *VolumeDownTool { return &VolumeDownTool{} }

func (t *VolumeDownTool) Name() string        { return "media.volume_down" }
func (t *VolumeDownTool) Category() string    { return "media" }
func (t *VolumeDownTool) RiskLevel() string   { return "low" }
func (t *VolumeDownTool) Description() string { return "Baja el volumen del sistema." }
func (t *VolumeDownTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"percent": map[string]interface{}{
				"type":        "integer",
				"description": "Porcentaje a bajar el volumen (por defecto: 5).",
			},
		},
	}
}

func (t *VolumeDownTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	percent := 5
	if v, ok := args["percent"].(float64); ok && v > 0 {
		percent = int(v)
	}
	if err := globalVolume.Down(ctx, percent); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Volumen bajado un %d%%.", percent),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.mute ──────────────────────────────────────────────────────────────

type MuteTool struct{}

func NewMuteTool() *MuteTool { return &MuteTool{} }

func (t *MuteTool) Name() string        { return "media.mute" }
func (t *MuteTool) Category() string    { return "media" }
func (t *MuteTool) RiskLevel() string   { return "low" }
func (t *MuteTool) Description() string { return "Activa o desactiva el silencio del sistema." }
func (t *MuteTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *MuteTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	if err := globalVolume.Mute(ctx); err != nil {
		return nil, err
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       "Silencio alternado (mute/unmute).",
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ─── media.status ────────────────────────────────────────────────────────────

type StatusTool struct{}

func NewStatusTool() *StatusTool { return &StatusTool{} }

func (t *StatusTool) Name() string        { return "media.status" }
func (t *StatusTool) Category() string    { return "media" }
func (t *StatusTool) RiskLevel() string   { return "low" }
func (t *StatusTool) Description() string {
	return "Devuelve el estado actual del reproductor multimedia y la pista en reproducción."
}
func (t *StatusTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *StatusTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	status, err := globalPlayer.Status(ctx)
	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Text:       "No hay ningún reproductor multimedia activo.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	track, _ := globalPlayer.CurrentTrack(ctx)

	text := fmt.Sprintf("Estado: %s", status)
	if track != "" && track != "Pista desconocida" {
		text = fmt.Sprintf("Estado: %s — Pista: %s", status, track)
	}

	return &executor.ToolResult{
		Success: true,
		Text:    text,
		Data: map[string]interface{}{
			"status": status,
			"track":  track,
		},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
