package llm

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
)

// Manager coordina el proveedor activo, gestiona fallbacks y persiste la configuración en SQLite.
type Manager struct {
	mu       sync.RWMutex
	db       *sql.DB
	registry *Registry
	active   Provider
	activeID string // nombre del proveedor activo
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

// SetActive establece el proveedor activo por su nombre registrado.
func (m *Manager) SetActive(name string) error {
	p, ok := m.registry.Get(name)
	if !ok {
		return fmt.Errorf("proveedor '%s' no está registrado", name)
	}

	m.mu.Lock()
	m.active = p
	m.activeID = name
	m.mu.Unlock()

	// Persistir en la base de datos
	if m.db != nil {
		m.persistActiveProvider(name, p.ModelID())
	}

	log.Printf("[LLM Manager] Proveedor activo establecido: %s (modelo: %s)", name, p.ModelID())
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

		// Verificar si el proveedor está disponible
		if err := p.Ping(ctx); err != nil {
			log.Printf("[LLM Manager] Proveedor fallback '%s' no disponible: %v", p.Name(), err)
			continue
		}

		log.Printf("[LLM Manager] Usando proveedor fallback: %s", p.Name())
		resp, err := p.Chat(ctx, messages, tools, opts)
		if err == nil {
			return resp, nil
		}
		log.Printf("[LLM Manager] Error con proveedor fallback '%s': %v", p.Name(), err)
	}

	return nil, fmt.Errorf("todos los proveedores LLM fallaron, ningún fallback disponible")
}

// ListModels delega al proveedor activo.
func (m *Manager) ListModels(ctx context.Context) ([]ModelInfo, error) {
	active := m.Active()
	if active == nil {
		return nil, fmt.Errorf("no hay proveedor LLM activo configurado")
	}
	return active.ListModels(ctx)
}

// ListModelsForProvider lista modelos de un proveedor específico sin cambiar el proveedor activo.
func (m *Manager) ListModelsForProvider(ctx context.Context, providerName string) ([]ModelInfo, error) {
	p, ok := m.registry.Get(providerName)
	if !ok {
		return nil, fmt.Errorf("proveedor '%s' no está registrado", providerName)
	}
	return p.ListModels(ctx)
}

// ListAllModels lista modelos de todos los proveedores registrados.
func (m *Manager) ListAllModels(ctx context.Context) (map[string][]ModelInfo, error) {
	result := make(map[string][]ModelInfo)
	for _, p := range m.registry.List() {
		models, err := p.ListModels(ctx)
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

	active.SetModel(modelID)

	if m.db != nil {
		m.persistActiveProvider(m.ActiveName(), modelID)
	}

	log.Printf("[LLM Manager] Modelo cambiado a: %s", modelID)
	return nil
}

// SwitchProviderModel cambia el proveedor activo y su modelo en una sola operación explícita.
func (m *Manager) SwitchProviderModel(providerName, modelID string) error {
	p, ok := m.registry.Get(providerName)
	if !ok {
		return fmt.Errorf("proveedor '%s' no está registrado", providerName)
	}
	if modelID == "" {
		return fmt.Errorf("modelID requerido")
	}
	p.SetModel(modelID)

	m.mu.Lock()
	m.active = p
	m.activeID = providerName
	m.mu.Unlock()

	if m.db != nil {
		m.persistActiveProvider(providerName, modelID)
	}

	log.Printf("[LLM Manager] Proveedor activo establecido: %s (modelo: %s)", providerName, modelID)
	return nil
}

// LoadFromDB carga la configuración del proveedor activo desde la base de datos.
func (m *Manager) LoadFromDB() error {
	if m.db == nil {
		return nil
	}

	var providerName, modelID string
	err := m.db.QueryRow(`SELECT provider_name, model_id FROM llm_providers WHERE is_active = 1 LIMIT 1`).
		Scan(&providerName, &modelID)
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
	m.mu.Unlock()

	log.Printf("[LLM Manager] Proveedor cargado desde BD: %s (modelo: %s)", providerName, modelID)
	return nil
}

// persistActiveProvider guarda el proveedor activo en la tabla llm_providers.
func (m *Manager) persistActiveProvider(name, modelID string) {
	// Desactivar todos los proveedores
	_, _ = m.db.Exec(`UPDATE llm_providers SET is_active = 0`)

	// Insertar o actualizar el activo
	_, err := m.db.Exec(`
		INSERT INTO llm_providers (provider_name, provider_type, base_url, model_id, is_active, updated_at)
		VALUES (?, ?, '', ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_name) DO UPDATE SET
			model_id = excluded.model_id,
			is_active = 1,
			updated_at = CURRENT_TIMESTAMP
	`, name, name, modelID)
	if err != nil {
		log.Printf("[LLM Manager] Error persistiendo proveedor activo: %v", err)
	}
}

// SaveProviderConfig guarda la configuración completa de un proveedor en la BD.
func (m *Manager) SaveProviderConfig(cfg ProviderConfig) error {
	if m.db == nil {
		return fmt.Errorf("base de datos no disponible")
	}

	_, err := m.db.Exec(`
		INSERT INTO llm_providers (provider_name, provider_type, base_url, api_key_hash, model_id, is_active, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(provider_name) DO UPDATE SET
			provider_type = excluded.provider_type,
			base_url = excluded.base_url,
			api_key_hash = excluded.api_key_hash,
			model_id = excluded.model_id,
			is_active = excluded.is_active,
			updated_at = CURRENT_TIMESTAMP
	`, cfg.Name, cfg.Type, cfg.BaseURL, obfuscateKey(cfg.APIKey), cfg.Model, boolToInt(cfg.IsActive))

	return err
}

// GetProviderConfigs retorna todas las configuraciones de proveedores almacenadas.
func (m *Manager) GetProviderConfigs() ([]ProviderConfig, error) {
	if m.db == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	rows, err := m.db.Query(`SELECT provider_name, provider_type, base_url, model_id, is_active FROM llm_providers ORDER BY provider_name`)
	if err != nil {
		return nil, fmt.Errorf("error listando proveedores: %w", err)
	}
	defer rows.Close()

	var configs []ProviderConfig
	for rows.Next() {
		var cfg ProviderConfig
		var isActive int
		if err := rows.Scan(&cfg.Name, &cfg.Type, &cfg.BaseURL, &cfg.Model, &isActive); err != nil {
			continue
		}
		cfg.IsActive = isActive == 1
		configs = append(configs, cfg)
	}

	return configs, nil
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
