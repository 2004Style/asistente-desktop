package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"rbot/internal/executor"
)

type RememberTool struct {
	DB *sql.DB
}

func NewRememberTool(db *sql.DB) *RememberTool {
	return &RememberTool{DB: db}
}

func (t *RememberTool) Name() string { return "memory.remember" }
func (t *RememberTool) Description() string {
	return "Guarda o actualiza un dato o preferencia sobre el usuario en la memoria persistente del sistema."
}
func (t *RememberTool) Category() string  { return "memory" }
func (t *RememberTool) RiskLevel() string { return "low" }
func (t *RememberTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Nombre o identificador corto de la clave de memoria a guardar (ej: nombre, color_preferido).",
			},
			"value": map[string]interface{}{
				"type":        "string",
				"description": "El valor o información a recordar para dicha clave.",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Categoría general de la información (ej: usuario, preferencias, gustos).",
			},
		},
		"required": []string{"key", "value", "category"},
	}
}

func (t *RememberTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	key, _ := args["key"].(string)
	val, _ := args["value"].(string)
	cat, _ := args["category"].(string)

	if key == "" || val == "" || cat == "" {
		return nil, fmt.Errorf("los argumentos 'key', 'value' y 'category' son obligatorios")
	}

	query := `
		INSERT INTO user_memory (key, value, category, source)
		VALUES (?, ?, ?, 'conversation')
		ON CONFLICT(key, category) DO UPDATE SET
			value = excluded.value,
			updated_at = datetime('now');
	`
	_, err := t.DB.ExecContext(ctx, query, strings.ToLower(key), val, strings.ToLower(cat))
	if err != nil {
		return nil, fmt.Errorf("error al guardar en memoria: %v", err)
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("He recordado que tu %s (%s) es: %s", key, cat, val),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
