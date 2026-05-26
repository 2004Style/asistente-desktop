package runtime_test

import (
	"context"
	"testing"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/llm"
	"rbot/internal/runtime"
)

type runtimeMockProvider struct {
	name  string
	model string
}

func (m *runtimeMockProvider) Name() string       { return m.name }
func (m *runtimeMockProvider) ModelID() string    { return m.model }
func (m *runtimeMockProvider) SetModel(id string) { m.model = id }
func (m *runtimeMockProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	return &llm.Message{Role: "assistant", Content: "ok"}, nil
}
func (m *runtimeMockProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{{ID: m.model, Name: m.model, Provider: m.name}}, nil
}
func (m *runtimeMockProvider) Ping(ctx context.Context) error { return nil }

func TestDaemonProviderScopedModelCommands(t *testing.T) {
	reg := llm.NewRegistry()
	_ = reg.Register(&runtimeMockProvider{name: "a", model: "ma"})
	_ = reg.Register(&runtimeMockProvider{name: "b", model: "mb"})
	mgr := llm.NewManager(nil, reg)
	if err := mgr.SetActive("a"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	d := &runtime.Daemon{LLMManager: mgr}

	res, err := d.HandleCommand("models.list", map[string]interface{}{"provider": "b"})
	if err != nil {
		t.Fatalf("models.list with provider failed: %v", err)
	}
	models := res.([]map[string]interface{})
	if len(models) != 1 || models[0]["id"] != "mb" {
		t.Fatalf("expected provider b models, got %#v", models)
	}
	if mgr.ActiveName() != "a" {
		t.Fatalf("provider-scoped list must not change active provider, got %q", mgr.ActiveName())
	}

	_, err = d.HandleCommand("models.switch", map[string]interface{}{"provider": "b", "model": "mb-new"})
	if err != nil {
		t.Fatalf("models.switch provider model failed: %v", err)
	}
	if mgr.ActiveName() != "b" || mgr.Active().ModelID() != "mb-new" {
		t.Fatalf("expected active b/mb-new, got %s/%s", mgr.ActiveName(), mgr.Active().ModelID())
	}
}

func TestDaemonProcessInputBlocksCriticalCommand(t *testing.T) {
	// 1. Inicializar DB en memoria
	sqliteDB, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Error inicializando DB de pruebas: %v", err)
	}
	defer sqliteDB.Close()

	// Insertar una skill mock en la base de datos de pruebas para system.run_command_safe
	_, err = sqliteDB.Exec(`
		INSERT INTO skills (name, description, path, skill_md_path, frontmatter_json, enabled)
		VALUES (?, ?, '', '', ?, 1)
	`, "system.run_command_safe", "Ejecuta comandos", `{"name":"system.run_command_safe", "voice_triggers":["ejecuta", "ejecutar", "corre"], "risk_level":"high"}`)
	if err != nil {
		t.Fatalf("Error insertando skill de pruebas: %v", err)
	}

	// 2. Mockear configuración básica
	conf := &config.Config{}
	conf.Agent.Name = "TestBot"
	conf.Agent.WakeWords = []string{"rbot"}

	// 3. Crear daemon
	d := runtime.NewDaemon(conf, sqliteDB)

	// Registrar un listener para el EventBus para verificar que se emite "policy.blocked"
	ch := d.EventBus.Subscribe()
	defer d.EventBus.Unsubscribe(ch)

	// 4. Procesar comando crítico destructivo
	ctx := context.Background()
	resp, _, err := d.ProcessInput(ctx, "ejecuta sudo rm -rf /", "cli", "local_cli", nil)
	if err != nil {
		t.Fatalf("Expected no error from ProcessInput, got %v", err)
	}

	// 5. Validar que la respuesta indica bloqueo preventivo
	blockedEventReceived := false
Loop:
	for {
		select {
		case ev := <-ch:
			if ev.Type == "policy.blocked" {
				blockedEventReceived = true
			}
		default:
			break Loop
		}
	}

	if !blockedEventReceived {
		t.Error("Expected EventBus to receive policy.blocked event, but it didn't")
	}

	// Verificar la base de datos para asegurarse de que no se guardó confirmación para este plan
	var count int
	err = sqliteDB.QueryRow("SELECT COUNT(*) FROM pending_confirmations WHERE status = 'pending'").Scan(&count)
	if err != nil {
		t.Fatalf("Error querying pending confirmations: %v", err)
	}
	if count > 0 {
		t.Error("Expected 0 pending confirmations for critical commands, but found some saved")
	}

	// Verificar que el log de auditoría registró el bloqueo preventivo
	var logCount int
	err = sqliteDB.QueryRow("SELECT COUNT(*) FROM action_log WHERE status = 'denied'").Scan(&logCount)
	if err != nil {
		t.Fatalf("Error querying action log: %v", err)
	}
	if logCount == 0 {
		t.Error("Expected denied action to be logged in action_log, but it wasn't")
	}

	_ = resp
}

func TestDaemonWorkspaceAndShortcuts(t *testing.T) {
	// 1. Create temp directory for workspace
	tempDir := t.TempDir()

	// 2. Initialize DB in memory
	sqliteDB, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Error inicializando DB de pruebas: %v", err)
	}
	defer sqliteDB.Close()

	// 3. Set up config
	conf := &config.Config{}
	conf.Agent.Name = "TestBot"
	conf.Workspace.Path = tempDir
	conf.Workspace.AutoCreate = true
	conf.Workspace.WatchChanges = false

	// 4. Create daemon
	d := runtime.NewDaemon(conf, sqliteDB)

	// 5. Test workspace.status command
	statusRes, err := d.HandleCommand("workspace.status", nil)
	if err != nil {
		t.Fatalf("Error executing workspace.status: %v", err)
	}
	statusMap, ok := statusRes.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} result, got %T", statusRes)
	}
	if statusMap["path"] != tempDir {
		t.Errorf("Expected path %s, got %v", tempDir, statusMap["path"])
	}

	// 6. Test workspace.validate
	valRes, err := d.HandleCommand("workspace.validate", nil)
	if err != nil {
		t.Fatalf("Error executing workspace.validate: %v", err)
	}
	if valRes != "Políticas y shortcuts del workspace validados correctamente." {
		t.Errorf("Expected validation success message, got: %v", valRes)
	}

	// 7. Test shortcuts.list
	shortcutsRes, err := d.HandleCommand("shortcuts.list", nil)
	if err != nil {
		t.Fatalf("Error executing shortcuts.list: %v", err)
	}
	shortcutsList, ok := shortcutsRes.([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected []map[string]interface{} result, got %T", shortcutsRes)
	}
	// Default templates have exactly 1 shortcut initially (modo trabajo)
	if len(shortcutsList) != 1 {
		t.Errorf("Expected 1 shortcut, got %d", len(shortcutsList))
	}
}
