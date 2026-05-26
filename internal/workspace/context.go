package workspace

import "time"

type ShortcutStep struct {
	Intent string                 `yaml:"intent" json:"intent"`
	Args   map[string]interface{} `yaml:"args" json:"args"`
}

type Shortcut struct {
	Name        string         `yaml:"name" json:"name"`
	Triggers    []string       `yaml:"triggers" json:"triggers"`
	Description string         `yaml:"description" json:"description"`
	Steps       []ShortcutStep `yaml:"steps" json:"steps"`
}

type WorkspaceContext struct {
	AgentRules string
	Identity   string
	Tools      string
	Policies   string
	Memory     string
	Tasks      string
	Shortcuts  []Shortcut
	LoadedAt   time.Time
}
