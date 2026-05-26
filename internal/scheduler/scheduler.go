package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/policy"
	"rbot/internal/tools/notifications"
)

type Scheduler struct {
	db        *sql.DB
	config    *config.Config
	notifMgr  *notifications.NotificationManager
	execObj   *executor.Executor
	polEngine *policy.Engine
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

type jobInfo struct {
	id          int64
	jobType     string
	payloadJSON string
	runAt       time.Time
	attempts    int
	maxAttempts int
}

func NewScheduler(db *sql.DB, cfg *config.Config, nm *notifications.NotificationManager, ex *executor.Executor, pol *policy.Engine) *Scheduler {
	return &Scheduler{
		db:        db,
		config:    cfg,
		notifMgr:  nm,
		execObj:   ex,
		polEngine: pol,
		stopChan:  make(chan struct{}),
	}
}

// Start arranca el bucle de polling en una goroutine
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("[Scheduler] Iniciando motor de tareas programadas persistentes...")

		// Ejecutar recuperación en el arranque
		recoverySummary, err := s.ProcessRecovery(ctx)
		if err != nil {
			log.Printf("[Scheduler Recovery] Error al procesar recuperación: %v", err)
		} else if recoverySummary != "" {
			log.Printf("[Scheduler Recovery] %s", recoverySummary)
			// Enviar resumen al usuario
			_ = s.notifMgr.Send(ctx, "default", "Resumen de Inicio", recoverySummary, false)
		}

		tickInterval := 15 * time.Second
		if s.config != nil && s.config.Scheduler.TickSeconds > 0 {
			tickInterval = time.Duration(s.config.Scheduler.TickSeconds) * time.Second
		}
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				log.Println("[Scheduler] Deteniendo planificador...")
				return
			case <-ctx.Done():
				log.Println("[Scheduler] Contexto cancelado, deteniendo planificador...")
				return
			case <-ticker.C:
				s.pollAndExecuteJobs(ctx)
			}
		}
	}()
}

// Stop cancela el bucle del planificador de forma limpia
func (s *Scheduler) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	log.Println("[Scheduler] Planificador detenido con éxito.")
}

func (s *Scheduler) pollAndExecuteJobs(ctx context.Context) {
	jobs, err := s.lockAndGetPendingJobs(ctx)
	if err != nil {
		log.Printf("[Scheduler] Error al consultar/bloquear trabajos pendientes: %v", err)
		return
	}

	for _, job := range jobs {
		s.wg.Add(1)
		go func(ji jobInfo) {
			defer s.wg.Done()
			log.Printf("[Scheduler] Ejecutando trabajo %d (%s) programado para %v...", ji.id, ji.jobType, ji.runAt)

			err := s.DispatchJob(ctx, ji.jobType, ji.payloadJSON)

			finalStatus := "completed"
			var errStr sql.NullString
			if err != nil {
				log.Printf("[Scheduler] Error al ejecutar trabajo %d: %v", ji.id, err)
				finalStatus = "failed"
				errStr.String = err.Error()
				errStr.Valid = true
			}

			// Actualizar base de datos
			_, updateErr := s.db.ExecContext(ctx,
				`UPDATE scheduled_jobs 
				 SET status = ?, completed_at = ?, last_error = ?, updated_at = datetime('now') 
				 WHERE id = ?`,
				finalStatus, time.Now().UTC().Format(time.RFC3339), errStr, ji.id,
			)
			if updateErr != nil {
				log.Printf("[Scheduler] Error al actualizar estado del trabajo %d: %v", ji.id, updateErr)
			}
		}(job)
	}
}

func (s *Scheduler) lockAndGetPendingJobs(ctx context.Context) ([]jobInfo, error) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	pid := os.Getpid()
	hostname, _ := os.Hostname()
	lockedBy := fmt.Sprintf("daemon-%d-%s", pid, hostname)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Obtener los IDs de los trabajos pendientes vencidos
	rows, err := tx.QueryContext(ctx,
		`SELECT id, job_type, payload_json, run_at, attempts, max_attempts
		 FROM scheduled_jobs
		 WHERE status = 'pending' AND run_at <= ? AND locked_at IS NULL`,
		nowStr,
	)
	if err != nil {
		return nil, err
	}

	var jobs []jobInfo
	for rows.Next() {
		var ji jobInfo
		var runAtStr string
		if err := rows.Scan(&ji.id, &ji.jobType, &ji.payloadJSON, &runAtStr, &ji.attempts, &ji.maxAttempts); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, runAtStr); err == nil {
			ji.runAt = t
		} else {
			if t, err := time.Parse("2006-01-02 15:04:05", runAtStr); err == nil {
				ji.runAt = t
			}
		}
		jobs = append(jobs, ji)
	}
	rows.Close()

	if len(jobs) == 0 {
		return nil, nil
	}

	// 2. Marcar cada uno como bloqueado (running)
	for _, ji := range jobs {
		_, err = tx.ExecContext(ctx,
			`UPDATE scheduled_jobs
			 SET status = 'running', locked_at = ?, locked_by = ?, attempts = attempts + 1, updated_at = datetime('now')
			 WHERE id = ? AND status = 'pending' AND locked_at IS NULL`,
			nowStr, lockedBy, ji.id,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return jobs, nil
}
