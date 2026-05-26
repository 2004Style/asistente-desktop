package notifications

import (
	"context"
	"fmt"
	"os/exec"
)

func (m *NotificationManager) sendDesktop(ctx context.Context, title, message string) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("notify-send no está instalado")
	}

	cmd := exec.CommandContext(ctx, "notify-send", title, message)
	return cmd.Run()
}
