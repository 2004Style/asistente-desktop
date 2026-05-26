package reminders

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/timeparser"
)

// AddReminderTool crea un nuevo recordatorio
type AddReminderTool struct {
	repo   *Repository
	config *config.Config
}

func NewAddReminderTool(repo *Repository, cfg *config.Config) *AddReminderTool {
	return &AddReminderTool{repo: repo, config: cfg}
}

func (t *AddReminderTool) Name() string { return "reminders.add" }
func (t *AddReminderTool) Description() string {
	return "Programa un nuevo recordatorio o alerta. Admite recordatorios recurrentes y expresiones de tiempo natural en español."
}
func (t *AddReminderTool) Category() string  { return "productivity" }
func (t *AddReminderTool) RiskLevel() string { return "low" }
func (t *AddReminderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Título breve del recordatorio.",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Mensaje o cuerpo detallado a notificar.",
			},
			"remind_at": map[string]interface{}{
				"type":        "string",
				"description": "Fecha, hora o patrón de tiempo (ej: 'en 5 minutos', 'mañana a las 8pm', 'cada 30 minutos', 'todos los días a las 9am').",
			},
			"recurrence": map[string]interface{}{
				"type":        "string",
				"description": "Regla de recurrencia alternativa (ej: 'diario', 'FREQ=DAILY;INTERVAL=1', 'FREQ=MINUTELY;INTERVAL=30').",
			},
			"channels": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Lista de canales para notificar (ej: ['desktop', 'voice', 'hud', 'sound']).",
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"description": "Prioridad: normal o urgent.",
				"enum":        []interface{}{"normal", "urgent"},
			},
		},
		"required": []string{"title", "remind_at"},
	}
}

func (t *AddReminderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	title, _ := args["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("el argumento 'title' es obligatorio")
	}
	message, _ := args["message"].(string)
	remindAtStr, _ := args["remind_at"].(string)
	if remindAtStr == "" {
		return nil, fmt.Errorf("el argumento 'remind_at' es obligatorio")
	}

	priority, _ := args["priority"].(string)
	if priority == "" {
		priority = "normal"
	}

	var channelsJSON string
	if channelsRaw, ok := args["channels"]; ok {
		if bytes, err := json.Marshal(channelsRaw); err == nil {
			channelsJSON = string(bytes)
		}
	}
	if channelsJSON == "" {
		var defaultChs []string
		if t.config != nil && len(t.config.Reminders.DefaultChannels) > 0 {
			defaultChs = t.config.Reminders.DefaultChannels
		} else {
			defaultChs = []string{"desktop", "voice", "hud"}
		}
		bytes, _ := json.Marshal(defaultChs)
		channelsJSON = string(bytes)
	}

	// Parsear el tiempo
	defTime := "09:00"
	if t.config != nil && t.config.Reminders.DefaultTime != "" {
		defTime = t.config.Reminders.DefaultTime
	}

	var parsedStartAt time.Time
	var recurrenceRule string

	if parsedTime, err := time.Parse(time.RFC3339, remindAtStr); err == nil {
		parsedStartAt = parsedTime
	} else {
		res, err := timeparser.Parse(remindAtStr, defTime)
		if err != nil {
			return nil, fmt.Errorf("no se pudo parsear remind_at: %v", err)
		}
		parsedStartAt = res.StartAt
		recurrenceRule = res.RecurrenceRule
	}

	// Si se pasó una regla de recurrencia explícita, la priorizamos o configuramos
	recurrenceParam, _ := args["recurrence"].(string)
	if recurrenceParam != "" {
		if recurrenceParam == "diario" || recurrenceParam == "daily" {
			recurrenceRule = "FREQ=DAILY;INTERVAL=1"
		} else if recurrenceParam == "semanal" || recurrenceParam == "weekly" {
			recurrenceRule = "FREQ=WEEKLY;INTERVAL=1"
		} else {
			recurrenceRule = recurrenceParam
		}
	}

	remUTCStr := parsedStartAt.UTC().Format(time.RFC3339)

	rem, err := t.repo.Create(ctx, title, message, remUTCStr, recurrenceRule, channelsJSON, priority, "voice")
	if err != nil {
		return nil, fmt.Errorf("error al guardar el recordatorio: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}
	remindAtFormatted := rem.RemindAt.In(loc).Format("2006-01-02 15:04:05")

	recurMsg := ""
	if rem.RecurrenceRule != "" {
		recurMsg = fmt.Sprintf(" (recurrencia: %s)", rem.RecurrenceRule)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Recordatorio %d programado para el %s%s: %q.", rem.ID, remindAtFormatted, recurMsg, rem.Title),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ListRemindersTool enumera los recordatorios
type ListRemindersTool struct {
	repo   *Repository
	config *config.Config
}

func NewListRemindersTool(repo *Repository, cfg *config.Config) *ListRemindersTool {
	return &ListRemindersTool{repo: repo, config: cfg}
}

func (t *ListRemindersTool) Name() string { return "reminders.list" }
func (t *ListRemindersTool) Description() string {
	return "Muestra los recordatorios activos programados en el sistema."
}
func (t *ListRemindersTool) Category() string  { return "productivity" }
func (t *ListRemindersTool) RiskLevel() string { return "low" }
func (t *ListRemindersTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Filtro de estado: scheduled, triggered, cancelled, expired, o all (por defecto scheduled).",
				"enum":        []interface{}{"scheduled", "triggered", "cancelled", "expired", "all"},
			},
		},
	}
}

func (t *ListRemindersTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	status, _ := args["status"].(string)

	var statuses []string
	if status == "all" {
		statuses = []string{}
	} else if status != "" {
		statuses = []string{status}
	} else {
		statuses = []string{"scheduled"}
	}

	list, err := t.repo.List(ctx, statuses)
	if err != nil {
		return nil, fmt.Errorf("error al listar recordatorios: %v", err)
	}

	if len(list) == 0 {
		return &executor.ToolResult{
			Success:    true,
			Text:       "No se encontraron recordatorios activos.",
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

	text := "Lista de recordatorios:\n"
	for _, rem := range list {
		recurMsg := ""
		if rem.RecurrenceRule != "" {
			recurMsg = fmt.Sprintf(" [Recurrente: %s]", rem.RecurrenceRule)
		}
		text += fmt.Sprintf("[%s] ID %d: %q para el %s (Canales: %s, Prioridad: %s)%s\n",
			rem.Status, rem.ID, rem.Title, rem.RemindAt.In(loc).Format("2006-01-02 15:04"), rem.ChannelsJSON, rem.Priority, recurMsg)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// CancelReminderTool cancela un recordatorio
type CancelReminderTool struct {
	repo *Repository
}

func NewCancelReminderTool(repo *Repository) *CancelReminderTool {
	return &CancelReminderTool{repo: repo}
}

func (t *CancelReminderTool) Name() string { return "reminders.cancel" }
func (t *CancelReminderTool) Description() string {
	return "Cancela un recordatorio programado y sus trabajos futuros en el scheduler usando su ID."
}
func (t *CancelReminderTool) Category() string  { return "productivity" }
func (t *CancelReminderTool) RiskLevel() string { return "low" }
func (t *CancelReminderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID del recordatorio a cancelar.",
			},
		},
		"required": []string{"id"},
	}
}

func (t *CancelReminderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	rem, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró el recordatorio con ID %d", id)
	}

	err = t.repo.UpdateStatus(ctx, id, "cancelled")
	if err != nil {
		return nil, fmt.Errorf("error al cancelar recordatorio: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Recordatorio %d: %q cancelado correctamente.", id, rem.Title),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// RescheduleReminderTool reprograma un recordatorio existente
type RescheduleReminderTool struct {
	repo   *Repository
	config *config.Config
}

func NewRescheduleReminderTool(repo *Repository, cfg *config.Config) *RescheduleReminderTool {
	return &RescheduleReminderTool{repo: repo, config: cfg}
}

func (t *RescheduleReminderTool) Name() string { return "reminders.reschedule" }
func (t *RescheduleReminderTool) Description() string {
	return "Cambia la fecha u hora programada para un recordatorio por su ID."
}
func (t *RescheduleReminderTool) Category() string  { return "productivity" }
func (t *RescheduleReminderTool) RiskLevel() string { return "low" }
func (t *RescheduleReminderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID del recordatorio a reprogramar.",
			},
			"remind_at": map[string]interface{}{
				"type":        "string",
				"description": "Nueva fecha y hora (ej: 'en 2 horas', o RFC3339).",
			},
		},
		"required": []string{"id", "remind_at"},
	}
}

func (t *RescheduleReminderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	remindAtStr, _ := args["remind_at"].(string)
	if remindAtStr == "" {
		return nil, fmt.Errorf("el argumento 'remind_at' es obligatorio")
	}

	rem, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró el recordatorio con ID %d", id)
	}

	defTime := "09:00"
	if t.config != nil && t.config.Reminders.DefaultTime != "" {
		defTime = t.config.Reminders.DefaultTime
	}

	var parsedStartAt time.Time
	if parsedTime, err := time.Parse(time.RFC3339, remindAtStr); err == nil {
		parsedStartAt = parsedTime
	} else {
		res, err := timeparser.Parse(remindAtStr, defTime)
		if err != nil {
			return nil, fmt.Errorf("no se pudo parsear remind_at: %v", err)
		}
		parsedStartAt = res.StartAt
	}

	remUTCStr := parsedStartAt.UTC().Format(time.RFC3339)

	err = t.repo.Reschedule(ctx, id, remUTCStr)
	if err != nil {
		return nil, fmt.Errorf("error al reprogramar recordatorio: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}
	remindAtFormatted := parsedStartAt.In(loc).Format("2006-01-02 15:04:05")

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Recordatorio %d: %q reprogramado para el %s.", id, rem.Title, remindAtFormatted),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
