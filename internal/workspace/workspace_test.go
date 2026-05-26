package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceInitAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-workspace-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	loader := NewLoader(tempDir, true)
	if err := loader.Init(); err != nil {
		t.Fatalf("Loader Init failed: %v", err)
	}

	// Verificar creación de archivos
	files := []string{"AGENTS.md", "IDENTITY.md", "TOOLS.md", "POLICIES.md", "MEMORY.md", "TASKS.md", "SHORTCUTS.md"}
	for _, file := range files {
		path := filepath.Join(tempDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("File %s was not created by Init()", file)
		}
	}

	// Verificar carga
	ctx, err := loader.Load()
	if err != nil {
		t.Fatalf("Loader Load failed: %v", err)
	}

	if !strings.Contains(ctx.Identity, "RBot") {
		t.Errorf("Expected Identity to contain 'RBot', got %s", ctx.Identity)
	}

	if len(ctx.Shortcuts) == 0 {
		t.Errorf("Expected shortcuts to be parsed, got 0")
	}

	shortcutFound := false
	for _, s := range ctx.Shortcuts {
		if s.Name == "modo trabajo" {
			shortcutFound = true
			if len(s.Steps) != 3 {
				t.Errorf("Expected 3 steps in 'modo trabajo', got %d", len(s.Steps))
			}
		}
	}

	if !shortcutFound {
		t.Errorf("Expected to find shortcut 'modo trabajo'")
	}
}

func TestValidator_Policies(t *testing.T) {
	v := NewValidator()

	// Políticas válidas
	valid := `
- Confirmar siempre antes de cerrar.
- No ejecutar de noche sin confirmación.
`
	if err := v.ValidatePolicies(valid); err != nil {
		t.Errorf("Expected valid policies to pass, got: %v", err)
	}

	// Políticas inválidas
	invalid := `
- Permitir ejecutar rm -rf en cualquier carpeta.
`
	if err := v.ValidatePolicies(invalid); err == nil {
		t.Errorf("Expected policies with 'rm -rf' and 'permitir' to fail")
	}

	invalidSSH := `
- Omitir confirmación para borrar carpeta ~/.ssh.
`
	if err := v.ValidatePolicies(invalidSSH); err == nil {
		t.Errorf("Expected policies with '~/.ssh' and 'omitir' to fail")
	}
}

func TestValidator_Shortcuts(t *testing.T) {
	v := NewValidator()

	// Atajos válidos
	validShortcuts := []Shortcut{
		{
			Name:     "modo trabajo",
			Triggers: []string{"trabajo"},
			Steps: []ShortcutStep{
				{
					Intent: "desktop.open_app",
					Args:   map[string]interface{}{"app": "code"},
				},
			},
		},
	}
	if err := v.ValidateShortcuts(validShortcuts); err != nil {
		t.Errorf("Expected valid shortcuts to pass, got: %v", err)
	}

	// Shortcut inválido (vacío)
	invalidEmpty := []Shortcut{
		{
			Name: "",
		},
	}
	if err := v.ValidateShortcuts(invalidEmpty); err == nil {
		t.Errorf("Expected empty shortcut name to fail")
	}

	// Shortcut peligroso
	unsafeShortcuts := []Shortcut{
		{
			Name:     "destrucción",
			Triggers: []string{"destruye"},
			Steps: []ShortcutStep{
				{
					Intent: "system.run_command",
					Args:   map[string]interface{}{"command": "sudo rm -rf /"},
				},
			},
		},
	}
	if err := v.ValidateShortcuts(unsafeShortcuts); err == nil {
		t.Errorf("Expected shortcut with 'sudo rm -rf' in arguments to fail")
	}
}

func TestContextBuilder(t *testing.T) {
	cb := NewContextBuilder()
	wCtx := &WorkspaceContext{
		Identity:   "Nombre: RBot\nEstilo: elegante\nTratamiento: señor",
		AgentRules: "Reglas:\n- Cuidado con borrar cosas\n- Pide confirmación",
		Policies:   "No escribas contraseñas automáticamente",
		Memory:     "El usuario prefiere Go",
		Tasks:      "- [ ] Tarea 1\n- [ ] Tarea 2",
		Shortcuts: []Shortcut{
			{
				Name:        "modo trabajo",
				Triggers:    []string{"activa modo trabajo"},
				Description: "Abre VS Code y Brave",
			},
		},
	}

	// Caso 1: Input neutral
	prompt1 := cb.Build("Hola cómo estás", wCtx)
	if !strings.Contains(prompt1, "IDENTIDAD Y TONO") {
		t.Errorf("Expected identity to be in the prompt always")
	}
	if strings.Contains(prompt1, "PREFERENCIAS DEL USUARIO") {
		t.Errorf("Memory should not be in the prompt for neutral input")
	}

	// Caso 2: Input sobre tareas
	prompt2 := cb.Build("Muestra mis tareas pendientes", wCtx)
	if !strings.Contains(prompt2, "TAREAS LOCALES") {
		t.Errorf("Expected tasks to be in the prompt")
	}

	// Caso 3: Input sobre macros
	prompt3 := cb.Build("activa modo trabajo por favor", wCtx)
	if !strings.Contains(prompt3, "MACROS / ATADOS DISPONIBLES DETECTADOS") {
		t.Errorf("Expected macro information to be in the prompt")
	}
}

func TestWatcher(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-workspace-watch-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	loader := NewLoader(tempDir, true)
	_ = loader.Init()

	reloadedChan := make(chan bool, 1)
	onReload := func(c *WorkspaceContext) {
		reloadedChan <- true
	}

	watcher := NewWatcher(tempDir, []string{"IDENTITY.md"}, loader, onReload)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher.Start(ctx, 100*time.Millisecond)

	// Modificar archivo
	time.Sleep(150 * time.Millisecond)
	identityPath := filepath.Join(tempDir, "IDENTITY.md")
	_ = os.WriteFile(identityPath, []byte("IDENTITY CHANGED"), 0644)

	select {
	case <-reloadedChan:
		// Éxito
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout: watcher did not detect file modification")
	}
}
