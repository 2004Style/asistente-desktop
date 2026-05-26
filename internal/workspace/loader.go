package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Loader struct {
	workspacePath string
	autoCreate    bool
}

func NewLoader(workspacePath string, autoCreate bool) *Loader {
	return &Loader{
		workspacePath: workspacePath,
		autoCreate:    autoCreate,
	}
}

func (l *Loader) Init() error {
	if !l.autoCreate {
		return nil
	}

	if err := os.MkdirAll(l.workspacePath, 0755); err != nil {
		return fmt.Errorf("error al crear el directorio del workspace %s: %w", l.workspacePath, err)
	}

	files := map[string]string{
		"AGENTS.md":    DefaultAgentsMD,
		"IDENTITY.md":  DefaultIdentityMD,
		"TOOLS.md":     DefaultToolsMD,
		"POLICIES.md":  DefaultPoliciesMD,
		"MEMORY.md":    DefaultMemoryMD,
		"TASKS.md":     DefaultTasksMD,
		"SHORTCUTS.md": DefaultShortcutsMD,
	}

	for filename, content := range files {
		path := filepath.Join(l.workspacePath, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("error al escribir plantilla por defecto %s: %w", filename, err)
			}
		}
	}

	// Crear carpeta local de skills si no existe
	localSkillsPath := filepath.Join(l.workspacePath, "skills", "local")
	_ = os.MkdirAll(localSkillsPath, 0755)

	return nil
}

func (l *Loader) Load() (*WorkspaceContext, error) {
	ctx := &WorkspaceContext{
		LoadedAt: time.Now(),
	}

	// Cargar AGENTS.md
	agentsPath := filepath.Join(l.workspacePath, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		ctx.AgentRules = string(data)
	}

	// Cargar IDENTITY.md
	identityPath := filepath.Join(l.workspacePath, "IDENTITY.md")
	if data, err := os.ReadFile(identityPath); err == nil {
		ctx.Identity = string(data)
	}

	// Cargar TOOLS.md
	toolsPath := filepath.Join(l.workspacePath, "TOOLS.md")
	if data, err := os.ReadFile(toolsPath); err == nil {
		ctx.Tools = string(data)
	}

	// Cargar POLICIES.md
	policiesPath := filepath.Join(l.workspacePath, "POLICIES.md")
	if data, err := os.ReadFile(policiesPath); err == nil {
		ctx.Policies = string(data)
	}

	// Cargar MEMORY.md
	memoryPath := filepath.Join(l.workspacePath, "MEMORY.md")
	if data, err := os.ReadFile(memoryPath); err == nil {
		ctx.Memory = string(data)
	}

	// Cargar TASKS.md
	tasksPath := filepath.Join(l.workspacePath, "TASKS.md")
	if data, err := os.ReadFile(tasksPath); err == nil {
		ctx.Tasks = string(data)
	}

	// Cargar SHORTCUTS.md y parsear macros YAML
	shortcutsPath := filepath.Join(l.workspacePath, "SHORTCUTS.md")
	if data, err := os.ReadFile(shortcutsPath); err == nil {
		shortcuts, err := parseShortcuts(string(data))
		if err == nil {
			ctx.Shortcuts = shortcuts
		}
	}

	return ctx, nil
}

func parseShortcuts(content string) ([]Shortcut, error) {
	var list []Shortcut

	startMarker := "```yaml"
	endMarker := "```"

	text := content
	for {
		startIdx := strings.Index(text, startMarker)
		if startIdx == -1 {
			// Intentar con marcador genérico ``` si no hay ```yaml
			startMarker = "```"
			startIdx = strings.Index(text, startMarker)
			if startIdx == -1 {
				break
			}
		}

		sub := text[startIdx+len(startMarker):]
		endIdx := strings.Index(sub, endMarker)
		if endIdx == -1 {
			break
		}

		yamlStr := sub[:endIdx]

		type ShortcutsWrapper struct {
			Shortcuts []Shortcut `yaml:"shortcuts"`
		}

		var wrapper ShortcutsWrapper
		if err := yaml.Unmarshal([]byte(yamlStr), &wrapper); err == nil {
			list = append(list, wrapper.Shortcuts...)
		}

		text = sub[endIdx+len(endMarker):]
		// Reset startMarker for next iterations
		startMarker = "```yaml"
	}

	return list, nil
}
