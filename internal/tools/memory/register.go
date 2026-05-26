package memory

import (
	"database/sql"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete memory en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB) error {
	if err := reg.Register(NewRememberTool(db)); err != nil {
		return err
	}
	return nil
}
