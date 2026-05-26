package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS user_memory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    category TEXT NOT NULL,
    source TEXT,
    confidence REAL DEFAULT 1.0,
    is_sensitive INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(key, category)
);

CREATE TABLE IF NOT EXISTS path_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('file', 'directory', 'symlink', 'unknown')),
    extension TEXT,
    parent_path TEXT,
    size_bytes INTEGER,
    modified_at TEXT,
    last_seen_at TEXT,
    last_verified_at TEXT,
    exists_now INTEGER DEFAULT 1,
    is_stale INTEGER DEFAULT 0,
    source TEXT,
    confidence REAL DEFAULT 1.0,
    open_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS path_aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alias TEXT NOT NULL UNIQUE,
    path_entry_id INTEGER NOT NULL,
    description TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(path_entry_id) REFERENCES path_entries(id)
);

CREATE TABLE IF NOT EXISTS path_search_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    query TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    result_path TEXT,
    result_found INTEGER DEFAULT 0,
    search_roots TEXT,
    duration_ms INTEGER,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS folder_summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path_entry_id INTEGER NOT NULL,
    summary TEXT NOT NULL,
    file_count INTEGER DEFAULT 0,
    directory_count INTEGER DEFAULT 0,
    total_size_bytes INTEGER DEFAULT 0,
    content_hash TEXT,
    generated_by_model TEXT,
    generated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    is_stale INTEGER DEFAULT 0,
    FOREIGN KEY(path_entry_id) REFERENCES path_entries(id)
);

CREATE TABLE IF NOT EXISTS file_summaries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path_entry_id INTEGER NOT NULL,
    summary TEXT NOT NULL,
    language TEXT,
    content_hash TEXT,
    generated_by_model TEXT,
    generated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    is_stale INTEGER DEFAULT 0,
    FOREIGN KEY(path_entry_id) REFERENCES path_entries(id)
);

CREATE TABLE IF NOT EXISTS app_launchers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    display_name TEXT,
    executable TEXT NOT NULL,
    desktop_file TEXT,
    command TEXT NOT NULL,
    categories TEXT,
    icon TEXT,
    source TEXT,
    is_available INTEGER DEFAULT 1,
    last_verified_at TEXT,
    open_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, executable)
);

CREATE TABLE IF NOT EXISTS skills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    version TEXT,
    path TEXT NOT NULL,
    skill_md_path TEXT NOT NULL,
    frontmatter_json TEXT,
    permissions_json TEXT,
    risk_level TEXT DEFAULT 'medium',
    priority INTEGER DEFAULT 0,
    category TEXT,
    exclusive INTEGER DEFAULT 0,
    enabled INTEGER DEFAULT 0,
    trusted INTEGER DEFAULT 0,
    source TEXT,
    content_hash TEXT,
    installed_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skill_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id INTEGER NOT NULL,
    relative_path TEXT NOT NULL,
    file_type TEXT,
    content_hash TEXT,
    size_bytes INTEGER,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(skill_id) REFERENCES skills(id)
);

CREATE TABLE IF NOT EXISTS mcp_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    transport TEXT NOT NULL CHECK(transport IN ('stdio', 'http')),
    command TEXT,
    args_json TEXT,
    url TEXT,
    env_json TEXT,
    enabled INTEGER DEFAULT 0,
    trusted INTEGER DEFAULT 0,
    risk_level TEXT DEFAULT 'high',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mcp_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema_json TEXT,
    permission TEXT,
    risk_level TEXT DEFAULT 'medium',
    enabled INTEGER DEFAULT 1,
    requires_confirmation INTEGER DEFAULT 0,
    discovered_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(server_id) REFERENCES mcp_servers(id),
    UNIQUE(server_id, name)
);

CREATE TABLE IF NOT EXISTS internal_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    input_schema_json TEXT NOT NULL,
    permission TEXT NOT NULL,
    risk_level TEXT DEFAULT 'low',
    enabled INTEGER DEFAULT 1,
    requires_confirmation INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS action_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_input TEXT,
    selected_skill TEXT,
    tool_name TEXT,
    tool_source TEXT CHECK(tool_source IN ('internal', 'mcp')),
    arguments_json TEXT,
    result_json TEXT,
    success INTEGER,
    error TEXT,
    required_confirmation INTEGER DEFAULT 0,
    confirmed_by_user INTEGER DEFAULT 0,
    duration_ms INTEGER,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS environment_capabilities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT,
    available INTEGER DEFAULT 0,
    checked_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
    entity_type,
    entity_id,
    title,
    body,
    path,
    tokenize = 'unicode61'
);
`,
	},
	{
		Version: 2,
		Name:     "tool_registry",
		SQL: `
ALTER TABLE action_log ADD COLUMN plan_id TEXT;
ALTER TABLE action_log ADD COLUMN risk_level TEXT;
ALTER TABLE action_log ADD COLUMN status TEXT;
ALTER TABLE action_log ADD COLUMN started_at TEXT;
ALTER TABLE action_log ADD COLUMN finished_at TEXT;

CREATE TABLE IF NOT EXISTS tool_registry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    category TEXT,
    risk_level TEXT,
    schema_json TEXT,
    enabled INTEGER DEFAULT 1,
    requires_confirmation INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`,
	},
	{
		Version: 3,
		Name:     "pending_confirmations",
		SQL: `
CREATE TABLE IF NOT EXISTS pending_confirmations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL UNIQUE,
    plan_json TEXT NOT NULL,
    reason TEXT,
    source TEXT,
    session_id TEXT,
    status TEXT DEFAULT 'pending',
    expires_at TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`,
	},
	{
		Version: 4,
		Name:     "desktop_capabilities",
		SQL: `
SELECT 1;
`,
	},
	{
		Version: 5,
		Name:     "scheduler_tasks",
		SQL: `
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    priority TEXT DEFAULT 'normal',
    due_at TEXT,
    source TEXT DEFAULT 'voice',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tasks_status_due_at ON tasks(status, due_at);

CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    message TEXT,
    remind_at TEXT NOT NULL,
    recurrence_rule TEXT,
    channels_json TEXT,
    priority TEXT DEFAULT 'normal',
    status TEXT DEFAULT 'scheduled',
    source TEXT DEFAULT 'voice',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_reminders_status_remind_at ON reminders(status, remind_at);

CREATE TABLE IF NOT EXISTS meetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT,
    location TEXT,
    source TEXT DEFAULT 'local',
    external_id TEXT,
    notify_before_minutes INTEGER DEFAULT 10,
    status TEXT DEFAULT 'scheduled',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_meetings_status_starts_at ON meetings(status, starts_at);

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    run_at TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    last_error TEXT,
    locked_at TEXT,
    locked_by TEXT,
    completed_at TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_status_run_at ON scheduled_jobs(status, run_at);

CREATE TABLE IF NOT EXISTS notification_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel TEXT NOT NULL,
    title TEXT,
    message TEXT NOT NULL,
    status TEXT DEFAULT 'sent',
    error TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`,
	},
	{
		Version: 6,
		Name:     "workspace_skills",
		SQL: `
ALTER TABLE skills ADD COLUMN status TEXT DEFAULT 'disabled';
ALTER TABLE skills ADD COLUMN hash TEXT;
ALTER TABLE skills ADD COLUMN last_validated_at TEXT;
ALTER TABLE skills ADD COLUMN validation_errors TEXT;

CREATE TABLE IF NOT EXISTS skill_triggers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id INTEGER NOT NULL,
    trigger TEXT NOT NULL,
    trigger_type TEXT DEFAULT 'positive',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(skill_id) REFERENCES skills(id),
    UNIQUE(skill_id, trigger, trigger_type)
);

CREATE TABLE IF NOT EXISTS skill_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id INTEGER NOT NULL,
    permission TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(skill_id) REFERENCES skills(id),
    UNIQUE(skill_id, permission)
);

CREATE TABLE IF NOT EXISTS shortcuts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    triggers_json TEXT NOT NULL,
    plan_json TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    source TEXT DEFAULT 'workspace',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workspace_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`,
	},
	{
		Version: 7,
		Name:     "llm_providers_and_models",
		SQL: `
CREATE TABLE IF NOT EXISTS llm_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_name TEXT NOT NULL UNIQUE,
    provider_type TEXT NOT NULL,
    base_url TEXT DEFAULT '',
    api_key_hash TEXT DEFAULT '',
    model_id TEXT DEFAULT '',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS llm_models_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_name TEXT NOT NULL,
    model_id TEXT NOT NULL,
    model_name TEXT,
    family TEXT,
    size TEXT,
    tool_calling INTEGER DEFAULT 0,
    streaming INTEGER DEFAULT 0,
    vision INTEGER DEFAULT 0,
    conversation_state INTEGER DEFAULT 0,
    cached_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider_name, model_id)
);
CREATE INDEX IF NOT EXISTS idx_llm_models_cache_provider ON llm_models_cache(provider_name);
`,
	},
}

func RunMigrations(db *sql.DB) error {
	// Crear tabla de migraciones si no existe
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT DEFAULT CURRENT_TIMESTAMP
);
`)
	if err != nil {
		return fmt.Errorf("error creando tabla schema_migrations: %w", err)
	}

	for _, m := range migrations {
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", m.Version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error al verificar migración %d: %w", m.Version, err)
		}

		if exists > 0 {
			continue // Ya aplicada
		}

		log.Printf("[DB Migrations] Aplicando migración %d: %s...", m.Version, m.Name)

		// Ejecutar la migración dentro de una transacción para mantener la consistencia
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error iniciando transacción para migración %d: %w", m.Version, err)
		}

		// sqlite3 driver soporta múltiples sentencias separadas por punto y coma.
		// Ejecutamos todo el SQL de la migración.
		statements := strings.Split(m.SQL, ";")
		for _, stmt := range statements {
			stmtTrimmed := strings.TrimSpace(stmt)
			if stmtTrimmed == "" {
				continue
			}
			if _, err := tx.Exec(stmtTrimmed); err != nil {
				// Ignorar errores si la columna ya existe (por ejemplo, de migraciones manuales/semi-aplicadas previas)
				if strings.Contains(err.Error(), "duplicate column name") {
					log.Printf("[DB Migrations] Advertencia: la columna ya existe, ignorando. Sentencia: %s", stmtTrimmed)
					continue
				}
				tx.Rollback()
				return fmt.Errorf("error ejecutando migración %d (%s) en sentencia '%s': %w", m.Version, m.Name, stmtTrimmed, err)
			}
		}

		// Registrar migración completada
		_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.Version, m.Name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error registrando migración %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error haciendo commit de migración %d: %w", m.Version, err)
		}
		log.Printf("[DB Migrations] Migración %d aplicada con éxito.", m.Version)
	}

	return nil
}
