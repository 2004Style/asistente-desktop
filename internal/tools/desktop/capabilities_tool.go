package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"rbot/internal/environment"
	"rbot/internal/executor"
)

// CapabilitiesTool implements environment.capabilities.
type CapabilitiesTool struct {
	DB *sql.DB
}

func NewCapabilitiesTool(db *sql.DB) *CapabilitiesTool {
	return &CapabilitiesTool{DB: db}
}

func (t *CapabilitiesTool) Name() string        { return "environment.capabilities" }
func (t *CapabilitiesTool) Category() string    { return "environment" }
func (t *CapabilitiesTool) RiskLevel() string   { return "low" }
func (t *CapabilitiesTool) Description() string {
	return "Devuelve las capacidades detectadas del entorno de escritorio: tipo de sesión, compositor, backends disponibles (ventanas, entrada, media, volumen) y herramientas instaladas."
}
func (t *CapabilitiesTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// capabilitiesResponse is the JSON structure returned to the AI.
type capabilitiesResponse struct {
	SessionType string            `json:"session_type"`
	Desktop     string            `json:"desktop"`
	Backends    backendsInfo      `json:"backends"`
	Binaries    map[string]bool   `json:"binaries"`
}

type backendsInfo struct {
	Window string `json:"window"`
	Input  string `json:"input"`
	Media  string `json:"media"`
	Volume string `json:"volume"`
}

func (t *CapabilitiesTool) Execute(ctx context.Context, _ map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	caps, err := environment.LoadCapabilities(t.DB)
	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      fmt.Sprintf("no se pudieron cargar las capacidades del entorno: %v", err),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	resp := capabilitiesResponse{
		SessionType: caps.SessionType,
		Desktop:     caps.Desktop,
		Backends: backendsInfo{
			Window: caps.WindowBackend,
			Input:  caps.InputBackend,
			Media:  caps.MediaBackend,
			Volume: caps.VolumeBackend,
		},
		Binaries: map[string]bool{
			"hyprctl":    caps.HasHyprctl,
			"swaymsg":    caps.HasSwaymsg,
			"wmctrl":     caps.HasWmctrl,
			"xdotool":    caps.HasXdotool,
			"wtype":      caps.HasWtype,
			"ydotool":    caps.HasYdotool,
			"playerctl":  caps.HasPlayerctl,
			"wpctl":      caps.HasWpctl,
			"pactl":      caps.HasPactl,
			"amixer":     caps.HasAmixer,
		},
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("serializing capabilities: %w", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Entorno: %s (%s), backend de ventanas: %s", caps.Desktop, caps.SessionType, caps.WindowBackend),
		Data:       map[string]interface{}{"capabilities": json.RawMessage(raw)},
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
