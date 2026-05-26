package workspace

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/skills"
	"rbot/internal/workspace"
)

type GenericTool struct {
	name        string
	description string
	category    string
	riskLevel   string
	schema      map[string]interface{}
	execute     func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error)
}

func (t *GenericTool) Name() string                   { return t.name }
func (t *GenericTool) Description() string            { return t.description }
func (t *GenericTool) Category() string               { return t.category }
func (t *GenericTool) RiskLevel() string              { return t.riskLevel }
func (t *GenericTool) Schema() map[string]interface{} { return t.schema }
func (t *GenericTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	return t.execute(ctx, args)
}

type WorkspaceController struct {
	db          *sql.DB
	cfg         *config.Config
	loader      *workspace.Loader
	installer   *skills.Installer
	getWS       func() *workspace.WorkspaceContext
	reloadWS    func() (*workspace.WorkspaceContext, error)
	runPlan     func(ctx context.Context, p interface{}) (*executor.ToolResult, error) // Para ejecutar shortcuts
}

func NewWorkspaceController(
	db *sql.DB,
	cfg *config.Config,
	loader *workspace.Loader,
	installer *skills.Installer,
	getWS func() *workspace.WorkspaceContext,
	reloadWS func() (*workspace.WorkspaceContext, error),
	runPlan func(ctx context.Context, p interface{}) (*executor.ToolResult, error),
) *WorkspaceController {
	return &WorkspaceController{
		db:        db,
		cfg:       cfg,
		loader:    loader,
		installer: installer,
		getWS:     getWS,
		reloadWS:  reloadWS,
		runPlan:   runPlan,
	}
}

func RegisterTools(reg *executor.Registry, ctrl *WorkspaceController) error {
	tools := []*GenericTool{
		{
			name:        "workspace.status",
			description: "Obtiene el estado actual del workspace de usuario.",
			category:    "system",
			riskLevel:   "low",
			schema:      map[string]interface{}{},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				ws := ctrl.getWS()
				if ws == nil {
					return &executor.ToolResult{
						Success:    false,
						Error:      "Workspace no inicializado",
						StartedAt:  time.Now(),
						FinishedAt: time.Now(),
					}, nil
				}
				data := map[string]interface{}{
					"path":      ctrl.cfg.Workspace.Path,
					"loaded_at": ws.LoadedAt.Format(time.RFC3339),
					"shortcuts": len(ws.Shortcuts),
				}
				return &executor.ToolResult{
					Success:    true,
					Text:       fmt.Sprintf("Workspace cargado en %s. Atajos activos: %d.", ctrl.cfg.Workspace.Path, len(ws.Shortcuts)),
					Data:       data,
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
				}, nil
			},
		},
		{
			name:        "workspace.reload",
			description: "Recarga todos los archivos Markdown y YAML del workspace de usuario.",
			category:    "system",
			riskLevel:   "low",
			schema:      map[string]interface{}{},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				ws, err := ctrl.reloadWS()
				if err != nil {
					return &executor.ToolResult{
						Success:    false,
						Error:      err.Error(),
						StartedAt:  time.Now(),
						FinishedAt: time.Now(),
					}, nil
				}
				return &executor.ToolResult{
					Success:    true,
					Text:       "Workspace recargado con éxito. Atajos detectados: " + fmt.Sprintf("%d", len(ws.Shortcuts)),
					StartedAt:  time.Now(),
					FinishedAt: time.Now(),
				}, nil
			},
		},
		{
			name:        "workspace.validate",
			description: "Valida las políticas y shortcuts en el workspace.",
			category:    "system",
			riskLevel:   "low",
			schema:      map[string]interface{}{},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				ws := ctrl.getWS()
				if ws == nil {
					return &executor.ToolResult{Success: false, Error: "Workspace no cargado"}, nil
				}
				val := workspace.NewValidator()
				if err := val.ValidatePolicies(ws.Policies); err != nil {
					return &executor.ToolResult{Success: false, Error: "Políticas inválidas: " + err.Error()}, nil
				}
				if err := val.ValidateShortcuts(ws.Shortcuts); err != nil {
					return &executor.ToolResult{Success: false, Error: "Shortcuts inválidos: " + err.Error()}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    "Políticas y shortcuts del workspace validados correctamente.",
				}, nil
			},
		},
		{
			name:        "skills.list",
			description: "Lista todas las habilidades registradas en la base de datos.",
			category:    "system",
			riskLevel:   "low",
			schema:      map[string]interface{}{},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				rows, err := ctrl.db.Query("SELECT name, description, risk_level, status FROM skills")
				if err != nil {
					return &executor.ToolResult{Success: false, Error: err.Error()}, nil
				}
				defer rows.Close()

				var list []map[string]interface{}
				var textLines []string
				for rows.Next() {
					var name, desc, risk, status string
					if err := rows.Scan(&name, &desc, &risk, &status); err == nil {
						list = append(list, map[string]interface{}{
							"name":       name,
							"desc":       desc,
							"risk_level": risk,
							"status":     status,
						})
						textLines = append(textLines, fmt.Sprintf("- %s (%s) [%s]: %s", name, risk, status, desc))
					}
				}
				return &executor.ToolResult{
					Success: true,
					Text:    "Habilidades:\n" + strings.Join(textLines, "\n"),
					Data:    map[string]interface{}{"skills": list},
				}, nil
			},
		},
		{
			name:        "skills.info",
			description: "Muestra información de una habilidad por su nombre.",
			category:    "system",
			riskLevel:   "low",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "Nombre de la habilidad"},
				},
				"required": []interface{}{"name"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				name, _ := args["name"].(string)
				if name == "" {
					return &executor.ToolResult{Success: false, Error: "El parámetro 'name' es requerido"}, nil
				}
				var desc, version, risk, status, validationErrors string
				err := ctrl.db.QueryRow("SELECT description, version, risk_level, status, validation_errors FROM skills WHERE name = ?", name).Scan(&desc, &version, &risk, &status, &validationErrors)
				if err != nil {
					return &executor.ToolResult{Success: false, Error: "Habilidad no encontrada: " + err.Error()}, nil
				}
				text := fmt.Sprintf("Habilidad: %s\nVersión: %s\nRiesgo: %s\nEstado: %s\nDescripción: %s", name, version, risk, status, desc)
				if validationErrors != "" {
					text += "\nErrores de validación: " + validationErrors
				}
				return &executor.ToolResult{
					Success: true,
					Text:    text,
					Data: map[string]interface{}{
						"name":              name,
						"description":       desc,
						"version":           version,
						"risk_level":         risk,
						"status":             status,
						"validation_errors": validationErrors,
					},
				}, nil
			},
		},
		{
			name:        "skills.install",
			description: "Instala una habilidad desde un archivo ZIP local en staging.",
			category:    "system",
			riskLevel:   "high",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Ruta absoluta al archivo ZIP"},
				},
				"required": []interface{}{"path"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				zipPath, _ := args["path"].(string)
				if zipPath == "" {
					return &executor.ToolResult{Success: false, Error: "La ruta 'path' es requerida"}, nil
				}
				meta, err := ctrl.installer.InstallZip(zipPath)
				if err != nil {
					return &executor.ToolResult{Success: false, Error: "Error de instalación: " + err.Error()}, nil
				}

				// Forzar el escaneo del directorio para registrarla en base de datos
				_ = skills.ScanSkills(ctrl.db, ctrl.cfg.Skills.Path)

				return &executor.ToolResult{
					Success: true,
					Text:    fmt.Sprintf("Habilidad '%s' instalada correctamente en estado 'disabled'. Habilítala manualmente.", meta.Name),
					Data:    map[string]interface{}{"name": meta.Name, "status": "disabled"},
				}, nil
			},
		},
		{
			name:        "skills.enable",
			description: "Habilita una habilidad previamente instalada.",
			category:    "system",
			riskLevel:   "medium",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "Nombre de la habilidad"},
				},
				"required": []interface{}{"name"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				name, _ := args["name"].(string)
				if name == "" {
					return &executor.ToolResult{Success: false, Error: "El nombre es requerido"}, nil
				}
				if err := skills.EnableSkill(ctrl.db, name); err != nil {
					return &executor.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    fmt.Sprintf("Habilidad '%s' habilitada correctamente.", name),
				}, nil
			},
		},
		{
			name:        "skills.disable",
			description: "Deshabilita una habilidad activa.",
			category:    "system",
			riskLevel:   "medium",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "Nombre de la habilidad"},
				},
				"required": []interface{}{"name"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				name, _ := args["name"].(string)
				if name == "" {
					return &executor.ToolResult{Success: false, Error: "El nombre es requerido"}, nil
				}
				if err := skills.DisableSkill(ctrl.db, name); err != nil {
					return &executor.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    fmt.Sprintf("Habilidad '%s' desactivada correctamente.", name),
				}, nil
			},
		},
		{
			name:        "skills.trust",
			description: "Marca una habilidad como confiable (trusted) para reducir confirmaciones.",
			category:    "system",
			riskLevel:   "high",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "Nombre de la habilidad"},
				},
				"required": []interface{}{"name"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				name, _ := args["name"].(string)
				if name == "" {
					return &executor.ToolResult{Success: false, Error: "El nombre es requerido"}, nil
				}
				if err := skills.TrustSkill(ctrl.db, name); err != nil {
					return &executor.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    fmt.Sprintf("Habilidad '%s' marcada como confiable (trusted).", name),
				}, nil
			},
		},
		{
			name:        "skills.quarantine",
			description: "Coloca una habilidad bajo cuarentena preventiva.",
			category:    "system",
			riskLevel:   "medium",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string", "description": "Nombre de la habilidad"},
				},
				"required": []interface{}{"name"},
			},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				name, _ := args["name"].(string)
				if name == "" {
					return &executor.ToolResult{Success: false, Error: "El nombre es requerido"}, nil
				}
				if err := skills.QuarantineSkill(ctrl.db, name); err != nil {
					return &executor.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    fmt.Sprintf("Habilidad '%s' colocada en cuarentena.", name),
				}, nil
			},
		},
		{
			name:        "shortcuts.list",
			description: "Lista todas las macros/shortcuts definidas en el workspace.",
			category:    "system",
			riskLevel:   "low",
			schema:      map[string]interface{}{},
			execute: func(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
				ws := ctrl.getWS()
				if ws == nil {
					return &executor.ToolResult{Success: false, Error: "Workspace no cargado"}, nil
				}
				var lines []string
				for _, s := range ws.Shortcuts {
					lines = append(lines, fmt.Sprintf("- '%s': %s (Pasos: %d)", s.Name, s.Description, len(s.Steps)))
				}
				if len(lines) == 0 {
					return &executor.ToolResult{Success: true, Text: "No hay shortcuts registrados en el workspace."}, nil
				}
				return &executor.ToolResult{
					Success: true,
					Text:    "Shortcuts:\n" + strings.Join(lines, "\n"),
				}, nil
			},
		},
	}

	for _, tool := range tools {
		if err := reg.Register(tool); err != nil {
			return err
		}
	}

	return nil
}


