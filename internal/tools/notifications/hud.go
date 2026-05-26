package notifications

func (m *NotificationManager) sendHUD(title, message string) error {
	if m.pub != nil {
		m.pub.Publish("notification.sent", map[string]interface{}{
			"title":   title,
			"message": message,
		})
	}
	return nil
}

