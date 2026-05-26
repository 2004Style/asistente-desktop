package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProviderFileConfig struct {
	ConfigFile     string `yaml:"config_file"`
	ActiveProvider string `yaml:"active_provider"`
	ActiveModel    string `yaml:"active_model"`
}

type Config struct {
	Agent struct {
		Name         string   `yaml:"name"`
		Language     string   `yaml:"language"`
		LocalFirst   bool     `yaml:"local_first"`
		VoiceEnabled bool     `yaml:"voice_enabled"`
		WakeWords    []string `yaml:"wake_words"`
	} `yaml:"agent"`
	Runtime struct {
		SocketPath      string `yaml:"socket_path"`
		EventSocketPath string `yaml:"event_socket_path"`
		LogLevel        string `yaml:"log_level"`
	} `yaml:"runtime"`
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
	Providers ProviderFileConfig `yaml:"providers"`
	Database  struct {
		Provider    string `yaml:"provider"`
		Path        string `yaml:"path"`
		EnableFTS   bool   `yaml:"enable_fts"`
		AutoMigrate bool   `yaml:"auto_migrate"`
	} `yaml:"database"`
	Skills struct {
		Path                        string `yaml:"path"`
		WorkspaceSkillsPath         string `yaml:"workspace_skills_path"`
		Format                      string `yaml:"format"`
		AutoDiscover                bool   `yaml:"auto_discover"`
		AutoEnableLowRisk           bool   `yaml:"auto_enable_low_risk"`
		AutoEnableHighRisk          bool   `yaml:"auto_enable_high_risk"`
		RemoteInstallEnabled        bool   `yaml:"remote_install_enabled"`
		ValidateOnStartup           bool   `yaml:"validate_on_startup"`
		QuarantineOnFailures        bool   `yaml:"quarantine_on_failures"`
		MaxFailuresBeforeQuarantine int    `yaml:"max_failures_before_quarantine"`
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
	Desktop struct {
		Backend             string `yaml:"backend"`
		EnableHyprland      bool   `yaml:"enable_hyprland"`
		EnableSway          bool   `yaml:"enable_sway"`
		EnableX11           bool   `yaml:"enable_x11"`
		AllowWindowClose    bool   `yaml:"allow_window_close"`
		RequireConfirmClose bool   `yaml:"require_confirm_close_window"`
	} `yaml:"desktop"`
	Input struct {
		Enabled                 bool   `yaml:"enabled"`
		Backend                 string `yaml:"backend"`
		AllowKeyboard           bool   `yaml:"allow_keyboard"`
		AllowMouse              bool   `yaml:"allow_mouse"`
		RequireConfirmTextInput bool   `yaml:"require_confirm_text_input"`
		RequireConfirmHotkeys   bool   `yaml:"require_confirm_hotkeys"`
		BlockPasswordTyping     bool   `yaml:"block_password_typing"`
		MaxTextLengthMedium     int    `yaml:"max_text_length_medium"`
		MaxTextLengthHigh       int    `yaml:"max_text_length_high"`
	} `yaml:"input"`
	Browser struct {
		Default              string `yaml:"default"`
		ReuseTabs            bool   `yaml:"reuse_tabs"`
		YouTubeReuseExisting bool   `yaml:"youtube_reuse_existing"`
	} `yaml:"browser"`
	Media struct {
		PreferPlayerctl   bool   `yaml:"prefer_playerctl"`
		DefaultMusicQuery string `yaml:"default_music_query"`
	} `yaml:"media"`
	Scheduler struct {
		Enabled                bool `yaml:"enabled"`
		TickSeconds            int  `yaml:"tick_seconds"`
		RunMissedJobsOnStartup bool `yaml:"run_missed_jobs_on_startup"`
		MaxLateMinutes         int  `yaml:"max_late_minutes"`
	} `yaml:"scheduler"`
	Tasks struct {
		DefaultPriority string `yaml:"default_priority"`
		NotifyDueTasks  bool   `yaml:"notify_due_tasks"`
	} `yaml:"tasks"`
	Reminders struct {
		DefaultChannels []string `yaml:"default_channels"`
		AllowRecurring  bool     `yaml:"allow_recurring"`
		DefaultTime     string   `yaml:"default_time"`
	} `yaml:"reminders"`
	Meetings struct {
		DefaultNotifyBeforeMinutes int  `yaml:"default_notify_before_minutes"`
		NotifyTodaySummary         bool `yaml:"notify_today_summary"`
	} `yaml:"meetings"`
	Notifications struct {
		Desktop    bool `yaml:"desktop"`
		Voice      bool `yaml:"voice"`
		HUD        bool `yaml:"hud"`
		Sound      bool `yaml:"sound"`
		QuietHours struct {
			Enabled      bool   `yaml:"enabled"`
			Start        string `yaml:"start"`
			End          string `yaml:"end"`
			MuteVoice    bool   `yaml:"mute_voice"`
			MuteSound    bool   `yaml:"mute_sound"`
			AllowDesktop bool   `yaml:"allow_desktop"`
			AllowHUD     bool   `yaml:"allow_hud"`
		} `yaml:"quiet_hours"`
	} `yaml:"notifications"`
	Time struct {
		Timezone   string `yaml:"timezone"`
		StoreAsUTC bool   `yaml:"store_as_utc"`
	} `yaml:"time"`
	Hud struct {
		Enabled         bool   `yaml:"enabled"`
		Backend         string `yaml:"backend"`
		EventSocketPath string `yaml:"event_socket_path"`
		Window          struct {
			Width        int     `yaml:"width"`
			Height       int     `yaml:"height"`
			Position     string  `yaml:"position"`
			Transparent  bool    `yaml:"transparent"`
			Borderless   bool    `yaml:"borderless"`
			AlwaysOnTop  bool    `yaml:"always_on_top"`
			NoFocus      bool    `yaml:"no_focus"`
			ClickThrough bool    `yaml:"click_through"`
			Opacity      float64 `yaml:"opacity"`
		} `yaml:"window"`
		Behavior struct {
			ShowOnWake         bool `yaml:"show_on_wake"`
			HideOnSleep        bool `yaml:"hide_on_sleep"`
			HideDelayMs        int  `yaml:"hide_delay_ms"`
			Reconnect          bool `yaml:"reconnect"`
			ReconnectInitialMs int  `yaml:"reconnect_initial_ms"`
			ReconnectMaxMs     int  `yaml:"reconnect_max_ms"`
		} `yaml:"behavior"`
		Visual struct {
			Theme                  string  `yaml:"theme"`
			ShowTranscription      bool    `yaml:"show_transcription"`
			ShowToolStatus         bool    `yaml:"show_tool_status"`
			ShowNotifications      bool    `yaml:"show_notifications"`
			MaxNotificationSeconds int     `yaml:"max_notification_seconds"`
			AudioSmoothing         float64 `yaml:"audio_smoothing"`
		} `yaml:"visual"`
		Hyprland struct {
			ClassName      string `yaml:"class_name"`
			PrintRulesHint bool   `yaml:"print_rules_hint"`
		} `yaml:"hyprland"`
	} `yaml:"hud"`
	Workspace struct {
		Enabled          bool     `yaml:"enabled"`
		Path             string   `yaml:"path"`
		AutoCreate       bool     `yaml:"auto_create"`
		WatchChanges     bool     `yaml:"watch_changes"`
		ReloadDebounceMs int      `yaml:"reload_debounce_ms"`
		IncludeFiles     []string `yaml:"include_files"`
	} `yaml:"workspace"`
	Policies struct {
		Editable               bool `yaml:"editable"`
		AllowWorkspacePolicies bool `yaml:"allow_workspace_policies"`
		ImmutableCriticalRules bool `yaml:"immutable_critical_rules"`
	} `yaml:"policies"`
}

func defaultBlockedPaths() []string {
	return []string{
		"~/.ssh",
		"~/.gnupg",
		"~/.aws",
		"~/.config/rclone",
		"~/.config/gh",
		"~/.docker/config.json",
		"~/.local/share/keyrings",
		"**/.env",
		"**/.env.*",
		"**/id_rsa",
		"**/id_ed25519",
		"**/*.pem",
		"**/*.key",
		"**/*.p12",
		"**/cookies.sqlite",
		"**/Cookies",
		"**/Login Data",
	}
}

// DefaultConfig construye una configuración completa y segura para RBot.
func DefaultConfig() *Config {
	defaultConfig := &Config{}
	defaultConfig.Agent.Name = "RBot"
	defaultConfig.Agent.Language = "es"
	defaultConfig.Agent.LocalFirst = true
	defaultConfig.Agent.VoiceEnabled = true
	defaultConfig.Agent.WakeWords = []string{"oye ronald", "ey ronald", "go ronald", "hola ronald", "ronald", "rbot"}

	defaultConfig.Runtime.SocketPath = "~/.local/share/rbot/rbot.sock"
	defaultConfig.Runtime.EventSocketPath = "~/.local/share/rbot/events.sock"
	defaultConfig.Runtime.LogLevel = "info"

	defaultConfig.Personality.Role = "operador personal de escritorio Linux"
	defaultConfig.Personality.Style = "mayordomo tecnológico elegante: sereno, preciso, discreto, técnico y confiable"
	defaultConfig.Personality.Humor = true

	defaultConfig.Model.Provider = "ollama"
	defaultConfig.Model.BaseURL = "http://localhost:11434"
	defaultConfig.Model.Model = "qwen2.5:7b"
	defaultConfig.Model.Temperature = 0.2
	defaultConfig.Model.ToolCalling = true

	defaultConfig.Providers.ConfigFile = "config/providers.yaml"
	defaultConfig.Providers.ActiveProvider = "ollama"
	defaultConfig.Providers.ActiveModel = "qwen2.5:7b"

	defaultConfig.Database.Provider = "sqlite"
	defaultConfig.Database.Path = "~/.local/share/rbot/rbot.db"
	defaultConfig.Database.EnableFTS = true
	defaultConfig.Database.AutoMigrate = true

	defaultConfig.Skills.Path = "~/.local/share/rbot/skills"
	defaultConfig.Skills.WorkspaceSkillsPath = "~/.local/share/rbot/workspace/skills"
	defaultConfig.Skills.Format = "SKILL.md"
	defaultConfig.Skills.AutoDiscover = true
	defaultConfig.Skills.AutoEnableLowRisk = false
	defaultConfig.Skills.AutoEnableHighRisk = false
	defaultConfig.Skills.RemoteInstallEnabled = false
	defaultConfig.Skills.ValidateOnStartup = true
	defaultConfig.Skills.QuarantineOnFailures = true
	defaultConfig.Skills.MaxFailuresBeforeQuarantine = 3

	defaultConfig.Mcp.Enabled = true
	defaultConfig.Mcp.ConfigPath = "~/.config/rbot/mcp_config.json"

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
	defaultConfig.Files.Ignore = []string{"node_modules", ".git", ".next", "bin", "build", "venv", "__pycache__", ".cache", ".local/share/Trash", "snap"}
	defaultConfig.Files.MaxDepth = 6

	defaultConfig.Security.ConfirmHighRisk = true
	defaultConfig.Security.BlockedPaths = defaultBlockedPaths()

	defaultConfig.Voice.PiperModel = "~/.local/share/rbot/models/piper/es_ES-davefx-medium.onnx"
	defaultConfig.Voice.WhisperModel = "~/.local/share/rbot/models/whisper/ggml-tiny.bin"
	defaultConfig.Voice.VadThreshold = 550.0
	defaultConfig.Voice.WhisperThreads = 8
	defaultConfig.Voice.WhisperFlags = ""

	defaultConfig.Desktop.Backend = "auto"
	defaultConfig.Desktop.EnableHyprland = true
	defaultConfig.Desktop.EnableSway = true
	defaultConfig.Desktop.EnableX11 = true
	defaultConfig.Desktop.AllowWindowClose = true
	defaultConfig.Desktop.RequireConfirmClose = true

	defaultConfig.Input.Enabled = true
	defaultConfig.Input.Backend = "auto"
	defaultConfig.Input.AllowKeyboard = true
	defaultConfig.Input.AllowMouse = true
	defaultConfig.Input.RequireConfirmTextInput = false
	defaultConfig.Input.RequireConfirmHotkeys = false
	defaultConfig.Input.BlockPasswordTyping = true
	defaultConfig.Input.MaxTextLengthMedium = 200
	defaultConfig.Input.MaxTextLengthHigh = 1000

	defaultConfig.Browser.Default = "auto"
	defaultConfig.Browser.ReuseTabs = true
	defaultConfig.Browser.YouTubeReuseExisting = true

	defaultConfig.Media.PreferPlayerctl = true
	defaultConfig.Media.DefaultMusicQuery = "lofi hip hop beats"

	defaultConfig.Scheduler.Enabled = true
	defaultConfig.Scheduler.TickSeconds = 15
	defaultConfig.Scheduler.RunMissedJobsOnStartup = true
	defaultConfig.Scheduler.MaxLateMinutes = 120

	defaultConfig.Tasks.DefaultPriority = "normal"
	defaultConfig.Tasks.NotifyDueTasks = true

	defaultConfig.Reminders.DefaultChannels = []string{"desktop", "voice", "hud"}
	defaultConfig.Reminders.AllowRecurring = true
	defaultConfig.Reminders.DefaultTime = "09:00"

	defaultConfig.Meetings.DefaultNotifyBeforeMinutes = 10
	defaultConfig.Meetings.NotifyTodaySummary = true

	defaultConfig.Notifications.Desktop = true
	defaultConfig.Notifications.Voice = true
	defaultConfig.Notifications.HUD = true
	defaultConfig.Notifications.Sound = true
	defaultConfig.Notifications.QuietHours.Enabled = true
	defaultConfig.Notifications.QuietHours.Start = "23:00"
	defaultConfig.Notifications.QuietHours.End = "07:00"
	defaultConfig.Notifications.QuietHours.MuteVoice = true
	defaultConfig.Notifications.QuietHours.MuteSound = true
	defaultConfig.Notifications.QuietHours.AllowDesktop = true
	defaultConfig.Notifications.QuietHours.AllowHUD = true

	defaultConfig.Time.Timezone = "America/Lima"
	defaultConfig.Time.StoreAsUTC = true

	defaultConfig.Hud.Enabled = true
	defaultConfig.Hud.Backend = "gio"
	defaultConfig.Hud.EventSocketPath = "~/.local/share/rbot/events.sock"
	defaultConfig.Hud.Window.Width = 520
	defaultConfig.Hud.Window.Height = 420
	defaultConfig.Hud.Window.Position = "center"
	defaultConfig.Hud.Window.Transparent = true
	defaultConfig.Hud.Window.Borderless = true
	defaultConfig.Hud.Window.AlwaysOnTop = true
	defaultConfig.Hud.Window.NoFocus = true
	defaultConfig.Hud.Window.ClickThrough = true
	defaultConfig.Hud.Window.Opacity = 0.94
	defaultConfig.Hud.Behavior.ShowOnWake = true
	defaultConfig.Hud.Behavior.HideOnSleep = true
	defaultConfig.Hud.Behavior.HideDelayMs = 1200
	defaultConfig.Hud.Behavior.Reconnect = true
	defaultConfig.Hud.Behavior.ReconnectInitialMs = 500
	defaultConfig.Hud.Behavior.ReconnectMaxMs = 10000
	defaultConfig.Hud.Visual.Theme = "cyber-blue"
	defaultConfig.Hud.Visual.ShowTranscription = true
	defaultConfig.Hud.Visual.ShowToolStatus = true
	defaultConfig.Hud.Visual.ShowNotifications = true
	defaultConfig.Hud.Visual.MaxNotificationSeconds = 8
	defaultConfig.Hud.Visual.AudioSmoothing = 0.75
	defaultConfig.Hud.Hyprland.ClassName = "rbot-hud"
	defaultConfig.Hud.Hyprland.PrintRulesHint = true

	defaultConfig.Workspace.Enabled = true
	defaultConfig.Workspace.Path = "~/.local/share/rbot/workspace"
	defaultConfig.Workspace.AutoCreate = true
	defaultConfig.Workspace.WatchChanges = true
	defaultConfig.Workspace.ReloadDebounceMs = 500
	defaultConfig.Workspace.IncludeFiles = []string{"AGENTS.md", "IDENTITY.md", "TOOLS.md", "POLICIES.md", "MEMORY.md", "TASKS.md", "SHORTCUTS.md"}

	defaultConfig.Policies.Editable = true
	defaultConfig.Policies.AllowWorkspacePolicies = true
	defaultConfig.Policies.ImmutableCriticalRules = true

	return defaultConfig
}

// SaveConfig escribe la configuración en disco.
func SaveConfig(path string, conf *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadConfig lee o crea el archivo rbot.yaml con valores por defecto.
func LoadConfig(path string) (*Config, error) {
	conf := DefaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := SaveConfig(path, conf); err != nil {
			return nil, err
		}
		NormalizeConfig(conf)
		conf.Providers.ConfigFile = resolveProvidersConfigPath(path, conf.Providers.ConfigFile)
		return conf, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, err
	}

	NormalizeConfig(conf)
	conf.Providers.ConfigFile = resolveProvidersConfigPath(path, conf.Providers.ConfigFile)
	return conf, nil
}

func resolveProvidersConfigPath(configPath, providersPath string) string {
	if providersPath == "" || providersPath == "config/providers.yaml" {
		return filepath.Join(filepath.Dir(configPath), "providers.yaml")
	}
	if strings.HasPrefix(providersPath, "~") || filepath.IsAbs(providersPath) {
		return providersPath
	}
	return filepath.Join(filepath.Dir(configPath), providersPath)
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

	conf.Runtime.SocketPath = expandPath(conf.Runtime.SocketPath, home)
	conf.Runtime.EventSocketPath = expandPath(conf.Runtime.EventSocketPath, home)
	conf.Mcp.ConfigPath = expandPath(conf.Mcp.ConfigPath, home)
	conf.Database.Path = expandPath(conf.Database.Path, home)
	conf.Skills.Path = expandPath(conf.Skills.Path, home)
	conf.Voice.PiperModel = expandPath(conf.Voice.PiperModel, home)
	conf.Voice.WhisperModel = expandPath(conf.Voice.WhisperModel, home)
	conf.Hud.EventSocketPath = expandPath(conf.Hud.EventSocketPath, home)
	conf.Workspace.Path = expandPath(conf.Workspace.Path, home)
	conf.Skills.WorkspaceSkillsPath = expandPath(conf.Skills.WorkspaceSkillsPath, home)

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
