package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/executor"
	"rbot/internal/policy"
	"rbot/internal/tools/notifications"
)

func TestParseRRULEAndGetNext(t *testing.T) {
	start := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		rrule    string
		now      time.Time
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "Minutely 30 interval",
			rrule:    "FREQ=MINUTELY;INTERVAL=30",
			now:      start.Add(15 * time.Minute),
			expected: start.Add(30 * time.Minute),
			wantErr:  false,
		},
		{
			name:     "Hourly 2 interval",
			rrule:    "FREQ=HOURLY;INTERVAL=2",
			now:      start.Add(30 * time.Minute),
			expected: start.Add(2 * time.Hour),
			wantErr:  false,
		},
		{
			name:     "Daily 1 interval",
			rrule:    "FREQ=DAILY;INTERVAL=1",
			now:      start.Add(12 * time.Hour),
			expected: start.AddDate(0, 0, 1),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRRULEAndGetNext(tt.rrule, start, tt.now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRRULEAndGetNext() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.expected) {
				t.Errorf("ParseRRULEAndGetNext() = %v; expected %v", got, tt.expected)
			}
		})
	}
}

func TestProcessRecovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-scheduler-test")
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
	cfg.Scheduler.MaxLateMinutes = 120
	cfg.Scheduler.TickSeconds = 1
	cfg.Time.Timezone = "UTC"

	nm := notifications.NewNotificationManager(database, nil, cfg)
	reg := executor.NewRegistry()
	pol := policy.NewEngine([]string{}, false)
	ex := executor.NewExecutor(reg, pol, nil, database)

	s := NewScheduler(database, cfg, nm, ex, pol)


	ctx := context.Background()

	// 1. Crear un job muy viejo (debe expirar ya que han pasado más de 120 minutos)
	veryOldTime := time.Now().UTC().Add(-180 * time.Minute).Format(time.RFC3339)
	res, err := database.ExecContext(ctx,
		`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
		 VALUES ('reminder.notify', '{"reminder_id":1}', ?, 'pending', datetime('now'), datetime('now'))`,
		veryOldTime,
	)
	if err != nil {
		t.Fatalf("Failed to insert old job: %v", err)
	}
	oldJobID, _ := res.LastInsertId()

	// 2. Crear un job medianamente viejo (debe ejecutarse tarde, ya que está dentro de los 120 minutos)
	recentOldTime := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)
	res, err = database.ExecContext(ctx,
		`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
		 VALUES ('reminder.notify', '{"reminder_id":2}', ?, 'pending', datetime('now'), datetime('now'))`,
		recentOldTime,
	)
	if err != nil {
		t.Fatalf("Failed to insert recent job: %v", err)
	}
	recentJobID, _ := res.LastInsertId()

	// Insertar recordatorios mock correspondientes en la DB
	_, _ = database.ExecContext(ctx,
		`INSERT INTO reminders (id, title, message, remind_at, recurrence_rule, channels_json, priority, status, source)
		 VALUES (1, 'Old Reminder', 'Msg', ?, '', '["hud"]', 'normal', 'scheduled', 'voice')`,
		veryOldTime,
	)
	_, _ = database.ExecContext(ctx,
		`INSERT INTO reminders (id, title, message, remind_at, recurrence_rule, channels_json, priority, status, source)
		 VALUES (2, 'Recent Reminder', 'Msg', ?, '', '["hud"]', 'normal', 'scheduled', 'voice')`,
		recentOldTime,
	)

	summary, err := s.ProcessRecovery(ctx)
	if err != nil {
		t.Fatalf("ProcessRecovery returned error: %v", err)
	}

	if summary == "" {
		t.Errorf("Expected non-empty recovery summary")
	}

	// Verificar en la DB que el viejo expiró
	var oldStatus string
	err = database.QueryRowContext(ctx, "SELECT status FROM scheduled_jobs WHERE id = ?", oldJobID).Scan(&oldStatus)
	if err != nil {
		t.Fatalf("Failed to query old job status: %v", err)
	}
	if oldStatus != "expired" {
		t.Errorf("Expected old job status to be 'expired', got %s", oldStatus)
	}

	// Esperar un momento breve para que la goroutine de ejecución del job delayed termine
	time.Sleep(100 * time.Millisecond)

	// Verificar que el reciente se ejecutó o está en ejecución
	var recentStatus string
	err = database.QueryRowContext(ctx, "SELECT status FROM scheduled_jobs WHERE id = ?", recentJobID).Scan(&recentStatus)
	if err != nil {
		t.Fatalf("Failed to query recent job status: %v", err)
	}
	if recentStatus != "completed" && recentStatus != "running" {
		t.Errorf("Expected recent job to be running or completed, got %s", recentStatus)
	}
}
