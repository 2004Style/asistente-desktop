package notifications

import (
	"rbot/internal/executor"
)

// RegisterTools registra todas las herramientas del paquete notifications en el Registry.
func RegisterTools(reg *executor.Registry, mgr *NotificationManager) error {
	if err := reg.Register(NewSendNotificationTool(mgr)); err != nil {
		return err
	}
	return nil
}
