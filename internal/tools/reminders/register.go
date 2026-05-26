package reminders

import (
	"database/sql"

	"rbot/internal/config"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete reminders en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB, cfg *config.Config) error {
	repo := NewRepository(db)
	if err := reg.Register(NewAddReminderTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewListRemindersTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewCancelReminderTool(repo)); err != nil {
		return err
	}
	if err := reg.Register(NewRescheduleReminderTool(repo, cfg)); err != nil {
		return err
	}
	return nil
}
