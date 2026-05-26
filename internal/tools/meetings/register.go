package meetings

import (
	"database/sql"

	"rbot/internal/config"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete meetings en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB, cfg *config.Config) error {
	repo := NewRepository(db)
	if err := reg.Register(NewAddMeetingTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewListMeetingsTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewTodayMeetingsTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewNextMeetingTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewCancelMeetingTool(repo)); err != nil {
		return err
	}
	return nil
}
