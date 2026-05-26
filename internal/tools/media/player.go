package media

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PlayerController controla la reproducción multimedia con playerctl.
type PlayerController struct{}

func newPlayerController() *PlayerController {
	return &PlayerController{}
}

func (p *PlayerController) available() bool {
	_, err := exec.LookPath("playerctl")
	return err == nil
}

func (p *PlayerController) run(ctx context.Context, args ...string) (string, error) {
	if !p.available() {
		return "", fmt.Errorf("playerctl no encontrado en el PATH")
	}
	cmd := exec.CommandContext(ctx, "playerctl", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		if output != "" {
			return "", fmt.Errorf("playerctl %s: %s", strings.Join(args, " "), output)
		}
		return "", fmt.Errorf("playerctl %s: %v", strings.Join(args, " "), err)
	}
	return output, nil
}

// Play inicia la reproducción.
func (p *PlayerController) Play(ctx context.Context) error {
	_, err := p.run(ctx, "play")
	return err
}

// Pause pausa la reproducción.
func (p *PlayerController) Pause(ctx context.Context) error {
	_, err := p.run(ctx, "pause")
	return err
}

// Toggle alterna entre reproducción y pausa.
func (p *PlayerController) Toggle(ctx context.Context) error {
	_, err := p.run(ctx, "play-pause")
	return err
}

// Next salta a la siguiente pista.
func (p *PlayerController) Next(ctx context.Context) error {
	_, err := p.run(ctx, "next")
	return err
}

// Previous regresa a la pista anterior.
func (p *PlayerController) Previous(ctx context.Context) error {
	_, err := p.run(ctx, "previous")
	return err
}

// Status devuelve el estado actual del reproductor: "Playing", "Paused" o "Stopped".
func (p *PlayerController) Status(ctx context.Context) (string, error) {
	return p.run(ctx, "status")
}

// CurrentTrack devuelve información sobre la pista actual.
func (p *PlayerController) CurrentTrack(ctx context.Context) (string, error) {
	out, err := p.run(ctx, "metadata", "--format", "{{title}} - {{artist}}")
	if err != nil {
		return "", err
	}
	// Limpiar formato vacío
	if out == " - " || out == "-" || out == "" {
		return "Pista desconocida", nil
	}
	return out, nil
}

// HasActivePlayer verifica si hay un reproductor activo disponible.
func (p *PlayerController) HasActivePlayer(ctx context.Context) bool {
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	status, err := p.Status(ctx2)
	if err != nil {
		return false
	}
	return status != "" && status != "No players found"
}
