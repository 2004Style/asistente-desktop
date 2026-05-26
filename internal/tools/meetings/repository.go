package meetings

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Meeting struct {
	ID                  int64      `json:"id"`
	Title               string     `json:"title"`
	StartsAt            time.Time  `json:"starts_at"`
	EndsAt              *time.Time `json:"ends_at"`
	Location            string     `json:"location"`
	Source              string     `json:"source"`
	ExternalID          string     `json:"external_id"`
	NotifyBeforeMinutes int        `json:"notify_before_minutes"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, title, startsAtStr, endsAtStr, location, source, externalID string, notifyBeforeMins int) (*Meeting, error) {
	if source == "" {
		source = "local"
	}
	if notifyBeforeMins <= 0 {
		notifyBeforeMins = 10
	}

	var endVal sql.NullString
	if endsAtStr != "" {
		endVal.String = endsAtStr
		endVal.Valid = true
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO meetings (title, starts_at, ends_at, location, source, external_id, notify_before_minutes, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'scheduled', datetime('now'), datetime('now'))`,
		title, startsAtStr, endVal, location, source, externalID, notifyBeforeMins,
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Calcular run_at para el scheduled_job (starts_at - notifyBeforeMins)
	startTime, err := time.Parse(time.RFC3339, startsAtStr)
	if err == nil {
		runAt := startTime.Add(-time.Duration(notifyBeforeMins) * time.Minute)
		runAtStr := runAt.Format(time.RFC3339)

		payload := map[string]interface{}{
			"meeting_id": id,
		}
		payloadBytes, _ := json.Marshal(payload)

		_, err = tx.ExecContext(ctx,
			`INSERT INTO scheduled_jobs (job_type, payload_json, run_at, status, created_at, updated_at)
			 VALUES ('meeting.notify', ?, ?, 'pending', datetime('now'), datetime('now'))`,
			string(payloadBytes), runAtStr,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.Get(ctx, id)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, starts_at, ends_at, location, source, external_id, notify_before_minutes, status, created_at, updated_at
		 FROM meetings WHERE id = ?`, id,
	)

	var m Meeting
	var endVal sql.NullString
	var loc sql.NullString
	var ext sql.NullString
	var startStr, createdStr, updatedStr string

	err := row.Scan(&m.ID, &m.Title, &startStr, &endVal, &loc, &m.Source, &ext, &m.NotifyBeforeMinutes, &m.Status, &createdStr, &updatedStr)
	if err != nil {
		return nil, err
	}

	m.Location = loc.String
	m.ExternalID = ext.String

	if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
		m.StartsAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", startStr); err == nil {
			m.StartsAt = parsed
		}
	}

	if endVal.Valid && endVal.String != "" {
		if parsed, err := time.Parse(time.RFC3339, endVal.String); err == nil {
			m.EndsAt = &parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", endVal.String); err == nil {
				m.EndsAt = &parsed
			}
		}
	}

	if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
		m.CreatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
			m.CreatedAt = parsed
		}
	}

	if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
		m.UpdatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
			m.UpdatedAt = parsed
		}
	}

	return &m, nil
}

func (r *Repository) List(ctx context.Context, statuses []string) ([]Meeting, error) {
	query := `SELECT id, title, starts_at, ends_at, location, source, external_id, notify_before_minutes, status, created_at, updated_at FROM meetings`
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
	query += " ORDER BY starts_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Meeting
	for rows.Next() {
		var m Meeting
		var endVal sql.NullString
		var loc sql.NullString
		var ext sql.NullString
		var startStr, createdStr, updatedStr string

		err := rows.Scan(&m.ID, &m.Title, &startStr, &endVal, &loc, &m.Source, &ext, &m.NotifyBeforeMinutes, &m.Status, &createdStr, &updatedStr)
		if err != nil {
			return nil, err
		}

		m.Location = loc.String
		m.ExternalID = ext.String

		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			m.StartsAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", startStr); err == nil {
				m.StartsAt = parsed
			}
		}

		if endVal.Valid && endVal.String != "" {
			if parsed, err := time.Parse(time.RFC3339, endVal.String); err == nil {
				m.EndsAt = &parsed
			} else {
				if parsed, err := time.Parse("2006-01-02 15:04:05", endVal.String); err == nil {
					m.EndsAt = &parsed
				}
			}
		}

		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			m.CreatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
				m.CreatedAt = parsed
			}
		}

		if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			m.UpdatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
				m.UpdatedAt = parsed
			}
		}

		list = append(list, m)
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
		`UPDATE meetings SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return err
	}

	// Si se canceló la reunión, también cancelar el scheduled_job pendiente
	if status == "cancelled" {
		payloadLike := `%"meeting_id":` + jsonStringifyInt(id) + `}%`
		_, err = tx.ExecContext(ctx,
			`UPDATE scheduled_jobs SET status = 'cancelled', updated_at = datetime('now')
			 WHERE job_type = 'meeting.notify' AND status = 'pending' AND payload_json LIKE ?`,
			payloadLike,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM meetings WHERE id = ?`, id)
	if err != nil {
		return err
	}

	// Eliminar scheduled_jobs pendientes para esta reunión
	payloadLike := `%"meeting_id":` + jsonStringifyInt(id) + `}%`
	_, err = tx.ExecContext(ctx,
		`DELETE FROM scheduled_jobs WHERE job_type = 'meeting.notify' AND status = 'pending' AND payload_json LIKE ?`,
		payloadLike,
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
