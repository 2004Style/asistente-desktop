package policy_test

import (
	"testing"
	"time"

	"rbot/internal/db"
	"rbot/internal/planner"
	"rbot/internal/policy"
)

func TestConfirmationEngine(t *testing.T) {
	engine := policy.NewEngine(nil, true)

	plan := &planner.Plan{ID: "plan-123"}
	engine.AddPending(plan, "dangerous action", 50*time.Millisecond)

	pending := engine.GetPending()
	if pending == nil {
		t.Fatalf("Expected pending plan, got nil")
	}
	if pending.PlanID != "plan-123" {
		t.Errorf("Expected plan-123, got %s", pending.PlanID)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)
	expired := engine.GetPending()
	if expired != nil {
		t.Fatalf("Expected nil after expiration, got plan")
	}
}

func TestSQLitePendingConfirmations(t *testing.T) {
	// Inicializar base de datos SQLite en memoria
	sqliteDB, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer sqliteDB.Close()

	plan := &planner.Plan{
		ID:        "test-plan-456",
		UserInput: "elimina la carpeta build",
		RiskLevel: "high",
	}

	source := "cli"
	sessionID := "local_cli"

	// 1. Guardar plan pendiente con TTL holgado (10s) para evitar problemas de latencia en tests
	err = policy.SavePendingPlan(sqliteDB, plan, "riesgo alto", source, sessionID, 10*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 2. Recuperar plan pendiente
	retrieved, reason, err := policy.GetPendingPlan(sqliteDB, source, sessionID)
	if err != nil {
		t.Fatalf("Expected no error retrieving plan, got %v", err)
	}
	if retrieved == nil {
		t.Fatalf("Expected retrieved plan, got nil")
	}
	if retrieved.ID != plan.ID || reason != "riesgo alto" {
		t.Errorf("Mismatch in retrieved plan. Got ID: %s, Reason: %s", retrieved.ID, reason)
	}

	// 3. Recuperar desde una sesión/canal distinto (debe ser nil)
	different, _, _ := policy.GetPendingPlan(sqliteDB, "voice", sessionID)
	if different != nil {
		t.Errorf("Expected nil for different source channel, got plan %s", different.ID)
	}

	// 4. Esperar expiración de un plan con TTL extremadamente corto
	planShort := &planner.Plan{
		ID:        "test-plan-short",
		UserInput: "test",
		RiskLevel: "high",
	}
	err = policy.SavePendingPlan(sqliteDB, planShort, "corto", source, "short_session", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected no error saving short plan, got %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	expired, _, err := policy.GetPendingPlan(sqliteDB, source, "short_session")

	if err != nil {
		t.Fatalf("Error checking expired plan: %v", err)
	}
	if expired != nil {
		t.Errorf("Expected plan to be expired and return nil, but got plan: %s", expired.ID)
	}
}

func TestAffirmativeNegativeCheck(t *testing.T) {
	affirmatives := []string{"sí", "si", "afirmativo", "confirmo", "aceptar", "ejecuta", "dale", "procede", "ok", "sí, hazlo", "dale de una"}
	negatives := []string{"no", "negativo", "cancela", "cancelar", "rechazo", "no ejecutes", "no hacer", "nop"}

	for _, text := range affirmatives {
		if !policy.IsAffirmative(text) {
			t.Errorf("Expected %q to be affirmative", text)
		}
		if policy.IsNegative(text) {
			t.Errorf("Expected %q NOT to be negative", text)
		}
	}

	for _, text := range negatives {
		if !policy.IsNegative(text) {
			t.Errorf("Expected %q to be negative", text)
		}
		if policy.IsAffirmative(text) {
			t.Errorf("Expected %q NOT to be affirmative", text)
		}
	}
}
