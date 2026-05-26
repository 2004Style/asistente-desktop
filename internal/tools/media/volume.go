package media

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// VolumeBackend define la interfaz de control de volumen.
type VolumeBackend interface {
	Up(ctx context.Context, percent int) error
	Down(ctx context.Context, percent int) error
	Mute(ctx context.Context) error
}

// VolumeController selecciona el backend disponible: wpctl > pactl > amixer.
type VolumeController struct {
	backend VolumeBackend
}

func newVolumeController() *VolumeController {
	if _, err := exec.LookPath("wpctl"); err == nil {
		return &VolumeController{backend: &wpctlBackend{}}
	}
	if _, err := exec.LookPath("pactl"); err == nil {
		return &VolumeController{backend: &pactlBackend{}}
	}
	if _, err := exec.LookPath("amixer"); err == nil {
		return &VolumeController{backend: &amixerBackend{}}
	}
	return &VolumeController{backend: &noopVolumeBackend{}}
}

func (v *VolumeController) Up(ctx context.Context, percent int) error {
	return v.backend.Up(ctx, percent)
}

func (v *VolumeController) Down(ctx context.Context, percent int) error {
	return v.backend.Down(ctx, percent)
}

func (v *VolumeController) Mute(ctx context.Context) error {
	return v.backend.Mute(ctx)
}

// ─── wpctl backend ────────────────────────────────────────────────────────────

type wpctlBackend struct{}

func (b *wpctlBackend) Up(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("%d%%+", percent)
	cmd := exec.CommandContext(ctx, "wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wpctl set-volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *wpctlBackend) Down(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("%d%%-", percent)
	cmd := exec.CommandContext(ctx, "wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wpctl set-volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *wpctlBackend) Mute(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wpctl set-mute: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ─── pactl backend ────────────────────────────────────────────────────────────

type pactlBackend struct{}

func (b *pactlBackend) Up(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("+%d%%", percent)
	cmd := exec.CommandContext(ctx, "pactl", "set-sink-volume", "@DEFAULT_SINK@", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pactl set-sink-volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *pactlBackend) Down(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("-%d%%", percent)
	cmd := exec.CommandContext(ctx, "pactl", "set-sink-volume", "@DEFAULT_SINK@", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pactl set-sink-volume: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *pactlBackend) Mute(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pactl set-sink-mute: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ─── amixer backend ───────────────────────────────────────────────────────────

type amixerBackend struct{}

func (b *amixerBackend) Up(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("%d%%+", percent)
	cmd := exec.CommandContext(ctx, "amixer", "sset", "Master", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("amixer sset: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *amixerBackend) Down(ctx context.Context, percent int) error {
	arg := fmt.Sprintf("%d%%-", percent)
	cmd := exec.CommandContext(ctx, "amixer", "sset", "Master", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("amixer sset: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *amixerBackend) Mute(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "amixer", "sset", "Master", "toggle")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("amixer toggle: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ─── noop backend ─────────────────────────────────────────────────────────────

type noopVolumeBackend struct{}

func (b *noopVolumeBackend) Up(_ context.Context, _ int) error {
	return fmt.Errorf("control de volumen no disponible: no se encontró wpctl, pactl ni amixer")
}
func (b *noopVolumeBackend) Down(_ context.Context, _ int) error {
	return fmt.Errorf("control de volumen no disponible: no se encontró wpctl, pactl ni amixer")
}
func (b *noopVolumeBackend) Mute(_ context.Context) error {
	return fmt.Errorf("control de volumen no disponible: no se encontró wpctl, pactl ni amixer")
}
