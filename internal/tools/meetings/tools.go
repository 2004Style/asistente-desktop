package meetings

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/timeparser"
)

// AddMeetingTool crea una nueva reunión
type AddMeetingTool struct {
	repo   *Repository
	config *config.Config
}

func NewAddMeetingTool(repo *Repository, cfg *config.Config) *AddMeetingTool {
	return &AddMeetingTool{repo: repo, config: cfg}
}

func (t *AddMeetingTool) Name() string { return "meetings.add" }
func (t *AddMeetingTool) Description() string {
	return "Programa una reunión en el calendario local. Soporta fecha/hora en lenguaje natural en español."
}
func (t *AddMeetingTool) Category() string  { return "productivity" }
func (t *AddMeetingTool) RiskLevel() string { return "low" }
func (t *AddMeetingTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Título o motivo de la reunión.",
			},
			"starts_at": map[string]interface{}{
				"type":        "string",
				"description": "Fecha y hora de inicio (ej: 'hoy a las 4pm', 'el lunes a las 11:30', o RFC3339).",
			},
			"ends_at": map[string]interface{}{
				"type":        "string",
				"description": "Fecha y hora de término (opcional).",
			},
			"location": map[string]interface{}{
				"type":        "string",
				"description": "Ubicación o enlace a la sala virtual (opcional).",
			},
			"notify_before": map[string]interface{}{
				"type":        "integer",
				"description": "Minutos antes del inicio para notificar (por defecto 10).",
			},
		},
		"required": []string{"title", "starts_at"},
	}
}

func (t *AddMeetingTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	title, _ := args["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("el argumento 'title' es obligatorio")
	}
	startsAtStr, _ := args["starts_at"].(string)
	if startsAtStr == "" {
		return nil, fmt.Errorf("el argumento 'starts_at' es obligatorio")
	}
	endsAtStr, _ := args["ends_at"].(string)
	location, _ := args["location"].(string)

	notifyBefore := 10
	if val, ok := args["notify_before"]; ok {
		switch v := val.(type) {
		case float64:
			notifyBefore = int(v)
		case int:
			notifyBefore = v
		}
	} else if t.config != nil && t.config.Meetings.DefaultNotifyBeforeMinutes > 0 {
		notifyBefore = t.config.Meetings.DefaultNotifyBeforeMinutes
	}

	defTime := "09:00"
	if t.config != nil && t.config.Reminders.DefaultTime != "" {
		defTime = t.config.Reminders.DefaultTime
	}

	var parsedStartAt time.Time
	if parsedTime, err := time.Parse(time.RFC3339, startsAtStr); err == nil {
		parsedStartAt = parsedTime
	} else {
		res, err := timeparser.Parse(startsAtStr, defTime)
		if err != nil {
			return nil, fmt.Errorf("no se pudo parsear starts_at: %v", err)
		}
		parsedStartAt = res.StartAt
	}

	var parsedEndAtStr string
	if endsAtStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, endsAtStr); err == nil {
			parsedEndAtStr = parsedTime.UTC().Format(time.RFC3339)
		} else {
			res, err := timeparser.Parse(endsAtStr, defTime)
			if err != nil {
				return nil, fmt.Errorf("no se pudo parsear ends_at: %v", err)
			}
			parsedEndAtStr = res.StartAt.UTC().Format(time.RFC3339)
		}
	}

	startUTCStr := parsedStartAt.UTC().Format(time.RFC3339)

	m, err := t.repo.Create(ctx, title, startUTCStr, parsedEndAtStr, location, "local", "", notifyBefore)
	if err != nil {
		return nil, fmt.Errorf("error al guardar la reunión: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}
	startFormatted := m.StartsAt.In(loc).Format("2006-01-02 15:04:05")

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Reunión %d programada: %q para el %s (notificar %d minutos antes).", m.ID, m.Title, startFormatted, m.NotifyBeforeMinutes),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ListMeetingsTool enumera todas las reuniones
type ListMeetingsTool struct {
	repo   *Repository
	config *config.Config
}

func NewListMeetingsTool(repo *Repository, cfg *config.Config) *ListMeetingsTool {
	return &ListMeetingsTool{repo: repo, config: cfg}
}

func (t *ListMeetingsTool) Name() string { return "meetings.list" }
func (t *ListMeetingsTool) Description() string {
	return "Lista las reuniones programadas (por defecto programadas y activas)."
}
func (t *ListMeetingsTool) Category() string  { return "productivity" }
func (t *ListMeetingsTool) RiskLevel() string { return "low" }
func (t *ListMeetingsTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Filtrar por estado: scheduled, active, completed, cancelled, o all (por defecto scheduled y active).",
				"enum":        []interface{}{"scheduled", "active", "completed", "cancelled", "all"},
			},
		},
	}
}

func (t *ListMeetingsTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	status, _ := args["status"].(string)

	var statuses []string
	if status == "all" {
		statuses = []string{}
	} else if status != "" {
		statuses = []string{status}
	} else {
		statuses = []string{"scheduled", "active"}
	}

	list, err := t.repo.List(ctx, statuses)
	if err != nil {
		return nil, fmt.Errorf("error al listar reuniones: %v", err)
	}

	if len(list) == 0 {
		return &executor.ToolResult{
			Success:    true,
			Text:       "No se encontraron reuniones en la agenda.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}

	text := "Agenda de reuniones:\n"
	for _, m := range list {
		endMsg := ""
		if m.EndsAt != nil {
			endMsg = fmt.Sprintf(" - %s", m.EndsAt.In(loc).Format("15:04"))
		}
		locMsg := ""
		if m.Location != "" {
			locMsg = fmt.Sprintf(" en %s", m.Location)
		}
		text += fmt.Sprintf("[%s] ID %d: %q el %s%s%s\n",
			m.Status, m.ID, m.Title, m.StartsAt.In(loc).Format("2006-01-02 15:04"), endMsg, locMsg)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// TodayMeetingsTool enumera las reuniones de hoy
type TodayMeetingsTool struct {
	repo   *Repository
	config *config.Config
}

func NewTodayMeetingsTool(repo *Repository, cfg *config.Config) *TodayMeetingsTool {
	return &TodayMeetingsTool{repo: repo, config: cfg}
}

func (t *TodayMeetingsTool) Name() string { return "meetings.today" }
func (t *TodayMeetingsTool) Description() string {
	return "Muestra el resumen de las reuniones agendadas para el día de hoy."
}
func (t *TodayMeetingsTool) Category() string  { return "productivity" }
func (t *TodayMeetingsTool) RiskLevel() string { return "low" }
func (t *TodayMeetingsTool) Schema() map[string]interface{} {
	return map[string]interface{}{}
}

func (t *TodayMeetingsTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	list, err := t.repo.List(ctx, []string{"scheduled", "active"})
	if err != nil {
		return nil, fmt.Errorf("error al buscar reuniones: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)

	var todayList []Meeting
	for _, m := range list {
		mStart := m.StartsAt.In(loc)
		if (mStart.After(todayStart) || mStart.Equal(todayStart)) && (mStart.Before(todayEnd) || mStart.Equal(todayEnd)) {
			todayList = append(todayList, m)
		}
	}

	if len(todayList) == 0 {
		return &executor.ToolResult{
			Success:    true,
			Text:       "No tiene reuniones agendadas para hoy, señor.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	text := "Sus reuniones para hoy son:\n"
	for _, m := range todayList {
		endMsg := ""
		if m.EndsAt != nil {
			endMsg = fmt.Sprintf(" hasta las %s", m.EndsAt.In(loc).Format("15:04"))
		}
		locMsg := ""
		if m.Location != "" {
			locMsg = fmt.Sprintf(" (Ubicación: %s)", m.Location)
		}
		text += fmt.Sprintf("- %s: %q%s%s\n", m.StartsAt.In(loc).Format("15:04"), m.Title, endMsg, locMsg)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// NextMeetingTool devuelve la siguiente reunión pendiente
type NextMeetingTool struct {
	repo   *Repository
	config *config.Config
}

func NewNextMeetingTool(repo *Repository, cfg *config.Config) *NextMeetingTool {
	return &NextMeetingTool{repo: repo, config: cfg}
}

func (t *NextMeetingTool) Name() string { return "meetings.next" }
func (t *NextMeetingTool) Description() string {
	return "Muestra los detalles de la siguiente reunión más próxima en su agenda."
}
func (t *NextMeetingTool) Category() string  { return "productivity" }
func (t *NextMeetingTool) RiskLevel() string { return "low" }
func (t *NextMeetingTool) Schema() map[string]interface{} {
	return map[string]interface{}{}
}

func (t *NextMeetingTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	list, err := t.repo.List(ctx, []string{"scheduled"})
	if err != nil {
		return nil, fmt.Errorf("error al buscar reuniones: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)

	var next *Meeting
	for _, m := range list {
		mStart := m.StartsAt.In(loc)
		if mStart.After(now) {
			next = &m
			break
		}
	}

	if next == nil {
		return &executor.ToolResult{
			Success:    true,
			Text:       "No tiene reuniones pendientes próximas, señor.",
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	locMsg := ""
	if next.Location != "" {
		locMsg = fmt.Sprintf(" en %s", next.Location)
	}
	timeUntil := next.StartsAt.Sub(time.Now())
	minsUntil := int(timeUntil.Minutes())

	text := fmt.Sprintf("Su próxima reunión es: %q\nFecha: %s%s\nEmpieza en aproximadamente %d minutos.",
		next.Title, next.StartsAt.In(loc).Format("2006-01-02 15:04"), locMsg, minsUntil)

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// CancelMeetingTool cancela una reunión programada
type CancelMeetingTool struct {
	repo *Repository
}

func NewCancelMeetingTool(repo *Repository) *CancelMeetingTool {
	return &CancelMeetingTool{repo: repo}
}

func (t *CancelMeetingTool) Name() string { return "meetings.cancel" }
func (t *CancelMeetingTool) Description() string {
	return "Cancela una reunión programada por su ID."
}
func (t *CancelMeetingTool) Category() string  { return "productivity" }
func (t *CancelMeetingTool) RiskLevel() string { return "low" }
func (t *CancelMeetingTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID de la reunión a cancelar.",
			},
		},
		"required": []string{"id"},
	}
}

func (t *CancelMeetingTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	var id int64
	switch v := args["id"].(type) {
	case float64:
		id = int64(v)
	case int:
		id = int64(v)
	case int64:
		id = v
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ID inválido: %v", err)
		}
		id = parsed
	default:
		return nil, fmt.Errorf("el ID debe ser un número entero")
	}

	m, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró la reunión con ID %d", id)
	}

	err = t.repo.UpdateStatus(ctx, id, "cancelled")
	if err != nil {
		return nil, fmt.Errorf("error al cancelar la reunión: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Reunión %d: %q ha sido cancelada.", id, m.Title),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
