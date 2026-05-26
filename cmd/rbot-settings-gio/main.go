package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
)

type uiState struct {
	mu      sync.Mutex
	status  string
	busy    bool
	errText string
}

func main() {
	go func() {
		var w app.Window
		w.Option(
			app.Title("RBot Settings"),
			app.Size(unit.Dp(720), unit.Dp(420)),
			app.Decorated(true),
		)
		if err := loop(&w); err != nil {
			log.Printf("UI closed: %v", err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Palette.Bg = color.NRGBA{R: 0x07, G: 0x0B, B: 0x16, A: 0xFF}
	th.Palette.Fg = color.NRGBA{R: 0xD6, G: 0xE1, B: 0xFF, A: 0xFF}
	th.Palette.ContrastBg = color.NRGBA{R: 0x00, G: 0xE5, B: 0xFF, A: 0xFF}
	th.Palette.ContrastFg = color.NRGBA{R: 0x03, G: 0x0B, B: 0x14, A: 0xFF}
	th.TextSize = unit.Sp(16)
	var ops op.Ops
	state := &uiState{status: "Ready"}

	var providerEditor widget.Editor
	var modelEditor widget.Editor
	var secretEditor widget.Editor
	var testBtn widget.Clickable
	var applyBtn widget.Clickable

	providerEditor.SingleLine = true
	modelEditor.SingleLine = true
	secretEditor.SingleLine = true

	configPath := resolveConfigPath()
	conf, err := config.LoadConfig(configPath)
	if err != nil {
		state.setError(fmt.Sprintf("config load failed: %v", err))
	}
	providersConf, _ := config.LoadProvidersConfig(resolveProvidersPath(configPath, conf.Providers.ConfigFile))
	providerClicks := map[string]*widget.Clickable{}
	providerKeys := make([]string, 0, len(providersConf.Providers))
	for name := range providersConf.Providers {
		providerKeys = append(providerKeys, name)
		providerClicks[name] = new(widget.Clickable)
	}
	sort.Strings(providerKeys)

	providerEditor.SetText(conf.Providers.ActiveProvider)
	modelEditor.SetText(conf.Providers.ActiveModel)
	if p, ok := providersConf.Providers[conf.Providers.ActiveProvider]; ok {
		secretEditor.SetText(p.SecretRef)
	}

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			bg := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
			paint.Fill(gtx.Ops, th.Palette.Bg)
			bg.Pop()

			if testBtn.Clicked(gtx) && !state.isBusy() {
				state.setBusy(true)
				state.setStatus("Testing provider status...")
				go func() {
					defer state.setBusy(false)
					socket := db.ExpandPath(conf.Runtime.SocketPath)
					resp, err := ipc.SendCommandRPC(socket, "providers.status", nil, "providers-status-req")
					if err != nil {
						state.setError(fmt.Sprintf("Test failed: %v", err))
						w.Invalidate()
						return
					}
					state.setStatus(fmt.Sprintf("Status: %#v", resp.Result))
					w.Invalidate()
				}()
			}

			if applyBtn.Clicked(gtx) && !state.isBusy() {
				state.setBusy(true)
				state.setStatus("Applying selection...")
				prov := strings.TrimSpace(providerEditor.Text())
				model := strings.TrimSpace(modelEditor.Text())
				secret := strings.TrimSpace(secretEditor.Text())
				go func() {
					defer state.setBusy(false)
					params := map[string]any{}
					if prov != "" {
						params["provider"] = prov
					}
					if model != "" {
						params["model"] = model
					}
					if secret != "" {
						params["secret_ref"] = secret
					}
					socket := db.ExpandPath(conf.Runtime.SocketPath)
					_, err := ipc.SendCommandRPC(socket, "models.switch", params, "models-switch-req")
					if err != nil {
						state.setError(fmt.Sprintf("Apply failed: %v", err))
						w.Invalidate()
						return
					}
					state.setStatus("Applied")
					w.Invalidate()
				}()
			}

			layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				card := clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, 18).Push(gtx.Ops)
				paint.Fill(gtx.Ops, color.NRGBA{R: 0x0D, G: 0x13, B: 0x27, A: 0xFF})
				card.Pop()
				return layout.Inset{Top: unit.Dp(18), Left: unit.Dp(18), Right: unit.Dp(18), Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.H4(th, "RBot Settings").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Body1(th, "Neon control deck for local provider switching").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Overline(th, "Quick providers").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, providerButtonRow(gtx, th, providerKeys, providerClicks, &providerEditor, &modelEditor, &secretEditor, providersConf)...)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Editor(th, &providerEditor, "Provider").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Editor(th, &modelEditor, "Model").Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Editor(th, &secretEditor, "Secret ref (env:NAME or keyring:service/name)").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(material.Button(th, &testBtn, "Test connection").Layout),
								layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
								layout.Rigid(material.Button(th, &applyBtn, "Apply").Layout),
							)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Body2(th, state.snapshot()).Layout(gtx)
						}),
					)
				})
			})

			e.Frame(gtx.Ops)
		}
	}
}

func resolveConfigPath() string {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		return "config/rbot.yaml"
	}
	return configPath
}

func providerButtonRow(gtx layout.Context, th *material.Theme, keys []string, clicks map[string]*widget.Clickable, providerEditor, modelEditor, secretEditor *widget.Editor, providersConf *config.ProvidersConfig) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(keys)*2)
	for i, name := range keys {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
		}
		name := name
		click := clicks[name]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if click.Clicked(gtx) {
				providerEditor.SetText(name)
				if p, ok := providersConf.Providers[name]; ok {
					if p.Model != "" {
						modelEditor.SetText(p.Model)
					}
					if p.SecretRef != "" {
						secretEditor.SetText(p.SecretRef)
					}
				}
			}
			label := fmt.Sprintf("● %s", name)
			return material.Button(th, click, label).Layout(gtx)
		}))
	}
	return children
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

func (s *uiState) setStatus(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = v
	s.errText = ""
}

func (s *uiState) setError(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errText = v
	s.status = ""
}

func (s *uiState) setBusy(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.busy = v
}

func (s *uiState) isBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy
}

func (s *uiState) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.errText != "" {
		return s.errText
	}
	if s.busy {
		return "Working..."
	}
	return s.status
}
