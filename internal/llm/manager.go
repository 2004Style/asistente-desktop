package llm

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager coordina el proveedor activo, gestiona fallbacks, discovery/cache y persiste la configuración en SQLite.
type Manager struct {
	mu                       sync.RWMutex
	db                       *sql.DB
	registry                 *Registry
	active                   Provider
	activeID                 string // nombre del proveedor activo
	activeProfile            string
	OnActiveSelectionChanged func(providerName, modelID, profileName string)
}

// NewManager crea un manager con la base de datos y el registro dados.
func NewManager(db *sql.DB, registry *Registry) *Manager {
	return &Manager{
		db:       db,
		registry: registry,
	}
}

// Registry devuelve el registro de proveedores asociado.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// SetRegistry permite actualizar el registro de proveedores en caliente.
func (m *Manager) SetRegistry(registry *Registry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registry = registry
}

// Active devuelve el proveedor actualmente activo.
func (m *Manager) Active() Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// ActiveName devuelve el nombre del proveedor activo.
func (m *Manager) ActiveName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeID
}

// ActiveProfile devuelve el perfil activo conocido por el manager.
func (m *Manager) ActiveProfile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeProfile
}

// SetActive establece el proveedor activo por su nombre registrado.
func (m *Manager) SetActive(name string) error {
	p, ok := m.registry.Get(name)
	if !ok {
		return fmt.Errorf("proveedor '%s' no está registrado", name)
	}
	return m.SetActiveSelection(name, p.ModelID(), "")
}

// SetActiveSelection establece proveedor, modelo y perfil de forma atómica.
func (m *Manager) SetActiveSelection(providerName, modelID, profileName string) error {
	p, ok := m.registry.Get(providerName)
	if !ok {
		return fmt.Errorf("proveedor '%s' no está registrado", providerName)
	}
	if modelID == "" {
		modelID = p.ModelID()
	}
	p.SetModel(modelID)

	m.mu.Lock()
	m.active = p
	m.activeID = providerName
	m.activeProfile = profileName
	m.mu.Unlock()

	if m.db != nil {
		m.persistActiveSelection(providerName, modelID, profileName)
	}

	if profileName != "" {
		log.Printf("[LLM Manager] Selección activa establecida: profile=%s provider=%s model=%s", profileName, providerName, modelID)
	} else {
		log.Printf("[LLM Manager] Proveedor activo establecido: %s (modelo: %s)", providerName, modelID)
	}

	if m.OnActiveSelectionChanged != nil {
		m.OnActiveSelectionChanged(providerName, modelID, profileName)
	}
	return nil
}

// Chat delega la llamada al proveedor activo. Si falla, intenta fallback con otros proveedores.
func (m *Manager) Chat(ctx context.Context, messages []Message, tools []Tool, opts ChatOptions) (*Message, error) {
	active := m.Active()
	if active == nil {
		return nil, fmt.Errorf("no hay proveedor LLM activo configurado")
	}

	resp, err := active.Chat(ctx, messages, tools, opts)
	if err != nil {
		log.Printf("[LLM Manager] Error con proveedor '%s': %v. Intentando fallback...", m.ActiveName(), err)
		return m.chatWithFallback(ctx, messages, tools, opts, m.ActiveName())
	}

	return resp, nil
}

// chatWithFallback intenta usar otros proveedores disponibles si el activo falla.
func (m *Manager) chatWithFallback(ctx context.Context, messages []Message, tools []Tool, opts ChatOptions, excludeName string) (*Message, error) {
	for _, p := range m.registry.List() {
		if p.Name() == excludeName {
			continue
		}

		if err := p.Ping(ctx); err != nil {
			log.Printf("[LLM Manager] Proveedor fallback '%s' no disponible: %v", p.Name(), err)
			continue
		}

		log.Printf("[LLM Manager] Usando proveedor fallback: %s", p.Name())
		resp, err := p.Chat(ctx, messages, tools, opts)
		if err == nil {
			if resp.Content != "" {
				resp.Content = fmt.Sprintf("[Nota: El proveedor '%s' no respondió. Usando de respaldo '%s']\n%s", excludeName, p.Name(), resp.Content)
			}
			return resp, nil
		}
		log.Printf("[LLM Manager] Error con proveedor fallback '%s': %v", p.Name(), err)
	}

	return nil, fmt.Errorf("todos los proveedores LLM fallaron, ningún fallback disponible")
}

// ListModels delega al proveedor activo con cache de discovery.
func (m *Manager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	active := m.Active()
	if active == nil {
		return nil, fmt.Errorf("no hay proveedor LLM activo configurado")
	}
	return m.listModelsWithCache(ctx, active)
}

// ListModelsForProvider lista modelos de un proveedor específico sin cambiar el proveedor activo.
func (m *Manager) ListModelsForProvider(ctx context.Context, providerName string) ([]ModelInfo, error) {
	p, ok := m.registry.Get(providerName)
	if !ok {
		return nil, fmt.Errorf("proveedor '%s' no está registrado", providerName)
	}
	return m.listModelsWithCache(ctx, p)
}

// ListAllModels lista modelos de todos los proveedores registrados.
func (m *Manager) ListAllModels(ctx context.Context) (map[string][]ModelInfo, error) {
	result := make(map[string][]ModelInfo)
	for _, p := range m.registry.List() {
		models, err := m.listModelsWithCache(ctx, p)
		if err != nil {
			log.Printf("[LLM Manager] Error listando modelos de '%s': %v", p.Name(), err)
			continue
		}
		result[p.Name()] = models
	}
	return result, nil
}

// Ping verifica el proveedor activo.
func (m *Manager) Ping(ctx context.Context) error {
	active := m.Active()
	if active == nil {
		return fmt.Errorf("no hay proveedor LLM activo configurado")
	}
	return active.Ping(ctx)
}

// SwitchModel cambia el modelo del proveedor activo.
func (m *Manager) SwitchModel(modelID string) error {
	active := m.Active()
	if active == nil {
		return fmt.Errorf("no hay proveedor LLM activo configurado")
	}
	return m.SetActiveSelection(m.ActiveName(), modelID, m.ActiveProfile())
}

// SwitchProviderModel cambia el proveedor activo y su modelo en una sola operación explícita.
func (m *Manager) SwitchProviderModel(providerName, modelID string) error {
	if modelID == "" {
		return fmt.Errorf("modelID requerido")
	}
	return m.SetActiveSelection(providerName, modelID, m.ActiveProfile())
}

// LoadFromDB carga la configuración del proveedor activo desde la base de datos.
func (m *Manager) LoadFromDB() error {
	if m.db == nil {
		return nil
	}

	var providerName, modelID, profileName string
	err := m.db.QueryRow(`SELECT provider_name, model_id, COALESCE(active_profile, '') FROM llm_providers WHERE is_active = 1 LIMIT 1`).
		Scan(&providerName, &modelID, &profileName)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("[LLM Manager] No hay proveedor activo en la BD. Usando configuración por defecto.")
			return nil
		}
		return fmt.Errorf("error cargando proveedor activo desde la BD: %w", err)
	}

	p, ok := m.registry.Get(providerName)
	if !ok {
		log.Printf("[LLM Manager] Proveedor '%s' en BD no está registrado. Ignorando.", providerName)
		return nil
	}

	if modelID != "" {
		p.SetModel(modelID)
	}

	m.mu.Lock()
	m.active = p
	m.activeID = providerName
	m.activeProfile = profileName
	m.mu.Unlock()

	log.Printf("[LLM Manager] Proveedor cargado desde BD: %s (modelo: %s, profile: %s)", providerName, modelID, profileName)
	return nil
}

// persistActiveSelection guarda el proveedor activo en la tabla llm_providers.
func (m *Manager) persistActiveSelection(name, modelID, profileName string) {
	if m.db == nil {
		return
	}
	_, _ = m.db.Exec(`UPDATE llm_providers SET is_active = 0`)
	_, err := m.db.Exec(`
		INSERT INTO llm_providers (provider_name, provider_type, base_url, model_id, active_profile, is_active, updated_at)
		VALUES (?, ?, '', ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_name) DO UPDATE SET
			model_id = excluded.model_id,
			active_profile = excluded.active_profile,
			is_active = 1,
			updated_at = CURRENT_TIMESTAMP
	`, name, name, modelID, profileName)
	if err != nil {
		log.Printf("[LLM Manager] Error persistiendo selección activa: %v", err)
	}
}

// SaveProviderConfig guarda la configuración completa de un proveedor en la BD.
func (m *Manager) SaveProviderConfig(cfg ProviderConfig) error {
	if m.db == nil {
		return fmt.Errorf("base de datos no disponible")
	}

	_, err := m.db.Exec(`
		INSERT INTO llm_providers (provider_name, provider_type, base_url, api_key_hash, model_id, active_profile, is_active, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_name) DO UPDATE SET
			provider_type = excluded.provider_type,
			base_url = excluded.base_url,
			api_key_hash = excluded.api_key_hash,
			model_id = excluded.model_id,
			active_profile = excluded.active_profile,
			is_active = excluded.is_active,
			updated_at = CURRENT_TIMESTAMP
	`, cfg.Name, cfg.Type, cfg.BaseURL, obfuscateKey(cfg.APIKey), cfg.Model, cfg.ActiveProfile, boolToInt(cfg.IsActive))

	return err
}

// GetProviderConfigs retorna todas las configuraciones de proveedores almacenadas.
func (m *Manager) GetProviderConfigs() ([]ProviderConfig, error) {
	if m.db == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	rows, err := m.db.Query(`SELECT provider_name, provider_type, base_url, model_id, active_profile, is_active FROM llm_providers ORDER BY provider_name`)
	if err != nil {
		return nil, fmt.Errorf("error listando proveedores: %w", err)
	}
	defer rows.Close()

	var configs []ProviderConfig
	for rows.Next() {
		var cfg ProviderConfig
		var isActive int
		if err := rows.Scan(&cfg.Name, &cfg.Type, &cfg.BaseURL, &cfg.Model, &cfg.ActiveProfile, &isActive); err != nil {
			continue
		}
		cfg.IsActive = isActive == 1
		configs = append(configs, cfg)
	}

	return configs, nil
}

func (m *Manager) listModelsWithCache(ctx context.Context, p Provider) ([]ModelInfo, error) {
	providerName := p.Name()
	if cached, cachedAt, ok, err := m.loadCachedModels(providerName); err == nil && ok {
		if time.Since(cachedAt) < m.cacheTTL(providerName) {
			return cached, nil
		}
	}

	models, err := p.ListModels(ctx)
	if err != nil {
		if cached, _, ok, cacheErr := m.loadCachedModels(providerName); cacheErr == nil && ok {
			log.Printf("[LLM Manager] Discovery falló para %s, usando cache: %v", providerName, err)
			return cached, nil
		}
		return nil, err
	}
	if m.db != nil {
		_ = m.saveCachedModels(providerName, models)
	}
	return models, nil
}

func (m *Manager) cacheTTL(providerName string) time.Duration {
	switch providerName {
	case "ollama":
		return 60 * time.Second
	case "openai":
		return 60 * time.Minute
	default:
		return 10 * time.Minute
	}
}

func (m *Manager) loadCachedModels(providerName string) ([]ModelInfo, time.Time, bool, error) {
	if m.db == nil {
		return nil, time.Time{}, false, nil
	}
	rows, err := m.db.Query(`
		SELECT model_id, model_name, family, size, tool_calling, streaming, vision, conversation_state, cached_at
		FROM llm_models_cache
		WHERE provider_name = ?
		ORDER BY model_id
	`, providerName)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	defer rows.Close()

	var models []ModelInfo
	var latest time.Time
	for rows.Next() {
		var info ModelInfo
		var toolCalling, streaming, vision, conversationState int
		var cachedAtStr string
		if err := rows.Scan(&info.ID, &info.Name, &info.Family, &info.Size, &toolCalling, &streaming, &vision, &conversationState, &cachedAtStr); err != nil {
			continue
		}
		info.Provider = providerName
		info.Capabilities = ModelCapabilities{
			ToolCalling:       toolCalling == 1,
			Streaming:         streaming == 1,
			Vision:            vision == 1,
			ConversationState: conversationState == 1,
		}
		if ts, err := time.Parse(time.RFC3339, cachedAtStr); err == nil && ts.After(latest) {
			latest = ts
		}
		models = append(models, info)
	}
	if len(models) == 0 {
		return nil, time.Time{}, false, nil
	}
	if latest.IsZero() {
		latest = time.Now()
	}
	return models, latest, true, nil
}

func (m *Manager) saveCachedModels(providerName string, models []ModelInfo) error {
	if m.db == nil {
		return nil
	}
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM llm_models_cache WHERE provider_name = ?`, providerName); err != nil {
		return err
	}
	for _, model := range models {
		_, err := tx.Exec(`
			INSERT INTO llm_models_cache (
				provider_name, model_id, model_name, family, size,
				tool_calling, streaming, vision, conversation_state, cached_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, providerName, model.ID, model.Name, model.Family, model.Size, boolToInt(model.Capabilities.ToolCalling), boolToInt(model.Capabilities.Streaming), boolToInt(model.Capabilities.Vision), boolToInt(model.Capabilities.ConversationState))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func obfuscateKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
