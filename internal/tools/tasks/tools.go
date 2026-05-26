package tasks

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/timeparser"
)

// AddTaskTool crea una nueva tarea
type AddTaskTool struct {
	repo   *Repository
	config *config.Config
}

func NewAddTaskTool(repo *Repository, cfg *config.Config) *AddTaskTool {
	return &AddTaskTool{repo: repo, config: cfg}
}

func (t *AddTaskTool) Name() string { return "tasks.add" }
func (t *AddTaskTool) Description() string {
	return "Agrega una nueva tarea pendiente con prioridad y fecha de vencimiento opcional (soporta lenguaje natural en español)."
}
func (t *AddTaskTool) Category() string  { return "productivity" }
func (t *AddTaskTool) RiskLevel() string { return "low" }
func (t *AddTaskTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Título o descripción corta de la tarea.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Detalles adicionales de la tarea.",
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"description": "Prioridad de la tarea: low, normal, high, urgent.",
				"enum":        []interface{}{"low", "normal", "high", "urgent"},
			},
			"due_at": map[string]interface{}{
				"type":        "string",
				"description": "Fecha y hora de vencimiento (ej: 'mañana a las 5pm', 'el viernes a las 9:00', o en formato RFC3339).",
			},
		},
		"required": []string{"title"},
	}
}

func (t *AddTaskTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	title, _ := args["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("el argumento 'title' es obligatorio")
	}
	description, _ := args["description"].(string)
	priority, _ := args["priority"].(string)
	if priority == "" {
		if t.config != nil && t.config.Tasks.DefaultPriority != "" {
			priority = t.config.Tasks.DefaultPriority
		} else {
			priority = "normal"
		}
	}

	dueAtStr, _ := args["due_at"].(string)
	var dueUTCStr string
	if dueAtStr != "" {
		// Intentar parsear como RFC3339 primero
		if parsedTime, err := time.Parse(time.RFC3339, dueAtStr); err == nil {
			dueUTCStr = parsedTime.UTC().Format(time.RFC3339)
		} else {
			// Usar parser de tiempo en español
			defTime := "09:00"
			if t.config != nil && t.config.Reminders.DefaultTime != "" {
				defTime = t.config.Reminders.DefaultTime
			}
			res, err := timeparser.Parse(dueAtStr, defTime)
			if err != nil {
				return nil, fmt.Errorf("no se pudo parsear el vencimiento: %v", err)
			}
			dueUTCStr = res.StartAt.UTC().Format(time.RFC3339)
		}
	}

	task, err := t.repo.Create(ctx, title, description, priority, dueUTCStr, "voice")
	if err != nil {
		return nil, fmt.Errorf("error al guardar la tarea: %v", err)
	}

	dueMsg := "sin vencimiento"
	if task.DueAt != nil {
		loc := time.Local
		if t.config != nil && t.config.Time.Timezone != "" {
			if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
				loc = l
			}
		}
		dueMsg = fmt.Sprintf("vence el %s", task.DueAt.In(loc).Format("2006-01-02 15:04"))
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tarea %d agregada: %q (%s, prioridad: %s).", task.ID, task.Title, dueMsg, task.Priority),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ListTasksTool enumera las tareas
type ListTasksTool struct {
	repo   *Repository
	config *config.Config
}

func NewListTasksTool(repo *Repository, cfg *config.Config) *ListTasksTool {
	return &ListTasksTool{repo: repo, config: cfg}
}

func (t *ListTasksTool) Name() string { return "tasks.list" }
func (t *ListTasksTool) Description() string {
	return "Lista las tareas filtradas por su estado actual (por defecto muestra pending e in_progress)."
}
func (t *ListTasksTool) Category() string  { return "productivity" }
func (t *ListTasksTool) RiskLevel() string { return "low" }
func (t *ListTasksTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Estado de las tareas a listar: pending, in_progress, completed, cancelled, expired o all.",
				"enum":        []interface{}{"pending", "in_progress", "completed", "cancelled", "expired", "all"},
			},
		},
	}
}

func (t *ListTasksTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	status, _ := args["status"].(string)

	var statuses []string
	if status == "all" {
		statuses = []string{}
	} else if status != "" {
		statuses = []string{status}
	} else {
		statuses = []string{"pending", "in_progress"}
	}

	list, err := t.repo.List(ctx, statuses)
	if err != nil {
		return nil, fmt.Errorf("error al listar tareas: %v", err)
	}

	if len(list) == 0 {
		return &executor.ToolResult{
			Success:    true,
			Text:       "No se encontraron tareas con los criterios especificados.",
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

	text := "Lista de tareas:\n"
	for _, task := range list {
		dueMsg := "sin vencimiento"
		if task.DueAt != nil {
			dueMsg = fmt.Sprintf("vence: %s", task.DueAt.In(loc).Format("2006-01-02 15:04"))
		}
		descMsg := ""
		if task.Description != "" {
			descMsg = fmt.Sprintf(" - %s", task.Description)
		}
		text += fmt.Sprintf("[%s] ID %d: %q (Prioridad: %s, %s)%s\n", task.Status, task.ID, task.Title, task.Priority, dueMsg, descMsg)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       text,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// CompleteTaskTool marca una tarea como completada
type CompleteTaskTool struct {
	repo *Repository
}

func NewCompleteTaskTool(repo *Repository) *CompleteTaskTool {
	return &CompleteTaskTool{repo: repo}
}

func (t *CompleteTaskTool) Name() string { return "tasks.complete" }
func (t *CompleteTaskTool) Description() string {
	return "Marca una tarea específica como completada por su ID."
}
func (t *CompleteTaskTool) Category() string  { return "productivity" }
func (t *CompleteTaskTool) RiskLevel() string { return "low" }
func (t *CompleteTaskTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID numérico de la tarea.",
			},
		},
		"required": []string{"id"},
	}
}

func (t *CompleteTaskTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	task, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró la tarea con ID %d", id)
	}

	err = t.repo.UpdateStatus(ctx, id, "completed")
	if err != nil {
		return nil, fmt.Errorf("error al actualizar la tarea: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tarea %d: %q marcada como completada.", id, task.Title),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// DeleteTaskTool elimina una tarea de la base de datos
type DeleteTaskTool struct {
	repo *Repository
}

func NewDeleteTaskTool(repo *Repository) *DeleteTaskTool {
	return &DeleteTaskTool{repo: repo}
}

func (t *DeleteTaskTool) Name() string { return "tasks.delete" }
func (t *DeleteTaskTool) Description() string {
	return "Elimina físicamente una tarea por su ID."
}
func (t *DeleteTaskTool) Category() string  { return "productivity" }
func (t *DeleteTaskTool) RiskLevel() string { return "low" }
func (t *DeleteTaskTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID numérico de la tarea a eliminar.",
			},
		},
		"required": []string{"id"},
	}
}

func (t *DeleteTaskTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	task, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró la tarea con ID %d", id)
	}

	err = t.repo.Delete(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error al eliminar la tarea: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tarea %d: %q eliminada de la base de datos.", id, task.Title),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// RescheduleTaskTool cambia la fecha de vencimiento de una tarea
type RescheduleTaskTool struct {
	repo   *Repository
	config *config.Config
}

func NewRescheduleTaskTool(repo *Repository, cfg *config.Config) *RescheduleTaskTool {
	return &RescheduleTaskTool{repo: repo, config: cfg}
}

func (t *RescheduleTaskTool) Name() string { return "tasks.reschedule" }
func (t *RescheduleTaskTool) Description() string {
	return "Reprograma la fecha de vencimiento de una tarea por su ID."
}
func (t *RescheduleTaskTool) Category() string  { return "productivity" }
func (t *RescheduleTaskTool) RiskLevel() string { return "low" }
func (t *RescheduleTaskTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID numérico de la tarea.",
			},
			"due_at": map[string]interface{}{
				"type":        "string",
				"description": "Nueva fecha y hora de vencimiento (ej: 'mañana a las 10am' o RFC3339).",
			},
		},
		"required": []string{"id", "due_at"},
	}
}

func (t *RescheduleTaskTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	dueAtStr, _ := args["due_at"].(string)
	if dueAtStr == "" {
		return nil, fmt.Errorf("el argumento 'due_at' es obligatorio")
	}

	task, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró la tarea con ID %d", id)
	}

	var dueUTCStr string
	if parsedTime, err := time.Parse(time.RFC3339, dueAtStr); err == nil {
		dueUTCStr = parsedTime.UTC().Format(time.RFC3339)
	} else {
		defTime := "09:00"
		if t.config != nil && t.config.Reminders.DefaultTime != "" {
			defTime = t.config.Reminders.DefaultTime
		}
		res, err := timeparser.Parse(dueAtStr, defTime)
		if err != nil {
			return nil, fmt.Errorf("no se pudo parsear el vencimiento: %v", err)
		}
		dueUTCStr = res.StartAt.UTC().Format(time.RFC3339)
	}

	err = t.repo.Reschedule(ctx, id, dueUTCStr)
	if err != nil {
		return nil, fmt.Errorf("error al reprogramar la tarea: %v", err)
	}

	loc := time.Local
	if t.config != nil && t.config.Time.Timezone != "" {
		if l, err := time.LoadLocation(t.config.Time.Timezone); err == nil {
			loc = l
		}
	}
	parsedUTC, _ := time.Parse(time.RFC3339, dueUTCStr)
	newDueFormatted := parsedUTC.In(loc).Format("2006-01-02 15:04")

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tarea %d: %q reprogramada para el %s.", id, task.Title, newDueFormatted),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// UpdateTaskPriorityTool cambia la prioridad de una tarea
type UpdateTaskPriorityTool struct {
	repo *Repository
}

func NewUpdateTaskPriorityTool(repo *Repository) *UpdateTaskPriorityTool {
	return &UpdateTaskPriorityTool{repo: repo}
}

func (t *UpdateTaskPriorityTool) Name() string { return "tasks.update_priority" }
func (t *UpdateTaskPriorityTool) Description() string {
	return "Actualiza la prioridad de una tarea por su ID."
}
func (t *UpdateTaskPriorityTool) Category() string  { return "productivity" }
func (t *UpdateTaskPriorityTool) RiskLevel() string { return "low" }
func (t *UpdateTaskPriorityTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "ID numérico de la tarea.",
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"description": "Nueva prioridad: low, normal, high, urgent.",
				"enum":        []interface{}{"low", "normal", "high", "urgent"},
			},
		},
		"required": []string{"id", "priority"},
	}
}

func (t *UpdateTaskPriorityTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
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

	priority, _ := args["priority"].(string)
	if priority == "" {
		return nil, fmt.Errorf("el argumento 'priority' es obligatorio")
	}

	task, err := t.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("no se encontró la tarea con ID %d", id)
	}

	err = t.repo.UpdatePriority(ctx, id, priority)
	if err != nil {
		return nil, fmt.Errorf("error al actualizar prioridad de la tarea: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Tarea %d: %q actualizada con prioridad %s.", id, task.Title, priority),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
