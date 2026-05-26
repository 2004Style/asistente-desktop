package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
)

func main() {
	// helper resolveProvidersPath local copy
	resolveProvidersPath := func(configPath, providersPath string) string {
		if providersPath == "" {
			providersPath = "providers.yaml"
		}
		if filepath.IsAbs(providersPath) || strings.HasPrefix(providersPath, "~") {
			return providersPath
		}
		baseDir := filepath.Dir(configPath)
		return filepath.Join(baseDir, filepath.Base(providersPath))
	}
	a := app.New()
	w := a.NewWindow("RBot Settings")
	w.Resize(fyne.NewSize(520, 300))

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		configPath = "config/rbot.yaml"
	}
	conf, _ := config.LoadConfig(configPath)
	providersConf, _ := config.LoadProvidersConfig(resolveProvidersPath(configPath, conf.Providers.ConfigFile))

	providerEntry := widget.NewEntry()
	providerEntry.SetText(conf.Providers.ActiveProvider)
	modelEntry := widget.NewEntry()
	modelEntry.SetText(conf.Providers.ActiveModel)
	secretEntry := widget.NewEntry()
	if p, ok := providersConf.Providers[conf.Providers.ActiveProvider]; ok {
		secretEntry.SetText(p.SecretRef)
	}

	statusLabel := widget.NewLabel("")
	testBtn := widget.NewButton("Test connection", func() {
		socket := db.ExpandPath(conf.Runtime.SocketPath)
		resp, err := ipc.SendCommandRPC(socket, "providers.status", nil, "providers-status-req")
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("Error: %v", err))
			return
		}
		statusLabel.SetText(fmt.Sprintf("Status: %v", resp.Result))
	})

	applyBtn := widget.NewButton("Apply", func() {
		socket := db.ExpandPath(conf.Runtime.SocketPath)
		params := map[string]interface{}{}
		prov := providerEntry.Text
		model := modelEntry.Text
		if prov != "" {
			params["provider"] = prov
		}
		if model != "" {
			params["model"] = model
		}
		_, err := ipc.SendCommandRPC(socket, "models.switch", params, "models-switch-req")
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("Apply error: %v", err))
			return
		}
		statusLabel.SetText("Applied")
	})

	onboardingBtn := widget.NewButton("Onboarding...", func() {
		// Launch non-interactive onboarding default; better to call CLI
		ctx := context.Background()
		// Use IPC? Instead, spawn rbot setup - but keep it simple: call providers.use after writing config via onboarding CLI would be better.
		statusLabel.SetText("Use rbot setup in terminal for full onboarding")
		_ = ctx
	})

	form := container.NewVBox(
		widget.NewLabel("Provider"), providerEntry,
		widget.NewLabel("Model"), modelEntry,
		widget.NewLabel("Secret ref (env:NAME or keyring:service/name)"), secretEntry,
		container.NewHBox(testBtn, applyBtn, onboardingBtn),
		statusLabel,
	)

	w.SetContent(form)
	w.ShowAndRun()
}
