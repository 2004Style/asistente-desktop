package tasks

import (
	"context"
	"database/sql"
	"time"
)

type Task struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueAt       *time.Time `json:"due_at"`
	Source      string     `json:"source"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, title, desc, priority, dueAtStr, source string) (*Task, error) {
	if priority == "" {
		priority = "normal"
	}
	if source == "" {
		source = "voice"
	}

	var dueVal sql.NullString
	if dueAtStr != "" {
		dueVal.String = dueAtStr
		dueVal.Valid = true
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (title, description, priority, due_at, source, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		title, desc, priority, dueVal, source,
	)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.Get(ctx, id)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Task, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, title, description, status, priority, due_at, source, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	)

	var t Task
	var desc sql.NullString
	var dueVal sql.NullString
	var createdStr, updatedStr string

	err := row.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &dueVal, &t.Source, &createdStr, &updatedStr)
	if err != nil {
		return nil, err
	}

	t.Description = desc.String
	if dueVal.Valid && dueVal.String != "" {
		parsed, err := time.Parse(time.RFC3339, dueVal.String)
		if err == nil {
			t.DueAt = &parsed
		} else {
			// fallback sqlite datetime format
			parsed, err = time.Parse("2006-01-02 15:04:05", dueVal.String)
			if err == nil {
				t.DueAt = &parsed
			}
		}
	}

	if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
		t.CreatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
			t.CreatedAt = parsed
		}
	}

	if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
		t.UpdatedAt = parsed
	} else {
		if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
			t.UpdatedAt = parsed
		}
	}

	return &t, nil
}

func (r *Repository) List(ctx context.Context, statuses []string) ([]Task, error) {
	query := `SELECT id, title, description, status, priority, due_at, source, created_at, updated_at FROM tasks`
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
	query += " ORDER BY due_at ASC, priority DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Task
	for rows.Next() {
		var t Task
		var desc sql.NullString
		var dueVal sql.NullString
		var createdStr, updatedStr string

		err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &dueVal, &t.Source, &createdStr, &updatedStr)
		if err != nil {
			return nil, err
		}

		t.Description = desc.String
		if dueVal.Valid && dueVal.String != "" {
			parsed, err := time.Parse(time.RFC3339, dueVal.String)
			if err == nil {
				t.DueAt = &parsed
			} else {
				parsed, err = time.Parse("2006-01-02 15:04:05", dueVal.String)
				if err == nil {
					t.DueAt = &parsed
				}
			}
		}

		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			t.CreatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", createdStr); err == nil {
				t.CreatedAt = parsed
			}
		}

		if parsed, err := time.Parse(time.RFC3339, updatedStr); err == nil {
			t.UpdatedAt = parsed
		} else {
			if parsed, err := time.Parse("2006-01-02 15:04:05", updatedStr); err == nil {
				t.UpdatedAt = parsed
			}
		}

		list = append(list, t)
	}

	return list, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	return err
}

func (r *Repository) UpdatePriority(ctx context.Context, id int64, priority string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET priority = ?, updated_at = datetime('now') WHERE id = ?`,
		priority, id,
	)
	return err
}

func (r *Repository) Reschedule(ctx context.Context, id int64, dueAtStr string) error {
	var dueVal sql.NullString
	if dueAtStr != "" {
		dueVal.String = dueAtStr
		dueVal.Valid = true
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET due_at = ?, updated_at = datetime('now') WHERE id = ?`,
		dueVal, id,
	)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	return err
}
