package notifications

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func (m *NotificationManager) sendSound(ctx context.Context) error {
	// Intentar buscar un archivo de sonido estándar
	soundPaths := []string{
		"/usr/share/sounds/freedesktop/stereo/message.oga",
		"/usr/share/sounds/freedesktop/stereo/bell.oga",
		"/usr/share/sounds/freedesktop/stereo/dialog-information.oga",
		"/usr/share/sounds/freedesktop/stereo/complete.oga",
	}

	var selectedSound string
	for _, path := range soundPaths {
		if _, err := os.Stat(path); err == nil {
			selectedSound = path
			break
		}
	}

	if selectedSound == "" {
		// Si no se encuentra un sonido estándar, no fallamos ruidosamente
		return nil
	}

	// Buscar un reproductor compatible
	for _, player := range []string{"pw-play", "paplay"} {
		if _, err := exec.LookPath(player); err == nil {
			cmd := exec.CommandContext(ctx, player, selectedSound)
			return cmd.Run()
		}
	}

	// Si el archivo es .oga (ogg vorbis), aplay no lo reproducirá nativamente a menos que
	// busquemenos otro reproductor como mpv, paplay o pw-play.
	// Intentemos mpv o play (sox) si están disponibles
	for _, player := range []string{"mpv", "play"} {
		if _, err := exec.LookPath(player); err == nil {
			cmd := exec.CommandContext(ctx, player, selectedSound)
			return cmd.Run()
		}
	}

	return fmt.Errorf("no se encontró ningún reproductor de sonido compatible (pw-play, paplay, mpv, play)")
}
