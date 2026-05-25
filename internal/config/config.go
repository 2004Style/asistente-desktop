package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent struct {
		Name         string   `yaml:"name"`
		Language     string   `yaml:"language"`
		LocalFirst   bool     `yaml:"local_first"`
		VoiceEnabled bool     `yaml:"voice_enabled"`
		WakeWords    []string `yaml:"wake_words"`
	} `yaml:"agent"`
	Personality struct {
		Role  string `yaml:"role"`
		Style string `yaml:"style"`
		Humor bool   `yaml:"humor"`
	} `yaml:"personality"`
	Model struct {
		Provider    string  `yaml:"provider"`
		BaseURL     string  `yaml:"base_url"`
		Model       string  `yaml:"model"`
		Temperature float64 `yaml:"temperature"`
		ToolCalling bool    `yaml:"tool_calling"`
	} `yaml:"model"`
	Database struct {
		Provider    string `yaml:"provider"`
		Path        string `yaml:"path"`
		EnableFTS   bool   `yaml:"enable_fts"`
		AutoMigrate bool   `yaml:"auto_migrate"`
	} `yaml:"database"`
	Skills struct {
		Path                 string `yaml:"path"`
		Format               string `yaml:"format"`
		AutoDiscover         bool   `yaml:"auto_discover"`
		RemoteInstallEnabled bool   `yaml:"remote_install_enabled"`
	} `yaml:"skills"`
	Mcp struct {
		Enabled    bool   `yaml:"enabled"`
		ConfigPath string `yaml:"config_path"`
	} `yaml:"mcp"`
	Files struct {
		AllowedRoots []string `yaml:"allowed_roots"`
		Ignore       []string `yaml:"ignore"`
		MaxDepth     int      `yaml:"max_depth"`
	} `yaml:"files"`
	Security struct {
		ConfirmHighRisk bool     `yaml:"confirm_high_risk"`
		BlockedPaths    []string `yaml:"blocked_paths"`
	} `yaml:"security"`
	Voice struct {
		PiperModel     string  `yaml:"piper_model"`
		WhisperModel   string  `yaml:"whisper_model"`
		VadThreshold   float64 `yaml:"vad_threshold"`
		WhisperThreads int     `yaml:"whisper_threads"`
		WhisperFlags   string  `yaml:"whisper_flags"`
	} `yaml:"voice"`
}

// LoadConfig lee o crea el archivo rbot.yaml con valores por defecto
func LoadConfig(path string) (*Config, error) {
	// Comprobar si existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Crear carpeta config
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}

		// Crear valores por defecto
		defaultConfig := &Config{}
		defaultConfig.Agent.Name = "RBot"
		defaultConfig.Agent.Language = "es"
		defaultConfig.Agent.LocalFirst = true
		defaultConfig.Agent.VoiceEnabled = true
		defaultConfig.Agent.WakeWords = []string{"oye ronald", "ey ronald", "go ronald", "hola ronald", "ronald", "rbot"}

		defaultConfig.Personality.Role = "operador personal de escritorio Linux"
		defaultConfig.Personality.Style = "mayordomo tecnológico elegante: sereno, preciso, discreto, técnico y confiable"
		defaultConfig.Personality.Humor = true

		defaultConfig.Model.Provider = "ollama"
		defaultConfig.Model.BaseURL = "http://localhost:11434"
		defaultConfig.Model.Model = "qwen2.5:7b" // Modelo compatible con tool calling
		defaultConfig.Model.Temperature = 0.2
		defaultConfig.Model.ToolCalling = true

		defaultConfig.Database.Provider = "sqlite"
		defaultConfig.Database.Path = "~/.local/share/rbot/rbot.db"
		defaultConfig.Database.EnableFTS = true
		defaultConfig.Database.AutoMigrate = true

		defaultConfig.Skills.Path = "~/.local/share/rbot/skills"
		defaultConfig.Skills.Format = "SKILL.md"
		defaultConfig.Skills.AutoDiscover = true
		defaultConfig.Skills.RemoteInstallEnabled = true

		defaultConfig.Mcp.Enabled = true
		defaultConfig.Mcp.ConfigPath = "~/.config/rbot/mcp_config.json"

		// Directorio actual e home
		home, _ := os.UserHomeDir()
		wd, err := os.Getwd()
		defaultConfig.Files.AllowedRoots = []string{}
		if err == nil && wd != "" {
			defaultConfig.Files.AllowedRoots = append(defaultConfig.Files.AllowedRoots, wd)
		}
		if home != "" {
			defaultConfig.Files.AllowedRoots = append(defaultConfig.Files.AllowedRoots, filepath.Join(home, "Documentos"))
			defaultConfig.Files.AllowedRoots = append(defaultConfig.Files.AllowedRoots, filepath.Join(home, "Descargas"))
			defaultConfig.Files.AllowedRoots = append(defaultConfig.Files.AllowedRoots, filepath.Join(home, "Escritorio"))
		}

		defaultConfig.Files.Ignore = []string{"node_modules", ".git", ".next", "bin", "build", "venv", "__pycache__"}
		defaultConfig.Files.MaxDepth = 6

		defaultConfig.Security.ConfirmHighRisk = true
		defaultConfig.Security.BlockedPaths = []string{
			"~/.ssh", "~/.gnupg", "~/.local/share/keyrings", "**/.env",
		}

		defaultConfig.Voice.PiperModel = "~/.local/share/rbot/models/piper/es_ES-davefx-medium.onnx"
		defaultConfig.Voice.WhisperModel = "~/.local/share/rbot/models/whisper/ggml-tiny.bin"
		defaultConfig.Voice.VadThreshold = 550.0
		defaultConfig.Voice.WhisperThreads = 8
		defaultConfig.Voice.WhisperFlags = ""

		data, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, err
		}

		NormalizeConfig(defaultConfig)
		return defaultConfig, nil
	}

	// Leer del disco
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var conf Config
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	NormalizeConfig(&conf)
	return &conf, nil
}

func expandPath(p string, home string) string {
	if strings.HasPrefix(p, "~") && home != "" {
		return filepath.Join(home, p[1:])
	}
	return p
}

// NormalizeConfig expande tildes/home, rutas relativas como '.' y las normaliza a rutas absolutas limpias.
func NormalizeConfig(conf *Config) {
	home, _ := os.UserHomeDir()

	conf.Mcp.ConfigPath = expandPath(conf.Mcp.ConfigPath, home)
	conf.Database.Path = expandPath(conf.Database.Path, home)
	conf.Skills.Path = expandPath(conf.Skills.Path, home)
	conf.Voice.PiperModel = expandPath(conf.Voice.PiperModel, home)
	conf.Voice.WhisperModel = expandPath(conf.Voice.WhisperModel, home)

	// Normalizar AllowedRoots
	for i, root := range conf.Files.AllowedRoots {
		root = expandPath(root, home)
		absPath, err := filepath.Abs(root)
		if err == nil {
			conf.Files.AllowedRoots[i] = filepath.Clean(absPath)
		} else {
			conf.Files.AllowedRoots[i] = filepath.Clean(root)
		}
	}

	// Normalizar BlockedPaths
	for i, blocked := range conf.Security.BlockedPaths {
		blocked = expandPath(blocked, home)
		if !strings.Contains(blocked, "*") {
			absPath, err := filepath.Abs(blocked)
			if err == nil {
				conf.Security.BlockedPaths[i] = filepath.Clean(absPath)
			} else {
				conf.Security.BlockedPaths[i] = filepath.Clean(blocked)
			}
		} else {
			conf.Security.BlockedPaths[i] = filepath.Clean(blocked)
		}
	}
}
