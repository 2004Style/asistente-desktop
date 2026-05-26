package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"rbot/internal/config"
)

// EventPublisher define la interfaz necesaria para publicar eventos desde el gestor de notificaciones.
type EventPublisher interface {
	Publish(eventType string, payload map[string]interface{})
}

// NotificationManager coordina el envío de alertas y notificaciones a diferentes canales
// respetando políticas de silencio de Quiet Hours y registrando auditoría en SQLite.
type NotificationManager struct {
	db     *sql.DB
	pub    EventPublisher
	config *config.Config
}

// NewNotificationManager inicializa el gestor.
func NewNotificationManager(db *sql.DB, pub EventPublisher, cfg *config.Config) *NotificationManager {
	return &NotificationManager{
		db:     db,
		pub:    pub,
		config: cfg,
	}
}


// Send despacha una notificación por el canal indicado (o canales múltiples si es "all" o "default").
// isUrgent permite saltarse el Quiet Hours si allow_urgent es true (o tratarlo según configuración).
func (m *NotificationManager) Send(ctx context.Context, channel string, title, message string, isUrgent bool) error {
	inQuietHours := m.IsInQuietHours()

	// Decidir qué canales procesar
	var channels []string
	switch channel {
	case "all":
		channels = []string{"desktop", "voice", "hud", "sound"}
	case "default", "":
		if m.config != nil && len(m.config.Reminders.DefaultChannels) > 0 {
			channels = m.config.Reminders.DefaultChannels
		} else {
			channels = []string{"desktop", "voice", "hud"}
		}
	default:
		channels = []string{channel}
	}

	for _, ch := range channels {
		// Validar Quiet Hours por canal
		if inQuietHours && !isUrgent {
			if ch == "voice" && m.config.Notifications.QuietHours.MuteVoice {
				m.logNotification(ch, title, message, "muted", "silenciado por quiet hours")
				continue
			}
			if ch == "sound" && m.config.Notifications.QuietHours.MuteSound {
				m.logNotification(ch, title, message, "muted", "silenciado por quiet hours")
				continue
			}
			if ch == "desktop" && !m.config.Notifications.QuietHours.AllowDesktop {
				m.logNotification(ch, title, message, "muted", "silenciado por quiet hours")
				continue
			}
			if ch == "hud" && !m.config.Notifications.QuietHours.AllowHUD {
				m.logNotification(ch, title, message, "muted", "silenciado por quiet hours")
				continue
			}
		}

		// Ejecutar canal específico
		var err error
		switch ch {
		case "desktop":
			err = m.sendDesktop(ctx, title, message)
		case "voice":
			err = m.sendVoice(title + ". " + message)
		case "hud":
			err = m.sendHUD(title, message)
		case "sound":
			err = m.sendSound(ctx)
		default:
			err = fmt.Errorf("canal de notificación no soportado: %s", ch)
		}

		if err != nil {
			log.Printf("[Notification] Error al enviar a canal %q: %v", ch, err)
			m.logNotification(ch, title, message, "failed", err.Error())
		} else {
			m.logNotification(ch, title, message, "sent", "")
		}
	}

	return nil
}

// IsInQuietHours evalúa si el momento actual está dentro del rango nocturno de silencio.
func (m *NotificationManager) IsInQuietHours() bool {
	if m.config == nil || !m.config.Notifications.QuietHours.Enabled {
		return false
	}

	loc := time.Local
	if m.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(m.config.Time.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)
	startStr := m.config.Notifications.QuietHours.Start
	endStr := m.config.Notifications.QuietHours.End

	var startH, startM, endH, endM int
	if _, err := fmt.Sscanf(startStr, "%d:%d", &startH, &startM); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(endStr, "%d:%d", &endH, &endM); err != nil {
		return false
	}

	startVal := startH*60 + startM
	endVal := endH*60 + endM
	nowVal := now.Hour()*60 + now.Minute()

	if startVal < endVal {
		return nowVal >= startVal && nowVal < endVal
	}
	return nowVal >= startVal || nowVal < endVal
}

func (m *NotificationManager) logNotification(channel, title, message, status, errMsg string) {
	if m.db == nil {
		return
	}
	var errVal sql.NullString
	if errMsg != "" {
		errVal.String = errMsg
		errVal.Valid = true
	}

	_, err := m.db.Exec(
		`INSERT INTO notification_log (channel, title, message, status, error, created_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		channel, title, message, status, errVal,
	)
	if err != nil {
		log.Printf("[Notification Log] Error al guardar auditoría de notificación: %v", err)
	}
}
