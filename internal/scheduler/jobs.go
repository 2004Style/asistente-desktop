package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"rbot/internal/planner"
)

// DispatchJob despacha un trabajo específico según su tipo y datos.
func (s *Scheduler) DispatchJob(ctx context.Context, jobType, payloadJSON string) error {
	switch jobType {
	case "reminder.notify":
		return s.handleReminderNotify(ctx, payloadJSON)
	case "meeting.notify":
		return s.handleMeetingNotify(ctx, payloadJSON)
	case "task.due_notify":
		return s.handleTaskDueNotify(ctx, payloadJSON)
	case "tool.execute":
		return s.handleToolExecute(ctx, payloadJSON)
	default:
		return fmt.Errorf("tipo de trabajo no soportado: %s", jobType)
	}
}

func (s *Scheduler) handleReminderNotify(ctx context.Context, payloadJSON string) error {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("error unmarshaling payload: %v", err)
	}

	ridVal, ok := payload["reminder_id"]
	if !ok {
		return fmt.Errorf("reminder_id faltante en payload")
	}

	var rid int64
	switch v := ridVal.(type) {
	case float64:
		rid = int64(v)
	case int64:
		rid = v
	default:
		return fmt.Errorf("reminder_id de tipo no válido")
	}

	// Cargar el recordatorio
	row := s.db.QueryRowContext(ctx,
		`SELECT title, message, remind_at, recurrence_rule, channels_json, priority
		 FROM reminders WHERE id = ?`, rid,
	)

	var title, msg, remindAtStr, recurStr, channelsStr, priority string
	var messageVal sql.NullString
	var recurVal sql.NullString
	var channelsVal sql.NullString

	if err := row.Scan(&title, &messageVal, &remindAtStr, &recurVal, &channelsVal, &priority); err != nil {
		return fmt.Errorf("error al obtener recordatorio %d: %v", rid, err)
	}

	msg = messageVal.String
	recurStr = recurVal.String
	channelsStr = channelsVal.String

	// Decodificar canales
	var channels []string
	if channelsStr != "" {
		_ = json.Unmarshal([]byte(channelsStr), &channels)
	}
	if len(channels) == 0 {
		channels = []string{"default"}
	}

	isUrgent := priority == "urgent"

	body := msg
	if body == "" {
		body = title
	}

	// Enviar notificaciones
	for _, ch := range channels {
		_ = s.notifMgr.Send(ctx, ch, title, body, isUrgent)
	}

	// Calcular recurrencia si aplica
	if recurStr != "" {
		remindAt, err := time.Parse(time.RFC3339, remindAtStr)
		if err != nil {
			remindAt, _ = time.Parse("2006-01-02 15:04:05", remindAtStr)
		}

		nextTime, err := ParseRRULEAndGetNext(recurStr, remindAt, time.Now().UTC())
		if err == nil {
			nextUTCStr := nextTime.UTC().Format(time.RFC3339)

			tx, err := s.db.BeginTx(ctx, nil)
			if err == nil {
				defer tx.Rollback()

				// Actualizar recordatorio
				_, err = tx.ExecContext(ctx,
					`UPDATE reminders SET remind_at = ?, status = 'scheduled', updated_at = datetime('now') WHERE id = ?`,
					nextUTCStr, rid,
				)
				if err == nil {
					// Crear nuevo job programado
					newPayloadBytes, _ := json.Marshal(map[string]interface{}{"reminder_id": rid})
					_, err = tx.ExecContext(ctx,
						`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
						 VALUES ('reminder.notify', ?, ?, 'pending', datetime('now'), datetime('now'))`,
						string(newPayloadBytes), nextUTCStr,
					)
					if err == nil {
						_ = tx.Commit()
					}
				}
			}
		} else {
			log.Printf("[Scheduler] Error al calcular recurrencia del recordatorio %d: %v", rid, err)
			_, _ = s.db.ExecContext(ctx, `UPDATE reminders SET status = 'triggered', updated_at = datetime('now') WHERE id = ?`, rid)
		}
	} else {
		_, _ = s.db.ExecContext(ctx, `UPDATE reminders SET status = 'triggered', updated_at = datetime('now') WHERE id = ?`, rid)
	}

	return nil
}

func (s *Scheduler) handleMeetingNotify(ctx context.Context, payloadJSON string) error {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("error unmarshaling payload: %v", err)
	}

	midVal, ok := payload["meeting_id"]
	if !ok {
		return fmt.Errorf("meeting_id faltante en payload")
	}

	var mid int64
	switch v := midVal.(type) {
	case float64:
		mid = int64(v)
	case int64:
		mid = v
	}

	row := s.db.QueryRowContext(ctx, `SELECT title, starts_at, location FROM meetings WHERE id = ?`, mid)
	var title, startStr, location string
	var locVal sql.NullString
	if err := row.Scan(&title, &startStr, &locVal); err != nil {
		return fmt.Errorf("error al obtener reunión %d: %v", mid, err)
	}
	location = locVal.String

	// Enviar la notificación de la reunión
	locMsg := ""
	if location != "" {
		locMsg = fmt.Sprintf(" en %s", location)
	}
	message := fmt.Sprintf("Su reunión %q comenzará pronto%s.", title, locMsg)
	_ = s.notifMgr.Send(ctx, "default", "Reunión Próxima", message, true)

	// Cambiar estado de reunión a active
	_, _ = s.db.ExecContext(ctx, `UPDATE meetings SET status = 'active', updated_at = datetime('now') WHERE id = ?`, mid)

	return nil
}

func (s *Scheduler) handleTaskDueNotify(ctx context.Context, payloadJSON string) error {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("error unmarshaling payload: %v", err)
	}

	tidVal, ok := payload["task_id"]
	if !ok {
		return fmt.Errorf("task_id faltante en payload")
	}

	var tid int64
	switch v := tidVal.(type) {
	case float64:
		tid = int64(v)
	case int64:
		tid = v
	}

	row := s.db.QueryRowContext(ctx, `SELECT title FROM tasks WHERE id = ?`, tid)
	var title string
	if err := row.Scan(&title); err != nil {
		return fmt.Errorf("error al obtener tarea %d: %v", tid, err)
	}

	message := fmt.Sprintf("La tarea %q ha vencido.", title)
	_ = s.notifMgr.Send(ctx, "default", "Tarea Vencida", message, false)

	// Cambiar estado a expired
	_, _ = s.db.ExecContext(ctx, `UPDATE tasks SET status = 'expired', updated_at = datetime('now') WHERE id = ?`, tid)

	return nil
}

func (s *Scheduler) handleToolExecute(ctx context.Context, payloadJSON string) error {
	var payload struct {
		ToolName  string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("error unmarshaling tool execute payload: %v", err)
	}

	if payload.ToolName == "" {
		return fmt.Errorf("nombre de herramienta vacío")
	}

	// 1. Obtener la herramienta desde el Registry
	tool, ok := s.execObj.Registry.Get(payload.ToolName)
	if !ok {
		return fmt.Errorf("herramienta no registrada: %s", payload.ToolName)
	}

	// 2. Validar con el PolicyEngine
	decision := s.polEngine.EvaluateTool(ctx, tool, payload.Arguments)
	if !decision.Allowed {
		msg := fmt.Sprintf("Ejecución programada de '%s' denegada por seguridad: %s", payload.ToolName, decision.Reason)
		_ = s.notifMgr.Send(ctx, "default", "Seguridad de Tarea Programada", msg, true)
		return fmt.Errorf("denegado por motor de políticas: %s", decision.Reason)
	}

	if decision.RequiresConfirm {
		msg := fmt.Sprintf("Ejecución programada de '%s' cancelada: Requiere confirmación explícita del usuario y no puede correr en segundo plano.", payload.ToolName)
		_ = s.notifMgr.Send(ctx, "default", "Seguridad de Tarea Programada", msg, true)
		return fmt.Errorf("ejecución en segundo plano requiere confirmación")
	}

	// 3. Crear Plan de un solo paso y ejecutar
	step := planner.PlanStep{
		ID:        "scheduled-step",
		ToolName:  payload.ToolName,
		Args:      payload.Arguments,
		TimeoutMs: 20000,
	}
	plan := planner.Plan{
		ID:           fmt.Sprintf("sched-%d", time.Now().UnixNano()),
		UserInput:    "Ejecución de trabajo programado",
		Intent:       "scheduled_execution",
		Confidence:   1.0,
		RiskLevel:    decision.RiskLevel,
		NeedsConfirm: false,
		Steps:        []planner.PlanStep{step},
	}

	res, err := s.execObj.ExecutePlan(ctx, plan)
	if err != nil {
		return err
	}

	if !res.Success {
		return fmt.Errorf("fallo al ejecutar la herramienta: %s", res.Error)
	}

	return nil
}
