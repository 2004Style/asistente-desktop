package desktop

import (
	"database/sql"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete desktop en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB, allowedRoots []string, blockedPaths []string) error {
	// ── Existing tools ──────────────────────────────────────────────────────
	if err := reg.Register(NewOpenAppTool(db)); err != nil {
		return err
	}
	if err := reg.Register(NewCloseAppTool(db)); err != nil {
		return err
	}
	if err := reg.Register(NewOpenFolderTool(db, allowedRoots, blockedPaths)); err != nil {
		return err
	}

	// ── Window management tools ─────────────────────────────────────────────
	wm := NewWindowManager()
	if err := reg.Register(NewListWindowsTool(wm)); err != nil {
		return err
	}
	if err := reg.Register(NewActiveWindowTool(wm)); err != nil {
		return err
	}
	if err := reg.Register(NewFocusWindowTool(wm)); err != nil {
		return err
	}
	if err := reg.Register(NewCloseWindowTool(wm)); err != nil {
		return err
	}
	if err := reg.Register(NewMoveWindowTool(wm)); err != nil {
		return err
	}

	// ── Environment capabilities tool ───────────────────────────────────────
	if err := reg.Register(NewCapabilitiesTool(db)); err != nil {
		return err
	}

	return nil
}
