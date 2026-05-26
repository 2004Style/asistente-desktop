package main

import (
	"context"
	"fmt"
	"log"
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
	"rbot/internal/onboarding"
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
	w.Resize(fyne.NewSize(520, 340))

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		configPath = "config/rbot.yaml"
	}
	conf, _ := config.LoadConfig(configPath)
	providersPath := resolveProvidersPath(configPath, conf.Providers.ConfigFile)
	providersConf, _ := config.LoadProvidersConfig(providersPath)

	// Attempt to populate provider list via daemon IPC
	socket := db.ExpandPath(conf.Runtime.SocketPath)
	var providerNames []string
	if resp, err := ipc.SendCommandRPC(socket, "providers.list", nil, "providers-list-req"); err == nil {
		if resp.Error == nil {
			if list, ok := resp.Result.([]interface{}); ok {
				for _, p := range list {
					if pm, ok := p.(map[string]interface{}); ok {
						if name, _ := pm["name"].(string); name != "" {
							providerNames = append(providerNames, name)
						}
					}
				}
			}
		}
	} else {
		log.Printf("providers.list IPC failed: %v", err)
	}

	// Fallback to providers from providers.yaml
	if len(providerNames) == 0 {
		for name := range providersConf.Providers {
			providerNames = append(providerNames, name)
		}
	}

	// UI widgets
	providerSelect := widget.NewSelect(providerNames, func(s string) {})
	providerSelect.PlaceHolder = "Select provider"
	providerSelect.SetSelected(conf.Providers.ActiveProvider)

	modelEntry := widget.NewEntry()
	modelEntry.SetText(conf.Providers.ActiveModel)

	secretEntry := widget.NewPasswordEntry()
	if p, ok := providersConf.Providers[conf.Providers.ActiveProvider]; ok {
		secretEntry.SetText(p.SecretRef)
	}
	showSecret := widget.NewCheck("Show secret", func(b bool) {
		secretEntry.Password = !b
		secretEntry.Refresh()
	})

	statusLabel := widget.NewLabel("")

	// Test connection with basic error classification
	testBtn := widget.NewButton("Test connection", func() {
		go func() {
			socket := db.ExpandPath(conf.Runtime.SocketPath)
			resp, err := ipc.SendCommandRPC(socket, "providers.status", nil, "providers-status-req")
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("Error connecting to daemon: %v", err))
				return
			}
			if resp.Error != nil {
				statusLabel.SetText(fmt.Sprintf("Daemon error: %s", resp.Error.Message))
				return
			}
			m, ok := resp.Result.(map[string]interface{})
			if !ok {
				statusLabel.SetText(fmt.Sprintf("Unexpected response: %v", resp.Result))
				return
			}
			stRaw, _ := m["status"].(string)
			if strings.HasPrefix(stRaw, "error:") {
				msg := strings.TrimSpace(strings.TrimPrefix(stRaw, "error:"))
				low := strings.ToLower(msg)
				if strings.Contains(low, "environment secret") || strings.Contains(low, "keyring") || strings.Contains(low, "secret reference") || strings.Contains(low, "unsupported") {
					statusLabel.SetText(fmt.Sprintf("Secret resolution error: %s", msg))
					return
				}
				statusLabel.SetText(fmt.Sprintf("Provider ping error: %s", msg))
				return
			}
			statusLabel.SetText(fmt.Sprintf("Provider: %v — Status: %v", m["provider"], m["status"]))
		}()
	})

	// Apply (runtime change via IPC). Use models.switch which can change provider+model.
	applyBtn := widget.NewButton("Apply", func() {
		go func() {
			socket := db.ExpandPath(conf.Runtime.SocketPath)
			params := map[string]interface{}{}
			prov := providerSelect.Selected
			model := modelEntry.Text
			if prov != "" {
				params["provider"] = prov
			}
			if model != "" {
				params["model"] = model
			}
			resp, err := ipc.SendCommandRPC(socket, "models.switch", params, "models-switch-req")
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("Apply error: %v", err))
				return
			}
			if resp.Error != nil {
				statusLabel.SetText(fmt.Sprintf("Daemon error: %s", resp.Error.Message))
				return
			}
			statusLabel.SetText("Applied (runtime)")
		}()
	})

	// Save to config (persistence) via onboarding non-interactive
	saveBtn := widget.NewButton("Save to config", func() {
		go func() {
			opts := onboarding.Options{
				ConfigPath:     configPath,
				NonInteractive: true,
				Provider:       providerSelect.Selected,
				Model:          modelEntry.Text,
				SecretRef:      secretEntry.Text,
			}
			if err := onboarding.Run(context.Background(), opts); err != nil {
				statusLabel.SetText(fmt.Sprintf("Save error: %v", err))
				return
			}
			statusLabel.SetText("Saved to providers config")
		}()
	})

	onboardingBtn := widget.NewButton("Onboarding...", func() {
		statusLabel.SetText("Run 'rbot setup' in terminal for full onboarding")
	})

	// When provider selection changes, populate model/secret from providers.yaml if available
	providerSelect.OnChanged = func(s string) {
		if p, ok := providersConf.Providers[s]; ok {
			modelEntry.SetText(p.Model)
			secretEntry.SetText(p.SecretRef)
		} else {
			modelEntry.SetText("")
			secretEntry.SetText("")
		}
	}

	form := container.NewVBox(
		widget.NewLabel("Provider"), providerSelect,
		widget.NewLabel("Model"), modelEntry,
		widget.NewLabel("Secret ref (env:NAME or keyring:service/name)"), secretEntry, showSecret,
		container.NewHBox(testBtn, applyBtn, saveBtn, onboardingBtn),
		statusLabel,
	)

	w.SetContent(form)
	w.ShowAndRun()
}
