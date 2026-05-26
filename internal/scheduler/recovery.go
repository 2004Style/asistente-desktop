package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// ProcessRecovery escanea y recupera los trabajos atrasados o expirados.
// Retorna un resumen si hubo recordatorios/trabajos atrasados o expirados.
func (s *Scheduler) ProcessRecovery(ctx context.Context) (string, error) {
	now := time.Now().UTC()
	maxLateMins := 120
	if s.config != nil && s.config.Scheduler.MaxLateMinutes > 0 {
		maxLateMins = s.config.Scheduler.MaxLateMinutes
	}

	maxLateDuration := time.Duration(maxLateMins) * time.Minute

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, job_type, payload_json, run_at, attempts, max_attempts
		 FROM scheduled_jobs
		 WHERE status = 'pending' AND run_at < ? AND locked_at IS NULL`,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type jobInfo struct {
		id          int64
		jobType     string
		payloadJSON string
		runAt       time.Time
		attempts    int
		maxAttempts int
	}

	var jobsToProcess []jobInfo
	for rows.Next() {
		var ji jobInfo
		var runAtStr string
		if err := rows.Scan(&ji.id, &ji.jobType, &ji.payloadJSON, &runAtStr, &ji.attempts, &ji.maxAttempts); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, runAtStr); err == nil {
			ji.runAt = t
			jobsToProcess = append(jobsToProcess, ji)
		} else {
			if t, err := time.Parse("2006-01-02 15:04:05", runAtStr); err == nil {
				ji.runAt = t
				jobsToProcess = append(jobsToProcess, ji)
			}
		}
	}

	if len(jobsToProcess) == 0 {
		return "", nil
	}

	expiredCount := 0
	delayedCount := 0

	for _, job := range jobsToProcess {
		diff := now.Sub(job.runAt)
		if diff >= maxLateDuration {
			// Marcar como expirado
			_, err := s.db.ExecContext(ctx,
				`UPDATE scheduled_jobs SET status = 'expired', updated_at = datetime('now') WHERE id = ?`,
				job.id,
			)
			if err != nil {
				log.Printf("[Scheduler Recovery] Error al marcar job %d como expirado: %v", job.id, err)
			}
			expiredCount++

			if job.jobType == "reminder.notify" {
				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(job.payloadJSON), &payload); err == nil {
					if ridVal, ok := payload["reminder_id"]; ok {
						var rid int64
						switch v := ridVal.(type) {
						case float64:
							rid = int64(v)
						case int64:
							rid = v
						}
						_, _ = s.db.ExecContext(ctx,
							`UPDATE reminders SET status = 'expired', updated_at = datetime('now') WHERE id = ?`,
							rid,
						)
					}
				}
			}
		} else {
			delayedCount++
			go func(ji jobInfo) {
				pid := os.Getpid()
				hostname, _ := os.Hostname()
				lockedBy := fmt.Sprintf("recovery-%d-%s", pid, hostname)

				tx, err := s.db.BeginTx(ctx, nil)
				if err != nil {
					return
				}
				defer tx.Rollback()

				res, err := tx.ExecContext(ctx,
					`UPDATE scheduled_jobs 
					 SET status = 'running', locked_at = ?, locked_by = ?, updated_at = datetime('now')
					 WHERE id = ? AND status = 'pending' AND locked_at IS NULL`,
					now.Format(time.RFC3339), lockedBy, ji.id,
				)
				if err != nil {
					return
				}

				rowsAffected, err := res.RowsAffected()
				if err != nil || rowsAffected == 0 {
					return
				}

				if err := tx.Commit(); err != nil {
					return
				}

				// Despachar el trabajo
				err = s.DispatchJob(ctx, ji.jobType, ji.payloadJSON)

				finalStatus := "completed"
				var errStr sql.NullString
				if err != nil {
					finalStatus = "failed"
					errStr.String = err.Error()
					errStr.Valid = true
				}

				_, _ = s.db.ExecContext(ctx,
					`UPDATE scheduled_jobs 
					 SET status = ?, completed_at = ?, last_error = ?, updated_at = datetime('now') 
					 WHERE id = ?`,
					finalStatus, time.Now().UTC().Format(time.RFC3339), errStr, ji.id,
				)
			}(job)
		}
	}

	summary := ""
	if expiredCount > 0 || delayedCount > 0 {
		summary = fmt.Sprintf("Señor, mientras estuve fuera de línea vencieron %d tareas programadas. %d expiraron y %d se están ejecutando con retraso.",
			expiredCount+delayedCount, expiredCount, delayedCount)
	}

	return summary, nil
}
