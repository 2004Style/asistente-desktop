//go:build !hud

package hud

import (
	"context"
	"log"

	"rbot/internal/config"
)

// Renderer is a no-GTK placeholder used by default builds and CI.
// Build with -tags hud to compile the GTK renderer.
type Renderer struct {
	config *config.Config
}

func NewRenderer(conf *config.Config) *Renderer {
	return &Renderer{config: conf}
}

func (r *Renderer) Start(ctx context.Context, socketPath string) {
	log.Printf("RBot HUD renderer is not compiled in this build; rebuild with -tags hud to enable GTK HUD. event_socket=%s", socketPath)
	<-ctx.Done()
}
