package workspace

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Watcher struct {
	workspacePath string
	files         []string
	modTimes      map[string]time.Time
	mu            sync.Mutex
	onReload      func(*WorkspaceContext)
	loader        *Loader
}

func NewWatcher(workspacePath string, files []string, loader *Loader, onReload func(*WorkspaceContext)) *Watcher {
	if len(files) == 0 {
		files = []string{
			"AGENTS.md", "IDENTITY.md", "TOOLS.md", "POLICIES.md", "MEMORY.md", "TASKS.md", "SHORTCUTS.md",
		}
	}
	return &Watcher{
		workspacePath: workspacePath,
		files:         files,
		modTimes:      make(map[string]time.Time),
		onReload:      onReload,
		loader:        loader,
	}
}

func (w *Watcher) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	// Carga inicial del mapa de tiempos de modificación
	w.scan(true)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if w.scan(false) {
					log.Println("[Workspace Watcher] Cambios detectados en el workspace, recargando...")
					newCtx, err := w.loader.Load()
					if err == nil && w.onReload != nil {
						w.onReload(newCtx)
					}
				}
			}
		}
	}()
}

func (w *Watcher) scan(isInitial bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	changed := false

	for _, filename := range w.files {
		path := filepath.Join(w.workspacePath, filename)
		info, err := os.Stat(path)
		if err != nil {
			// El archivo no existe o no se puede leer
			if _, exists := w.modTimes[filename]; exists {
				delete(w.modTimes, filename)
				changed = true
			}
			continue
		}

		mTime := info.ModTime()
		prevTime, exists := w.modTimes[filename]
		if !exists || !mTime.Equal(prevTime) {
			w.modTimes[filename] = mTime
			if !isInitial {
				changed = true
			}
		}
	}

	return changed
}
