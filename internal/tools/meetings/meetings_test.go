package meetings

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rbot/internal/config"
	"rbot/internal/db"
)

func TestMeetingsWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-meetings-test")
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
	cfg.Meetings.DefaultNotifyBeforeMinutes = 10
	cfg.Time.Timezone = "UTC"

	repo := NewRepository(database)
	ctx := context.Background()

	startTime := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	endTime := time.Now().Add(60 * time.Minute).UTC().Format(time.RFC3339)

	// 1. Create meeting
	m, err := repo.Create(ctx, "Daily Scrum", startTime, endTime, "Google Meet", "local", "", 10)
	if err != nil {
		t.Fatalf("Failed to create meeting: %v", err)
	}

	if m.Title != "Daily Scrum" || m.Status != "scheduled" || m.NotifyBeforeMinutes != 10 {
		t.Errorf("Unexpected meeting properties: %+v", m)
	}

	// Verify scheduled_jobs has a pending job for this meeting, scheduled at starts_at - 10m
	var jobCount int
	var runAtStr string
	err = database.QueryRowContext(ctx, "SELECT COUNT(*), run_at FROM scheduled_jobs WHERE job_type = 'meeting.notify' AND status = 'pending'").Scan(&jobCount, &runAtStr)
	if err != nil {
		t.Fatalf("Failed to query scheduled_jobs: %v", err)
	}
	if jobCount != 1 {
		t.Errorf("Expected 1 pending meeting notification job, got %d", jobCount)
	}

	parsedStart, _ := time.Parse(time.RFC3339, startTime)
	expectedRunAt := parsedStart.Add(-10 * time.Minute)
	parsedRunAt, _ := time.Parse(time.RFC3339, runAtStr)

	if !parsedRunAt.Equal(expectedRunAt) {
		t.Errorf("Expected run_at %v, got %v", expectedRunAt, parsedRunAt)
	}

	// 2. Get list of meetings
	list, err := repo.List(ctx, []string{"scheduled"})
	if err != nil {
		t.Fatalf("Failed to list meetings: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Daily Scrum" {
		t.Errorf("Expected 1 meeting 'Daily Scrum', got list: %+v", list)
	}

	// 3. Update status to active
	err = repo.UpdateStatus(ctx, m.ID, "active")
	if err != nil {
		t.Fatalf("Failed to update meeting status: %v", err)
	}
	got, _ := repo.Get(ctx, m.ID)
	if got.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", got.Status)
	}

	// 4. Cancel meeting (should cancel scheduled_job too)
	err = repo.UpdateStatus(ctx, m.ID, "cancelled")
	if err != nil {
		t.Fatalf("Failed to cancel meeting: %v", err)
	}
	got, _ = repo.Get(ctx, m.ID)
	if got.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", got.Status)
	}

	var pendingCount int
	_ = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_jobs WHERE job_type = 'meeting.notify' AND status = 'pending'").Scan(&pendingCount)
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending jobs after cancel, got %d", pendingCount)
	}
}
