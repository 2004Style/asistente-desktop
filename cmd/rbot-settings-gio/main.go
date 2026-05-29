package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/zalando/go-keyring"
	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
	"rbot/internal/llm"
)

type uiState struct {
	mu      sync.Mutex
	status  string
	busy    bool
	phase   float64
	errText string

	// Selected states
	selectedProvider string
	selectedAuthMode string
	selectedModel    string

	// Clicks mapping for dynamic immediate mode rendering
	providerClicks map[string]*widget.Clickable
	authModeClicks map[string]*widget.Clickable
	modelClicks    map[string]*widget.Clickable

	// Form controllers
	apiKeyEditor widget.Editor

	// Action buttons
	loginBtn widget.Clickable
	testBtn  widget.Clickable
	saveBtn  widget.Clickable

	// Config and paths
	configPath    string
	providersConf *config.ProvidersConfig
	conf          *config.Config
}

func main() {
	go func() {
		var w app.Window
		w.Option(
			app.Title("RBot Settings Panel"),
			app.Size(unit.Dp(800), unit.Dp(600)),
			app.Decorated(true),
		)
		if err := loop(&w); err != nil {
			log.Printf("UI closed with error: %v", err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func initUIState() *uiState {
	configPath := resolveConfigPath()
	conf, err := config.LoadConfig(configPath)
	if err != nil || conf == nil {
		log.Printf("Error cargando rbot.yaml: %v. Usando valores por defecto.", err)
		conf = config.DefaultConfig()
	}
	providersPath := resolveProvidersPath(configPath, conf.Providers.ConfigFile)
	providersConf, err := config.LoadProvidersConfig(providersPath)
	if err != nil || providersConf == nil {
		log.Printf("Error cargando providers.yaml: %v. Usando valores por defecto.", err)
		providersConf = config.DefaultProvidersConfig()
	}

	state := &uiState{
		status:         "Ready",
		configPath:     configPath,
		providersConf:  providersConf,
		conf:           conf,
		providerClicks: make(map[string]*widget.Clickable),
		authModeClicks: make(map[string]*widget.Clickable),
		modelClicks:    make(map[string]*widget.Clickable),
	}

	state.apiKeyEditor.SingleLine = true

	// Pre-populate clickables
	for _, p := range []string{"ollama", "openai", "anthropic", "google_gemini", "openrouter", "deepseek"} {
		state.providerClicks[p] = new(widget.Clickable)
	}

	for _, a := range []string{"none", "api_key", "browser_oauth", "adc", "service_account"} {
		state.authModeClicks[a] = new(widget.Clickable)
	}

	// Initialize active selections
	activeSelection := providersConf.ResolveActiveSelection()
	state.selectedProvider = activeSelection.ProviderName
	if state.selectedProvider == "" {
		state.selectedProvider = conf.Providers.ActiveProvider
	}
	if state.selectedProvider == "" {
		state.selectedProvider = "ollama"
	}

	state.selectedModel = activeSelection.ModelID
	if state.selectedModel == "" {
		state.selectedModel = conf.Providers.ActiveModel
	}

	state.selectedAuthMode = activeSelection.AuthMode
	if state.selectedAuthMode == "" {
		state.selectedAuthMode = conf.Providers.ActiveAuthMode
	}
	if state.selectedAuthMode == "" {
		state.selectedAuthMode = "none"
	}

	// Load existing secret reference
	secretRef := activeSelection.SecretRef
	if secretRef == "" {
		if prov, ok := providersConf.Providers[state.selectedProvider]; ok {
			secretRef = prov.SecretRef
		}
	}
	state.loadSecretPlaceholder(secretRef)

	return state
}

func (s *uiState) loadSecretPlaceholder(secretRef string) {
	if secretRef != "" {
		if strings.HasPrefix(secretRef, "keyring:") {
			s.apiKeyEditor.SetText("[Clave guardada de forma segura en Llavero]")
		} else if strings.HasPrefix(secretRef, "env:") {
			s.apiKeyEditor.SetText("[Variable de entorno: " + secretRef[4:] + "]")
		} else if strings.HasPrefix(secretRef, "plain:") {
			s.apiKeyEditor.SetText(secretRef[6:])
		} else {
			s.apiKeyEditor.SetText(secretRef)
		}
	} else {
		s.apiKeyEditor.SetText("")
	}
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Palette.Bg = color.NRGBA{R: 0x05, G: 0x0A, B: 0x18, A: 0xFF} // Sleek cyber dark
	th.Palette.Fg = color.NRGBA{R: 0xE2, G: 0xEE, B: 0xFF, A: 0xFF}
	th.Palette.ContrastBg = color.NRGBA{R: 0x00, G: 0xF5, B: 0xFF, A: 0xFF} // Neon cyan
	th.Palette.ContrastFg = color.NRGBA{R: 0x02, G: 0x05, B: 0x0B, A: 0xFF}
	th.TextSize = unit.Sp(15)

	state := initUIState()
	var ops op.Ops

	// Micro-animation timer
	go func() {
		t := time.NewTicker(40 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			state.mu.Lock()
			state.phase = math.Mod(state.phase+0.02, 1)
			state.mu.Unlock()
			w.Invalidate()
		}
	}()

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			paint.Fill(gtx.Ops, th.Palette.Bg)
			phase := state.phaseValue()



			// Handle provider click
			for pName, click := range state.providerClicks {
				if click.Clicked(gtx) {
					state.selectedProvider = pName

					// Load provider defaults
					if entry, ok := state.providersConf.Providers[pName]; ok {
						state.selectedModel = entry.DefaultModel
						if state.selectedModel == "" {
							state.selectedModel = entry.Model
						}
						authModes := getAuthModesForProvider(pName, state.providersConf)
						if len(authModes) > 0 {
							state.selectedAuthMode = authModes[0]
						}
						state.loadSecretPlaceholder(entry.SecretRef)
					} else {
						// Fallbacks
						authModes := getAuthModesForProvider(pName, nil)
						state.selectedAuthMode = authModes[0]
						models := getModelsForProvider(pName, nil)
						state.selectedModel = models[0]
						state.apiKeyEditor.SetText("")
					}
				}
			}

			// Handle auth mode click
			for aMode, click := range state.authModeClicks {
				if click.Clicked(gtx) {
					state.selectedAuthMode = aMode
					// Try loading reference from providers config
					if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok {
						if cap, ok := entry.AuthModes[aMode]; ok && cap.SecretRef != "" {
							state.loadSecretPlaceholder(cap.SecretRef)
						} else {
							state.loadSecretPlaceholder(entry.SecretRef)
						}
					}
				}
			}

			// Handle model click
			for mID, click := range state.modelClicks {
				if click.Clicked(gtx) {
					state.selectedModel = mID
				}
			}

			// Handle OAuth Browser Login
			if state.loginBtn.Clicked(gtx) && !state.isBusy() {
				state.setBusy(true)
				state.setStatus("Esperando inicio de sesión en el navegador...")
				go func() {
					defer state.setBusy(false)
					provName := state.selectedProvider
					// Start local auth server
					token, err := llm.StartBrowserOAuth(context.Background(), provName, "127.0.0.1", "/auth/callback")
					if err != nil {
						state.setError(fmt.Sprintf("Login falló: %v", err))
						w.Invalidate()
						return
					}
					// Save token in Keyring
					err = keyring.Set("rbot", provName+"_session", token)
					if err != nil {
						// Fallback to storing as plain session
						log.Printf("Keyring not available: %v. Storing token as plain text ref.", err)
						state.setStatus("Sesión recibida (guardada temporalmente)")
					} else {
						state.setStatus("¡Sesión iniciada y token guardado en Llavero!")
					}
					w.Invalidate()
				}()
			}

			// Handle Test Connection
			if state.testBtn.Clicked(gtx) && !state.isBusy() {
				state.setBusy(true)
				state.setStatus("Comprobando estado del proveedor...")
				go func() {
					defer state.setBusy(false)
					socket := db.ExpandPath(state.conf.Runtime.SocketPath)
					resp, err := ipc.SendCommandRPC(socket, "providers.status", nil, "providers-status-req")
					if err != nil {
						state.setError(fmt.Sprintf("Prueba fallida: %v", err))
						w.Invalidate()
						return
					}
					state.setStatus(fmt.Sprintf("Ping: %v", resp.Result))
					w.Invalidate()
				}()
			}

			// Handle Apply / Save
			if state.saveBtn.Clicked(gtx) && !state.isBusy() {
				state.setBusy(true)
				state.setStatus("Guardando cambios...")
				go func() {
					defer state.setBusy(false)

					keyInput := strings.TrimSpace(state.apiKeyEditor.Text())
					finalSecretRef := ""

					if keyInput != "" {
						if strings.HasPrefix(keyInput, "[Clave guardada") {
							// Kept placeholder, load previous
							if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok {
								finalSecretRef = entry.SecretRef
							}
						} else if strings.HasPrefix(keyInput, "[Variable de entorno:") {
							// Kept env placeholder
							if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok {
								finalSecretRef = entry.SecretRef
							}
						} else {
							// Raw key input, resolve prefix/scheme
							if strings.HasPrefix(keyInput, "env:") || strings.HasPrefix(keyInput, "plain:") || strings.HasPrefix(keyInput, "keyring:") {
								finalSecretRef = keyInput
							} else {
								// Save to system keyring
								accountName := state.selectedProvider + "_api_key"
								err := keyring.Set("rbot", accountName, keyInput)
								if err != nil {
									log.Printf("Keyring not available: %v. Storing in plain text config.", err)
									finalSecretRef = "plain:" + keyInput
								} else {
									finalSecretRef = "keyring:" + accountName
								}
							}
						}
					}

					// Set browser oauth session reference if selected
					if state.selectedAuthMode == "browser_oauth" && finalSecretRef == "" {
						finalSecretRef = "keyring:" + state.selectedProvider + "_session"
					}

					// Update providers entry
					entry, ok := state.providersConf.Providers[state.selectedProvider]
					if !ok {
						entry = config.ProviderEntry{Enabled: true}
					}
					entry.Enabled = true
					entry.AuthMode = state.selectedAuthMode
					entry.SecretRef = finalSecretRef
					entry.DefaultModel = state.selectedModel
					entry.Model = state.selectedModel

					// Add capability information
					billingMode, runtimeMode := getBillingAndRuntimeMode(state.selectedProvider, state.selectedAuthMode, state.providersConf)
					entry.BillingMode = billingMode
					entry.RuntimeMode = runtimeMode

					state.providersConf.Providers[state.selectedProvider] = entry

					// Update active selectors
					state.providersConf.ActiveProfile = ""
					state.providersConf.ActiveProvider = state.selectedProvider
					state.providersConf.ActiveModel = state.selectedModel
					state.providersConf.ActiveAuthMode = state.selectedAuthMode
					state.providersConf.Active = config.ActiveConfig{
						Provider:    state.selectedProvider,
						Model:       state.selectedModel,
						AuthProfile: "",
					}

					// Write providers.yaml
					provPath := resolveProvidersPath(state.configPath, state.conf.Providers.ConfigFile)
					err := config.SaveProvidersConfig(provPath, state.providersConf)
					if err != nil {
						state.setError(fmt.Sprintf("Error guardando proveedores: %v", err))
						w.Invalidate()
						return
					}

					// Write rbot.yaml
					state.conf.Providers.ActiveProfile = ""
					state.conf.Providers.ActiveProvider = state.selectedProvider
					state.conf.Providers.ActiveModel = state.selectedModel
					state.conf.Providers.ActiveAuthMode = state.selectedAuthMode
					state.conf.Model.Provider = state.selectedProvider
					state.conf.Model.Model = state.selectedModel

					if state.selectedProvider == "ollama" {
						state.conf.Model.BaseURL = "http://localhost:11434"
					} else if entry.BaseURL != "" {
						state.conf.Model.BaseURL = entry.BaseURL
					} else {
						state.conf.Model.BaseURL = "https://api.openai.com/v1"
						for _, cap := range entry.Capabilities {
							if cap.AuthMode == state.selectedAuthMode && cap.BaseURL != "" {
								state.conf.Model.BaseURL = cap.BaseURL
								break
							}
						}
					}

					err = config.SaveConfig(state.configPath, state.conf)
					if err != nil {
						state.setError(fmt.Sprintf("Error guardando rbot.yaml: %v", err))
						w.Invalidate()
						return
					}

					// Apply config change via IPC
					socket := db.ExpandPath(state.conf.Runtime.SocketPath)
					params := map[string]any{"name": ""}
					_, err = ipc.SendCommandRPC(socket, "profiles.use", params, "profiles-use-req")
					if err != nil {
						log.Printf("Daemon offline, did not apply hot reload: %v", err)
						state.setStatus("Guardado en disco. Daemon offline.")
					} else {
						state.setStatus("¡Configuración guardada y aplicada!")
					}
					w.Invalidate()
				}()
			}

			// RENDER THE VIEW LAYOUT
			layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Header
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.H5(th, "✦ RBot Settings Panel").Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return liveBadge(gtx, th, phase)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),

					// Main deck split in two columns
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							// Column 1: Providers Sidebar
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Max.X = gtx.Dp(240)
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									// Provider selector list
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return material.Overline(th, "◆ PROVEEDORES").Layout(gtx)
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										list := []string{"ollama", "openai", "anthropic", "google_gemini", "openrouter", "deepseek"}
										children := make([]layout.FlexChild, 0, len(list))
										for _, name := range list {
											name := name
											lbl := strings.ToUpper(name)
											if state.selectedProvider == name {
												lbl = "◀ " + lbl
											}
											btn := state.providerClicks[name]
											children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													return material.Button(th, btn, lbl).Layout(gtx)
												})
											}))
										}
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
									}),
								)
							}),

							layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),

							// Column 2: Provider Capabilities & Authentication Config
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								cardRect := image.Rectangle{Max: gtx.Constraints.Max}
								card := clip.UniformRRect(cardRect, 12).Push(gtx.Ops)
								paint.Fill(gtx.Ops, color.NRGBA{R: 0x0D, G: 0x14, B: 0x2A, A: 0xFF}) // translucent panel
								card.Pop()

								return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									billing, runtime := getBillingAndRuntimeMode(state.selectedProvider, state.selectedAuthMode, state.providersConf)
									authModes := getAuthModesForProvider(state.selectedProvider, state.providersConf)
									models := getModelsForProvider(state.selectedProvider, state.providersConf)

									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										// Title provider details
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											title := "Configuración de " + strings.Title(state.selectedProvider)
											return material.H6(th, title).Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),

										// Read-only Capability badges
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return neonStatCard(gtx, th, "Facturación (billing_mode)", billing, accentColor(phase+0.1))
												}),
												layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return neonStatCard(gtx, th, "Ejecución (runtime_mode)", runtime, accentColor(phase+0.4))
												}),
											)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

										// Authentication Modes Selection
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return material.Overline(th, "MÉTODO DE AUTENTICACIÓN (auth_mode)").Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											children := make([]layout.FlexChild, 0, len(authModes)*2)
											for i, mode := range authModes {
												if i > 0 {
													children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
												}
												mode := mode
												lbl := mode
												if state.selectedAuthMode == mode {
													lbl = "◉ " + mode
												}
												btn := state.authModeClicks[mode]
												children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return material.Button(th, btn, lbl).Layout(gtx)
												}))
											}
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

										// Dynamic fields based on auth mode
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											switch state.selectedAuthMode {
											case "api_key":
												return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return material.Overline(th, "CLAVE DE API (secret_ref)").Layout(gtx)
													}),
													layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														// Render a nice single-line text input field
														return layout.Stack{}.Layout(gtx,
															layout.Expanded(func(gtx layout.Context) layout.Dimensions {
																r := image.Rectangle{Max: gtx.Constraints.Min}
																r.Max.Y = gtx.Dp(36)
																shape := clip.UniformRRect(r, 6).Op(gtx.Ops)
																paint.FillShape(gtx.Ops, color.NRGBA{R: 0x14, G: 0x1E, B: 0x3C, A: 0xFF}, shape)
																return layout.Dimensions{Size: r.Max}
															}),
															layout.Stacked(func(gtx layout.Context) layout.Dimensions {
																return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
																	return material.Editor(th, &state.apiKeyEditor, "Pegar clave aquí...").Layout(gtx)
																})
															}),
														)
													}),
												)
											case "browser_oauth":
												return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return material.Caption(th, "Autenticación segura basada en tokens mediante el navegador web (OAuth/PKCE).").Layout(gtx)
													}),
													layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return material.Button(th, &state.loginBtn, "Iniciar sesión con navegador").Layout(gtx)
													}),
												)
											case "none":
												return material.Caption(th, "Este modo no requiere credenciales externas.").Layout(gtx)
											default:
												return material.Caption(th, fmt.Sprintf("Módulo autenticado de forma pasiva mediante: %s", state.selectedAuthMode)).Layout(gtx)
											}
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

										// Models List Selection
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return material.Overline(th, "MODELO ACTIVO (model_id)").Layout(gtx)
										}),
										layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											children := make([]layout.FlexChild, 0, len(models)*2)
											for i, modelID := range models {
												if i > 0 {
													children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
												}
												modelID := modelID
												lbl := modelID
												if state.selectedModel == modelID {
													lbl = "✓ " + modelID
												}
												btn, ok := state.modelClicks[modelID]
												if !ok {
													btn = new(widget.Clickable)
													state.modelClicks[modelID] = btn
												}
												children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return material.Button(th, btn, lbl).Layout(gtx)
												}))
											}
											return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
										}),
									)
								})
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),

					// Footer with status messaging and main actions
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return material.Caption(th, "Estado: "+state.snapshot()).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(th, &state.testBtn, "⟲ Probar Conexión").Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(th, &state.saveBtn, "➜ Guardar y Aplicar").Layout(gtx)
							}),
						)
					}),
				)
			})

			e.Frame(gtx.Ops)
		}
	}
}

func colorBlock(gtx layout.Context, c color.NRGBA, size image.Point) layout.Dimensions {
	shape := clip.UniformRRect(image.Rectangle{Max: size}, 3).Op(gtx.Ops)
	paint.FillShape(gtx.Ops, c, shape)
	return layout.Dimensions{Size: size}
}

func neonStatCard(gtx layout.Context, th *material.Theme, title, value string, accent color.NRGBA) layout.Dimensions {
	// Restrict box constraints to render a nice compact tag
	gtx.Constraints.Max.X = gtx.Dp(240)
	gtx.Constraints.Max.Y = gtx.Dp(52)
	cardRect := image.Rectangle{Max: gtx.Constraints.Max}
	stack := clip.UniformRRect(cardRect, 6).Push(gtx.Ops)
	paint.Fill(gtx.Ops, color.NRGBA{R: 0x12, G: 0x1A, B: 0x35, A: 0xFF})
	stack.Pop()
	paint.FillShape(gtx.Ops, accent, clip.UniformRRect(image.Rect(0, 0, 4, cardRect.Dy()), 2).Op(gtx.Ops))
	return layout.Inset{Top: unit.Dp(6), Left: unit.Dp(10), Right: unit.Dp(10), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return material.Caption(th, title).Layout(gtx) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return material.Body2(th, value).Layout(gtx) }),
		)
	})
}

func resolveConfigPath() string {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		return "config/rbot.yaml"
	}
	return configPath
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

func accentColor(phase float64) color.NRGBA {
	p := math.Mod(phase, 1)
	r := uint8(10 + 40*math.Abs(math.Sin((p+0.0)*math.Pi*2)))
	g := uint8(180 + 75*math.Abs(math.Sin((p+0.33)*math.Pi*2)))
	b := uint8(220 + 35*math.Abs(math.Sin((p+0.66)*math.Pi*2)))
	return color.NRGBA{R: r, G: g, B: b, A: 0xFF}
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

func (s *uiState) phaseValue() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
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

func liveBadge(gtx layout.Context, th *material.Theme, phase float64) layout.Dimensions {
	accent := accentColor(phase + 0.2)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return colorBlock(gtx, accent, image.Pt(8, 8)) }),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Caption(th, "SYSTEM OPERATIONAL / DECK").Layout(gtx)
		}),
	)
}

func getModelsForProvider(provider string, providersConf *config.ProvidersConfig) []string {
	var list []string
	if providersConf != nil {
		if entry, ok := providersConf.Providers[provider]; ok && len(entry.Models) > 0 {
			for mID := range entry.Models {
				list = append(list, mID)
			}
			sort.Strings(list)
			return list
		}
	}
	switch provider {
	case "ollama":
		return []string{"qwen2.5:7b", "qwen2.5-coder:7b"}
	case "openai":
		return []string{"gpt-4o-mini", "gpt-4o"}
	case "anthropic":
		return []string{"claude-3-5-sonnet-latest"}
	case "google_gemini":
		return []string{"gemini-2.5-flash"}
	case "openrouter":
		return []string{"openrouter/free", "google/gemini-2.5-flash"}
	case "deepseek":
		return []string{"deepseek-chat"}
	default:
		return []string{"default"}
	}
}

func getAuthModesForProvider(provider string, providersConf *config.ProvidersConfig) []string {
	var list []string
	if providersConf != nil {
		if entry, ok := providersConf.Providers[provider]; ok && len(entry.Capabilities) > 0 {
			seen := make(map[string]bool)
			for _, cap := range entry.Capabilities {
				mode := cap.AuthMode
				if mode != "" && !seen[mode] {
					seen[mode] = true
					list = append(list, mode)
				}
			}
			if len(list) > 0 {
				return list
			}
		}
	}
	switch provider {
	case "ollama":
		return []string{"none"}
	case "openai", "anthropic":
		return []string{"browser_oauth", "api_key"}
	case "google_gemini":
		return []string{"browser_oauth", "api_key", "adc", "service_account"}
	case "openrouter", "deepseek":
		return []string{"api_key"}
	default:
		return []string{"api_key"}
	}
}

func getBillingAndRuntimeMode(provider, authMode string, providersConf *config.ProvidersConfig) (string, string) {
	if providersConf != nil {
		if entry, ok := providersConf.Providers[provider]; ok {
			for _, cap := range entry.Capabilities {
				if cap.AuthMode == authMode {
					return cap.BillingMode, cap.RuntimeMode
				}
			}
		}
	}
	if provider == "ollama" {
		return "local", "local_runtime"
	}
	switch authMode {
	case "browser_oauth":
		return "subscription", "official_cli_session"
	case "api_key":
		if provider == "openrouter" {
			return "credits", "gateway_api"
		}
		return "pay_as_you_go", "direct_api"
	case "adc", "service_account":
		return "cloud_project", "direct_api"
	default:
		return "none", "direct_api"
	}
}