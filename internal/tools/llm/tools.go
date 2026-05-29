package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/llm"
)

// ListProvidersTool enumera los proveedores registrados
type ListProvidersTool struct {
	manager *llm.Manager
}

func NewListProvidersTool(m *llm.Manager) *ListProvidersTool {
	return &ListProvidersTool{manager: m}
}

func (t *ListProvidersTool) Name() string { return "llm.list_providers" }
func (t *ListProvidersTool) Description() string {
	return "Muestra una lista de todos los proveedores de LLM registrados y su estado actual."
}
func (t *ListProvidersTool) Category() string  { return "llm" }
func (t *ListProvidersTool) RiskLevel() string { return "low" }
func (t *ListProvidersTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *ListProvidersTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	var lines []string
	lines = append(lines, "Proveedores de LLM registrados:")
	for _, p := range t.manager.Registry().List() {
		isActive := t.manager.ActiveName() == p.Name()
		activeIndicator := ""
		if isActive {
			activeIndicator = " [ACTIVO]"
		}
		lines = append(lines, fmt.Sprintf("- %s: modelo activo = %s%s", p.Name(), p.ModelID(), activeIndicator))
	}
	return &executor.ToolResult{
		Success:    true,
		Text:       strings.Join(lines, "\n"),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// GetStatusTool informa sobre el estado de la conexión activa
type GetStatusTool struct {
	manager *llm.Manager
}

func NewGetStatusTool(m *llm.Manager) *GetStatusTool {
	return &GetStatusTool{manager: m}
}

func (t *GetStatusTool) Name() string { return "llm.get_status" }
func (t *GetStatusTool) Description() string {
	return "Muestra el proveedor, modelo, perfil y modo de autenticación activos."
}
func (t *GetStatusTool) Category() string  { return "llm" }
func (t *GetStatusTool) RiskLevel() string { return "low" }
func (t *GetStatusTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *GetStatusTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	p := t.manager.Active()
	if p == nil {
		return nil, fmt.Errorf("no hay proveedor LLM activo configurado")
	}

	pingErr := p.Ping(ctx)
	status := "disponible"
	if pingErr != nil {
		status = fmt.Sprintf("error: %v", pingErr)
	}

	profileName := t.manager.ActiveProfile()
	if profileName == "" {
		profileName = "ninguno (personalizado)"
	}

	textMsg := fmt.Sprintf("Estado del LLM:\n- Proveedor: %s\n- Modelo activo: %s\n- Perfil: %s\n- Conectividad: %s",
		p.Name(), p.ModelID(), profileName, status)

	return &executor.ToolResult{
		Success:    true,
		Text:       textMsg,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// UseProviderTool cambia el proveedor activo
type UseProviderTool struct {
	manager *llm.Manager
}

func NewUseProviderTool(m *llm.Manager) *UseProviderTool {
	return &UseProviderTool{manager: m}
}

func (t *UseProviderTool) Name() string { return "llm.use_provider" }
func (t *UseProviderTool) Description() string {
	return "Cambia el proveedor LLM activo."
}
func (t *UseProviderTool) Category() string  { return "llm" }
func (t *UseProviderTool) RiskLevel() string { return "low" }
func (t *UseProviderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"provider": map[string]interface{}{"type": "string", "description": "Nombre del proveedor (ollama, openai, compatible)"},
			"model":    map[string]interface{}{"type": "string", "description": "Modelo a usar (opcional)"},
		},
		"required": []string{"provider"},
	}
}

func (t *UseProviderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	provider, _ := args["provider"].(string)
	model, _ := args["model"].(string)

	p, ok := t.manager.Registry().Get(provider)
	if !ok {
		return nil, fmt.Errorf("proveedor '%s' no está registrado", provider)
	}
	if model == "" {
		model = p.ModelID()
	}

	var warningMsg string
	if provider == "ollama" {
		models, err := t.manager.ListModelsForProvider(ctx, provider)
		if err == nil && len(models) > 0 {
			found := false
			var modelIDs []string
			for _, m := range models {
				modelIDs = append(modelIDs, m.ID)
				if m.ID == model || strings.HasPrefix(m.ID, model+":") || strings.TrimSuffix(m.ID, ":latest") == model {
					model = m.ID
					found = true
					break
				}
			}
			if !found {
				warningMsg = fmt.Sprintf("\nAdvertencia: El modelo '%s' no se encuentra entre los instalados en Ollama. Modelos disponibles: %s", model, strings.Join(modelIDs, ", "))
			}
		}
	}

	if err := t.manager.SetActiveSelection(provider, model, ""); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Proveedor LLM cambiado a '%s' con modelo '%s'.%s", provider, model, warningMsg),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// SwitchModelTool cambia el modelo del proveedor actual
type SwitchModelTool struct {
	manager *llm.Manager
}

func NewSwitchModelTool(m *llm.Manager) *SwitchModelTool {
	return &SwitchModelTool{manager: m}
}

func (t *SwitchModelTool) Name() string { return "llm.switch_model" }
func (t *SwitchModelTool) Description() string {
	return "Cambia el modelo activo del proveedor actual."
}
func (t *SwitchModelTool) Category() string  { return "llm" }
func (t *SwitchModelTool) RiskLevel() string { return "low" }
func (t *SwitchModelTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"model":    map[string]interface{}{"type": "string", "description": "Identificador del modelo"},
			"provider": map[string]interface{}{"type": "string", "description": "Nombre del proveedor (opcional, por defecto el activo)"},
		},
		"required": []string{"model"},
	}
}

func (t *SwitchModelTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	model, _ := args["model"].(string)
	provider, _ := args["provider"].(string)

	if provider == "" {
		provider = t.manager.ActiveName()
	}

	_, ok := t.manager.Registry().Get(provider)
	if !ok {
		return nil, fmt.Errorf("proveedor '%s' no está registrado", provider)
	}

	var warningMsg string
	if provider == "ollama" {
		models, err := t.manager.ListModelsForProvider(ctx, provider)
		if err == nil && len(models) > 0 {
			found := false
			var modelIDs []string
			for _, m := range models {
				modelIDs = append(modelIDs, m.ID)
				if m.ID == model || strings.HasPrefix(m.ID, model+":") || strings.TrimSuffix(m.ID, ":latest") == model {
					model = m.ID
					found = true
					break
				}
			}
			if !found {
				warningMsg = fmt.Sprintf("\nAdvertencia: El modelo '%s' no se encuentra entre los instalados en Ollama. Modelos disponibles: %s", model, strings.Join(modelIDs, ", "))
			}
		}
	}

	if err := t.manager.SetActiveSelection(provider, model, t.manager.ActiveProfile()); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Modelo LLM cambiado a '%s' en el proveedor '%s'.%s", model, provider, warningMsg),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ListModelsTool enumera modelos disponibles de un proveedor
type ListModelsTool struct {
	manager *llm.Manager
}

func NewListModelsTool(m *llm.Manager) *ListModelsTool {
	return &ListModelsTool{manager: m}
}

func (t *ListModelsTool) Name() string { return "llm.list_models" }
func (t *ListModelsTool) Description() string {
	return "Muestra una lista de los modelos de IA disponibles para el proveedor especificado (o el activo)."
}
func (t *ListModelsTool) Category() string  { return "llm" }
func (t *ListModelsTool) RiskLevel() string { return "low" }
func (t *ListModelsTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"provider": map[string]interface{}{"type": "string", "description": "Nombre del proveedor (opcional)"},
		},
	}
}

func (t *ListModelsTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	provider, _ := args["provider"].(string)
	if provider == "" {
		provider = t.manager.ActiveName()
	}

	models, err := t.manager.ListModelsForProvider(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("error listando modelos para %s: %w", provider, err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Modelos disponibles en %s:", provider))
	for i, m := range models {
		sizeStr := ""
		if m.Size != "" {
			sizeStr = fmt.Sprintf(" (%s)", m.Size)
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s", i+1, m.ID, sizeStr))
	}
	lines = append(lines, "", fmt.Sprintf("Modelo activo actual: %s", t.manager.Active().ModelID()))

	return &executor.ToolResult{
		Success:    true,
		Text:       strings.Join(lines, "\n"),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// ListProfilesTool lista los perfiles de ejecución configurados
type ListProfilesTool struct {
	manager       *llm.Manager
	providersConf *config.ProvidersConfig
}

func NewListProfilesTool(m *llm.Manager, conf *config.ProvidersConfig) *ListProfilesTool {
	return &ListProfilesTool{manager: m, providersConf: conf}
}

func (t *ListProfilesTool) Name() string { return "llm.list_profiles" }
func (t *ListProfilesTool) Description() string {
	return "Muestra una lista de todos los perfiles de ejecución configurados."
}
func (t *ListProfilesTool) Category() string  { return "llm" }
func (t *ListProfilesTool) RiskLevel() string { return "low" }
func (t *ListProfilesTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func (t *ListProfilesTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	if t.providersConf == nil {
		return nil, fmt.Errorf("configuración de perfiles no disponible")
	}

	var lines []string
	lines = append(lines, "Perfiles de ejecución:")
	for name, p := range t.providersConf.Profiles {
		activeStr := ""
		if t.manager.ActiveProfile() == name {
			activeStr = " [ACTIVO]"
		}
		lines = append(lines, fmt.Sprintf("- %s: provider=%s, model=%s, auth=%s%s", name, p.Provider, p.Model, p.AuthMode, activeStr))
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       strings.Join(lines, "\n"),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// UseProfileTool cambia el perfil de ejecución activo
type UseProfileTool struct {
	manager       *llm.Manager
	providersConf *config.ProvidersConfig
}

func NewUseProfileTool(m *llm.Manager, conf *config.ProvidersConfig) *UseProfileTool {
	return &UseProfileTool{manager: m, providersConf: conf}
}

func (t *UseProfileTool) Name() string { return "llm.use_profile" }
func (t *UseProfileTool) Description() string {
	return "Cambia al perfil de ejecución de LLM especificado."
}
func (t *UseProfileTool) Category() string  { return "llm" }
func (t *UseProfileTool) RiskLevel() string { return "low" }
func (t *UseProfileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "description": "Nombre del perfil"},
		},
		"required": []string{"name"},
	}
}

func (t *UseProfileTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	name, _ := args["name"].(string)

	if t.providersConf == nil {
		return nil, fmt.Errorf("configuración de perfiles no disponible")
	}

	profile, ok := t.providersConf.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("perfil '%s' no existe o no está configurado", name)
	}

	modelID := profile.Model
	if modelID == "" || modelID == "auto" {
		if p, ok := t.manager.Registry().Get(profile.Provider); ok {
			modelID = p.ModelID()
		}
	}

	if err := t.manager.SetActiveSelection(profile.Provider, modelID, name); err != nil {
		return nil, err
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Perfil de ejecución cambiado a '%s'. Provider: %s, Modelo: %s", name, profile.Provider, modelID),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// CreateProfileTool crea un nuevo perfil de ejecución
type CreateProfileTool struct {
	manager       *llm.Manager
	providersConf *config.ProvidersConfig
	configPath    string
}

func NewCreateProfileTool(m *llm.Manager, conf *config.ProvidersConfig, path string) *CreateProfileTool {
	return &CreateProfileTool{manager: m, providersConf: conf, configPath: path}
}

func (t *CreateProfileTool) Name() string { return "llm.create_profile" }
func (t *CreateProfileTool) Description() string {
	return "Crea un nuevo perfil de ejecución de LLM en providers.yaml."
}
func (t *CreateProfileTool) Category() string  { return "llm" }
func (t *CreateProfileTool) RiskLevel() string { return "low" }
func (t *CreateProfileTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "description": "Nombre único del perfil"},
			"provider":    map[string]interface{}{"type": "string", "description": "Nombre del proveedor (ollama, openai, compatible)"},
			"model":       map[string]interface{}{"type": "string", "description": "Nombre o ID del modelo"},
			"auth_mode":   map[string]interface{}{"type": "string", "description": "Modo de autenticación (none, api_key, browser_login)"},
			"description": map[string]interface{}{"type": "string", "description": "Descripción del perfil (opcional)"},
		},
		"required": []string{"name", "provider", "model"},
	}
}

func (t *CreateProfileTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	name, _ := args["name"].(string)
	provider, _ := args["provider"].(string)
	model, _ := args["model"].(string)
	authMode, _ := args["auth_mode"].(string)
	description, _ := args["description"].(string)

	if authMode == "" {
		authMode = "none"
		if provider == "openai" {
			authMode = "api_key"
		}
	}

	if t.providersConf == nil {
		return nil, fmt.Errorf("configuración de perfiles no disponible")
	}

	profileEntry := config.ProfileEntry{
		Provider:    provider,
		Model:       model,
		AuthMode:    authMode,
		Description: description,
		Enabled:     true,
	}

	t.providersConf.Profiles[name] = profileEntry

	if t.configPath != "" {
		if err := config.SaveProvidersConfig(t.configPath, t.providersConf); err != nil {
			return nil, fmt.Errorf("error guardando el nuevo perfil en yaml: %w", err)
		}
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("Perfil '%s' creado y guardado con éxito.", name),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}

// VerifyProviderTool valida conectividad de un proveedor
type VerifyProviderTool struct {
	manager *llm.Manager
}

func NewVerifyProviderTool(m *llm.Manager) *VerifyProviderTool {
	return &VerifyProviderTool{manager: m}
}

func (t *VerifyProviderTool) Name() string { return "llm.verify_provider" }
func (t *VerifyProviderTool) Description() string {
	return "Verifica si el proveedor LLM especificado está disponible y responde correctamente."
}
func (t *VerifyProviderTool) Category() string  { return "llm" }
func (t *VerifyProviderTool) RiskLevel() string { return "low" }
func (t *VerifyProviderTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"provider": map[string]interface{}{"type": "string", "description": "Nombre del proveedor"},
		},
	}
}

func (t *VerifyProviderTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()
	provider, _ := args["provider"].(string)
	if provider == "" {
		provider = t.manager.ActiveName()
	}

	p, ok := t.manager.Registry().Get(provider)
	if !ok {
		return nil, fmt.Errorf("proveedor '%s' no está registrado", provider)
	}

	pingErr := p.Ping(ctx)
	if pingErr != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      pingErr.Error(),
			Text:       fmt.Sprintf("El proveedor '%s' NO está disponible: %v", provider, pingErr),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       fmt.Sprintf("El proveedor '%s' está disponible y respondiendo correctamente.", provider),
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
