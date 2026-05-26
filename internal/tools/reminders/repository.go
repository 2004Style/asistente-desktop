package reminders

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Reminder struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	Message        string     `json:"message"`
	RemindAt       time.Time  `json:"remind_at"`
	RecurrenceRule string     `json:"recurrence_rule"`
	ChannelsJSON   string     `json:"channels_json"`
	Priority       string     `json:"priority"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, title, message, remindAtStr, recurrenceRule, channelsJSON, priority, source string) (*Reminder, error) {
	if priority == "" {
		priority = "normal"
	}
	if source == "" {
		source = "voice"
	}
	if channelsJSON == "" {
		channelsJSON = `["desktop","voice","hud"]`
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO reminders (title, message, remind_at, recurrence_rule, channels_json, priority, status, source, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'scheduled', ?, datetime('now'), datetime('now'))`,
		title, message, remindAtStr, recurrenceRule, channelsJSON, priority, source,
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// También insertar en la tabla scheduled_jobs
	payload := map[string]interface{}{
		"reminder_id": id,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
		 VALUES ('reminder.notify', ?, ?, 'pending', datetime('now'), datetime('now'))`,
		string(payloadBytes), remindAtStr,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.Get(ctx, id)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Reminder, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, message, remind_at, recurrence_rule, channels_json, priority, status, source, created_at, updated_at
		 FROM reminders WHERE id = ?`, id,
	)

	var rem Reminder
	var msg sql.NullString
	var recur sql.NullString
	var channels sql.NullString
	var remindAtStr, createdStr, updatedStr string

	err := row.Scan(&rem.ID, &rem.Title, &msg, &remindAtStr, &recur, &channels, &rem.Priority, &rem.Status, &rem.Source, &createdStr, &updatedStr)
	if err != nil {
		return nil, err
	}

	rem.Message = msg.String
	rem.RecurrenceRule = recur.String
	rem.ChannelsJSON = channels.String

	if parsed, err := time.Parse(time.RFC3339, remindAtStr); err == nil {
		rem.RemindAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", remindAtStr); err == nil {
			rem.RemindAt = parsed
		}
	}

	if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
		rem.CreatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
			rem.CreatedAt = parsed
		}
	}

	if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
		rem.UpdatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
			rem.UpdatedAt = parsed
		}
	}

	return &rem, nil
}

func (r *Repository) List(ctx context.Context, statuses []string) ([]Reminder, error) {
	query := `SELECT id, title, message, remind_at, recurrence_rule, channels_json, priority, status, source, created_at, updated_at FROM reminders`
	var args []interface{}
	if len(statuses) > 0 {
		query += " WHERE status IN ("
		for i, status := range statuses {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, status)
		}
		query += ")"
	}
	query += " ORDER BY remind_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Reminder
	for rows.Next() {
		var rem Reminder
		var msg sql.NullString
		var recur sql.NullString
		var channels sql.NullString
		var remindAtStr, createdStr, updatedStr string

		err := rows.Scan(&rem.ID, &rem.Title, &msg, &remindAtStr, &recur, &channels, &rem.Priority, &rem.Status, &rem.Source, &createdStr, &updatedStr)
		if err != nil {
			return nil, err
		}

		rem.Message = msg.String
		rem.RecurrenceRule = recur.String
		rem.ChannelsJSON = channels.String

		if parsed, err := time.Parse(time.RFC3339, remindAtStr); err == nil {
			rem.RemindAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", remindAtStr); err == nil {
				rem.RemindAt = parsed
			}
		}

		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			rem.CreatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
				rem.CreatedAt = parsed
			}
		}

		if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			rem.UpdatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
				rem.UpdatedAt = parsed
			}
		}

		list = append(list, rem)
	}

	return list, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE reminders SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return err
	}

	// Si se canceló el recordatorio, también cancelar el scheduled_job pendiente
	if status == "cancelled" {
		payloadLike := `%"reminder_id":` + jsonStringifyInt(id) + `}%`
		_, err = tx.ExecContext(ctx,
			`UPDATE scheduled_jobs SET status = 'cancelled', updated_at = datetime('now')
			 WHERE job_type = 'reminder.notify' AND status = 'pending' AND payload_json LIKE ?`,
			payloadLike,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) Reschedule(ctx context.Context, id int64, remindAtStr string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE reminders SET remind_at = ?, status = 'scheduled', updated_at = datetime('now') WHERE id = ?`,
		remindAtStr, id,
	)
	if err != nil {
		return err
	}

	// Cancelar el job anterior
	payloadLike := `%"reminder_id":` + jsonStringifyInt(id) + `}%`
	_, err = tx.ExecContext(ctx,
		`UPDATE scheduled_jobs SET status = 'cancelled', updated_at = datetime('now')
		 WHERE job_type = 'reminder.notify' AND status = 'pending' AND payload_json LIKE ?`,
		payloadLike,
	)
	if err != nil {
		return err
	}

	// Insertar un nuevo job
	payload := map[string]interface{}{
		"reminder_id": id,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
		 VALUES ('reminder.notify', ?, ?, 'pending', datetime('now'), datetime('now'))`,
		string(payloadBytes), remindAtStr,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func jsonStringifyInt(id int64) string {
	b, _ := json.Marshal(id)
	return string(b)
}
