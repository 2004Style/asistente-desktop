package onboarding

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"rbot/internal/config"
)

type Options struct {
	ConfigPath string
	In         io.Reader
	Out        io.Writer
}

func Run(ctx context.Context, opts Options) error {
	_ = ctx
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ConfigPath == "" {
		return fmt.Errorf("config path requerido")
	}

	conf, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}
	providersPath := resolveProvidersPath(opts.ConfigPath, conf.Providers.ConfigFile)
	providersConf, err := config.LoadProvidersConfig(providersPath)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(opts.In)
	fmt.Fprintln(opts.Out, "RBot onboarding")
	fmt.Fprintf(opts.Out, "Config: %s\n", opts.ConfigPath)
	fmt.Fprintf(opts.Out, "Providers: %s\n\n", providersPath)

	choice := prompt(opts.Out, reader, "Proveedor [1=ollama, 2=openai, 3=compatible] (default 1): ")
	selectedKey := "ollama"
	selectedType := "ollama"
	selectedModel := "qwen2.5:7b"
	selectedBaseURL := "http://localhost:11434"
	selectedSecretRef := ""
	selectedAuth := "none"

	switch strings.TrimSpace(choice) {
	case "2", "openai":
		selectedKey = "openai"
		selectedType = "openai"
		selectedModel = promptDefault(opts.Out, reader, "Modelo OpenAI (default gpt-4o-mini): ", "gpt-4o-mini")
		selectedBaseURL = promptDefault(opts.Out, reader, "Base URL OpenAI (default https://api.openai.com): ", "https://api.openai.com")
		selectedSecretRef = promptDefault(opts.Out, reader, "Referencia de secreto env (default env:OPENAI_API_KEY): ", "env:OPENAI_API_KEY")
		selectedAuth = "api_key"
	case "3", "compatible":
		selectedKey = promptDefault(opts.Out, reader, "Nombre del proveedor compatible (default compatible-local): ", "compatible-local")
		selectedType = "compatible"
		selectedBaseURL = promptRequired(opts.Out, reader, "Base URL compatible: ")
		selectedModel = promptRequired(opts.Out, reader, "Modelo compatible: ")
		selectedSecretRef = promptDefault(opts.Out, reader, "Referencia de secreto env (default env:COMPATIBLE_API_KEY): ", "env:COMPATIBLE_API_KEY")
		selectedAuth = "api_key"
	default:
		selectedKey = "ollama"
		selectedType = "ollama"
		selectedModel = promptDefault(opts.Out, reader, "Modelo Ollama (default qwen2.5:7b): ", "qwen2.5:7b")
		selectedBaseURL = promptDefault(opts.Out, reader, "Base URL Ollama (default http://localhost:11434): ", "http://localhost:11434")
		selectedAuth = "none"
	}

	providersConf.ActiveProvider = selectedKey
	providersConf.ActiveModel = selectedModel
	providersConf.Providers[selectedKey] = config.ProviderEntry{
		Enabled:   true,
		Type:      selectedType,
		AuthMode:  selectedAuth,
		BaseURL:   selectedBaseURL,
		SecretRef: selectedSecretRef,
		Model:     selectedModel,
	}
	if selectedKey != "ollama" {
		ollama := providersConf.Providers["ollama"]
		ollama.Enabled = true
		if ollama.Type == "" {
			ollama.Type = "ollama"
		}
		if ollama.BaseURL == "" {
			ollama.BaseURL = "http://localhost:11434"
		}
		if ollama.Model == "" {
			ollama.Model = "qwen2.5:7b"
		}
		providersConf.Providers["ollama"] = ollama
	}

	conf.Providers.ConfigFile = providersPath
	conf.Providers.ActiveProvider = selectedKey
	conf.Providers.ActiveModel = selectedModel
	conf.Model.Provider = selectedKey
	conf.Model.BaseURL = selectedBaseURL
	conf.Model.Model = selectedModel
	conf.Model.ToolCalling = true

	if err := config.SaveProvidersConfig(providersPath, providersConf); err != nil {
		return err
	}
	if err := config.SaveConfig(opts.ConfigPath, conf); err != nil {
		return err
	}

	fmt.Fprintln(opts.Out, "\nOnboarding completado.")
	fmt.Fprintf(opts.Out, "Proveedor activo: %s\n", selectedKey)
	fmt.Fprintf(opts.Out, "Modelo activo: %s\n", selectedModel)
	fmt.Fprintf(opts.Out, "Providers guardado en: %s\n", providersPath)
	return nil
}

func prompt(out io.Writer, reader *bufio.Reader, label string) string {
	fmt.Fprint(out, label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptDefault(out io.Writer, reader *bufio.Reader, label, def string) string {
	value := prompt(out, reader, label)
	if value == "" {
		return def
	}
	return value
}

func promptRequired(out io.Writer, reader *bufio.Reader, label string) string {
	for {
		value := prompt(out, reader, label)
		if value != "" {
			return value
		}
		fmt.Fprintln(out, "Valor requerido.")
	}
}

func resolveProvidersPath(configPath, providersPath string) string {
	if providersPath == "" {
		providersPath = "providers.yaml"
	}
	if filepath.IsAbs(providersPath) || strings.HasPrefix(providersPath, "~") {
		return providersPath
	}
	baseDir := filepath.Dir(configPath)
	return filepath.Join(baseDir, filepath.Base(providersPath))
}
