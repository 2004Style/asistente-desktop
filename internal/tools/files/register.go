package files

import (
	"database/sql"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete files en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB, allowedRoots []string, blockedPaths []string) error {
	if err := reg.Register(NewSearchIndexTool(db, allowedRoots, blockedPaths)); err != nil {
		return err
	}
	if err := reg.Register(NewReadFileTool(db, allowedRoots, blockedPaths)); err != nil {
		return err
	}
	if err := reg.Register(NewCreateFileTool(blockedPaths)); err != nil {
		return err
	}
	if err := reg.Register(NewDeleteFileTool(db, allowedRoots, blockedPaths)); err != nil {
		return err
	}
	if err := reg.Register(NewListDirectoryTool(db, allowedRoots, blockedPaths)); err != nil {
		return err
	}
	if err := reg.Register(NewCreateDirectoryTool(blockedPaths)); err != nil {
		return err
	}
	return nil
}
