package notifications

import (
	"rbot/internal/voice"
)

func (m *NotificationManager) sendVoice(message string) error {
	return voice.Speak(message)
}
