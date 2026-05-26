package skills

import (
	"database/sql"
	"strconv"
)

func RecordFailure(db *sql.DB, name string, maxFailures int) (bool, error) {
	if maxFailures <= 0 {
		maxFailures = 3
	}

	key := "skill_failures:" + name
	var val string
	err := db.QueryRow("SELECT value FROM workspace_state WHERE key = ?", key).Scan(&val)
	failures := 0
	if err == nil {
		failures, _ = strconv.Atoi(val)
	}

	failures++

	if failures >= maxFailures {
		// Cambiar estado a quarantined y desactivar habilitación
		_, err = db.Exec("UPDATE skills SET status = 'quarantined', enabled = 0 WHERE name = ?", name)
		if err != nil {
			return false, err
		}
		// Resetear contador de fallos
		_, _ = db.Exec("DELETE FROM workspace_state WHERE key = ?", key)
		return true, nil
	}

	// Guardar el contador incrementado
	_, err = db.Exec("INSERT INTO workspace_state (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, strconv.Itoa(failures))
	return false, err
}

func ResetFailures(db *sql.DB, name string) error {
	key := "skill_failures:" + name
	_, err := db.Exec("DELETE FROM workspace_state WHERE key = ?", key)
	return err
}
