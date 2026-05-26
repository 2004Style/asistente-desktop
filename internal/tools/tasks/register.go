package tasks

import (
	"database/sql"

	"rbot/internal/config"
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete tasks en el Registry.
func RegisterTools(reg *executor.Registry, db *sql.DB, cfg *config.Config) error {
	repo := NewRepository(db)
	if err := reg.Register(NewAddTaskTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewListTasksTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewCompleteTaskTool(repo)); err != nil {
		return err
	}
	if err := reg.Register(NewDeleteTaskTool(repo)); err != nil {
		return err
	}
	if err := reg.Register(NewRescheduleTaskTool(repo, cfg)); err != nil {
		return err
	}
	if err := reg.Register(NewUpdateTaskPriorityTool(repo)); err != nil {
		return err
	}
	return nil
}
