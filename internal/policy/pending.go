package policy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"rbot/internal/planner"
)

// SavePendingPlan guarda un plan de ejecución pendiente de confirmación en la base de datos.
func SavePendingPlan(db *sql.DB, plan *planner.Plan, reason, source, sessionID string, ttl time.Duration) error {
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("error serializando plan: %v", err)
	}

	expiresAt := time.Now().Add(ttl).Format(time.RFC3339)

	query := `INSERT OR REPLACE INTO pending_confirmations (plan_id, plan_json, reason, source, session_id, status, expires_at)
	          VALUES (?, ?, ?, ?, ?, 'pending', ?)`

	_, err = db.Exec(query, plan.ID, string(planJSON), reason, source, sessionID, expiresAt)
	if err != nil {
		return fmt.Errorf("error guardando plan pendiente en db: %v", err)
	}
	return nil
}

// GetPendingPlan busca el plan pendiente más reciente para una sesión/canal que no haya expirado.
// Retorna (plan, reason, error)
func GetPendingPlan(db *sql.DB, source, sessionID string) (*planner.Plan, string, error) {
	var planJSON, reason, expiresAtStr, planID string

	query := `SELECT plan_id, plan_json, reason, expires_at FROM pending_confirmations
	          WHERE source = ? AND session_id = ? AND status = 'pending'
	          ORDER BY created_at DESC LIMIT 1`

	err := db.QueryRow(query, source, sessionID).Scan(&planID, &planJSON, &reason, &expiresAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", nil
		}
		return nil, "", err
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return nil, "", fmt.Errorf("error al parsear expiración del plan: %v", err)
	}

	if time.Now().After(expiresAt) {
		// Marcar como expirado
		_, _ = db.Exec("UPDATE pending_confirmations SET status = 'expired' WHERE plan_id = ?", planID)
		return nil, "", nil
	}

	var plan planner.Plan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return nil, "", fmt.Errorf("error al deserializar plan: %v", err)
	}

	return &plan, reason, nil
}

// DeletePendingPlan actualiza el estado de un plan pendiente en la base de datos (e.g. 'accepted', 'cancelled').
func DeletePendingPlan(db *sql.DB, planID string, status string) error {
	query := `UPDATE pending_confirmations SET status = ? WHERE plan_id = ?`
	_, err := db.Exec(query, status, planID)
	return err
}
