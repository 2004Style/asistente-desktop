package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const Schema = `
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

CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
    entity_type,
    entity_id,
    title,
    body,
    path,
    tokenize = 'unicode61'
);
`

// ExpandPath expande la ruta si contiene el prefijo '~'.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// InitDB inicializa la base de datos SQLite y ejecuta las migraciones iniciales.
func InitDB(dbPath string) (*sql.DB, error) {
	fullPath := ExpandPath(dbPath)
	dir := filepath.Dir(fullPath)
	
	// Crear el directorio si no existe
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear el directorio de la base de datos: %v", err)
	}

	// Abrir la base de datos usando el driver de modernc (sqlite)
	db, err := sql.Open("sqlite", fullPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir sqlite: %v", err)
	}

	// Habilitar claves foráneas y busy timeout para evitar bloqueos por concurrencia
	if _, err := db.Exec("PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al configurar sqlite: %v", err)
	}

	// Intentar activar modo WAL de forma segura (no falla si el archivo está bloqueado por otro proceso)
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")

	// Ejecutar esquema inicial
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al crear el esquema inicial: %v", err)
	}

	// Migraciones (ignoramos errores si las columnas ya existen)
	_, _ = db.Exec("ALTER TABLE skills ADD COLUMN priority INTEGER DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE skills ADD COLUMN category TEXT;")
	_, _ = db.Exec("ALTER TABLE skills ADD COLUMN exclusive INTEGER DEFAULT 0;")

	return db, nil
}
