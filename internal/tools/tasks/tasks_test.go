package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rbot/internal/config"
	"rbot/internal/db"
)

func TestTasksWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-tasks-test")
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
	cfg.Tasks.DefaultPriority = "normal"
	cfg.Time.Timezone = "UTC"

	repo := NewRepository(database)
	ctx := context.Background()

	// 1. Create a task
	task, err := repo.Create(ctx, "Comprar leche", "Ir al supermercado", "high", "", "voice")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.Title != "Comprar leche" || task.Status != "pending" || task.Priority != "high" {
		t.Errorf("Unexpected task properties: %+v", task)
	}

	// 2. Get task
	got, err := repo.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if got.Title != "Comprar leche" {
		t.Errorf("Expected title 'Comprar leche', got '%s'", got.Title)
	}

	// 3. Update status to completed
	err = repo.UpdateStatus(ctx, task.ID, "completed")
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	got, _ = repo.Get(ctx, task.ID)
	if got.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", got.Status)
	}

	// 4. Update priority
	err = repo.UpdatePriority(ctx, task.ID, "urgent")
	if err != nil {
		t.Fatalf("Failed to update priority: %v", err)
	}
	got, _ = repo.Get(ctx, task.ID)
	if got.Priority != "urgent" {
		t.Errorf("Expected priority 'urgent', got '%s'", got.Priority)
	}

	// 5. Reschedule
	futureTime := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	err = repo.Reschedule(ctx, task.ID, futureTime)
	if err != nil {
		t.Fatalf("Failed to reschedule: %v", err)
	}
	got, _ = repo.Get(ctx, task.ID)
	if got.DueAt == nil || got.DueAt.Format(time.RFC3339) != futureTime {
		t.Errorf("Expected due_at '%s', got '%v'", futureTime, got.DueAt)
	}

	// 6. Delete
	err = repo.Delete(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	_, err = repo.Get(ctx, task.ID)
	if err == nil {
		t.Errorf("Expected error looking up deleted task, got nil")
	}
}
