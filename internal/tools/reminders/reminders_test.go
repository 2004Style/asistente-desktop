package reminders

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rbot/internal/config"
	"rbot/internal/db"
)

func TestRemindersWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-reminders-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "rbot.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer database.Close()

	cfg := &config.Config{}
	cfg.Reminders.DefaultChannels = []string{"hud"}
	cfg.Time.Timezone = "UTC"

	repo := NewRepository(database)
	ctx := context.Background()

	remindTime := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)

	// 1. Create reminder (should also insert scheduled_job!)
	rem, err := repo.Create(ctx, "Tomar agua", "Hidratarse", remindTime, "FREQ=MINUTELY;INTERVAL=30", `["desktop","hud"]`, "normal", "voice")
	if err != nil {
		t.Fatalf("Failed to create reminder: %v", err)
	}

	if rem.Title != "Tomar agua" || rem.Status != "scheduled" || rem.RecurrenceRule != "FREQ=MINUTELY;INTERVAL=30" {
		t.Errorf("Unexpected reminder properties: %+v", rem)
	}

	// Verify scheduled_jobs has a pending job for this reminder
	var jobCount int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_jobs WHERE job_type = 'reminder.notify' AND status = 'pending'").Scan(&jobCount)
	if err != nil {
		t.Fatalf("Failed to query scheduled_jobs: %v", err)
	}
	if jobCount != 1 {
		t.Errorf("Expected 1 pending scheduled job, got %d", jobCount)
	}

	// 2. Reschedule reminder (should cancel old job, create new job)
	newRemindTime := time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339)
	err = repo.Reschedule(ctx, rem.ID, newRemindTime)
	if err != nil {
		t.Fatalf("Failed to reschedule: %v", err)
	}

	got, _ := repo.Get(ctx, rem.ID)
	if got.RemindAt.Format(time.RFC3339) != newRemindTime {
		t.Errorf("Expected remind_at '%s', got '%s'", newRemindTime, got.RemindAt.Format(time.RFC3339))
	}

	// Verify old job was cancelled, new job is pending
	var pendingCount int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_jobs WHERE job_type = 'reminder.notify' AND status = 'pending'").Scan(&pendingCount)
	if err != nil {
		t.Fatalf("Failed to query pending jobs: %v", err)
	}
	if pendingCount != 1 {
		t.Errorf("Expected 1 pending job after reschedule, got %d", pendingCount)
	}

	var cancelledCount int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_jobs WHERE job_type = 'reminder.notify' AND status = 'cancelled'").Scan(&cancelledCount)
	if err != nil {
		t.Fatalf("Failed to query cancelled jobs: %v", err)
	}
	if cancelledCount != 1 {
		t.Errorf("Expected 1 cancelled job after reschedule, got %d", cancelledCount)
	}

	// 3. Cancel reminder (should cancel pending job)
	err = repo.UpdateStatus(ctx, rem.ID, "cancelled")
	if err != nil {
		t.Fatalf("Failed to cancel reminder: %v", err)
	}

	got, _ = repo.Get(ctx, rem.ID)
	if got.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", got.Status)
	}

	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_jobs WHERE job_type = 'reminder.notify' AND status = 'pending'").Scan(&pendingCount)
	if err != nil {
		t.Fatalf("Failed to query pending jobs: %v", err)
	}
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending jobs after cancel, got %d", pendingCount)
	}
}
