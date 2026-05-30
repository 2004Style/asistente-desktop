package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
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
	errText string

	// Selected states
	selectedProvider string
	selectedAuthMode string
	selectedModel    string

	// Clicks mapping for dynamic immediate mode rendering
	providerClicks map[string]*widget.Clickable
	authModeClicks map[string]*widget.Clickable
	modelClicks    map[string]*widget.Clickable

	// Scrollable lists
	providerList layout.List
	modelList    layout.List

	// Form controllers
	apiKeyEditor      widget.Editor
	modelSearchEditor widget.Editor

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
	if os.Getenv("DRI_PRIME") == "0" {
		execPath, err := os.Executable()
		if err == nil {
			os.Unsetenv("DRI_PRIME")
			cmd := exec.Command(execPath, os.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			env := os.Environ()
			var newEnv []string
			for _, e := range env {
				if !strings.HasPrefix(e, "DRI_PRIME=") {
					newEnv = append(newEnv, e)
				}
			}
			cmd.Env = newEnv
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("RBot Settings Panel"),
			app.Size(unit.Dp(1000), unit.Dp(700)),
			app.Decorated(true),
		)
		if err := loop(w); err != nil {
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
	state.modelSearchEditor.SingleLine = true
	state.providerList.Axis = layout.Vertical
	state.modelList.Axis = layout.Vertical

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
	th.Palette.Bg = color.NRGBA{R: 0x01, G: 0x02, B: 0x04, A: 0xFF} // var(--bg)
	th.Palette.Fg = color.NRGBA{R: 0xEA, G: 0xFF, B: 0xFF, A: 0xFF} // var(--text)
	th.Palette.ContrastBg = color.NRGBA{R: 0x00, G: 0xF5, B: 0xFF, A: 0xFF} // var(--cyan)
	th.Palette.ContrastFg = color.NRGBA{R: 0x02, G: 0x05, B: 0x0B, A: 0xFF}
	th.TextSize = unit.Sp(14)

	state := initUIState()
	var ops op.Ops

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			log.Println("DEBUG: FrameEvent received")
			gtx := app.NewContext(&ops, e)
			paint.Fill(gtx.Ops, th.Palette.Bg)

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
							if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok {
								finalSecretRef = entry.SecretRef
							}
						} else if strings.HasPrefix(keyInput, "[Variable de entorno:") {
							if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok {
								finalSecretRef = entry.SecretRef
							}
						} else {
							if strings.HasPrefix(keyInput, "env:") || strings.HasPrefix(keyInput, "plain:") || strings.HasPrefix(keyInput, "keyring:") {
								finalSecretRef = keyInput
							} else {
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

					if state.selectedAuthMode == "browser_oauth" && finalSecretRef == "" {
						finalSecretRef = "keyring:" + state.selectedProvider + "_session"
					}

					entry, ok := state.providersConf.Providers[state.selectedProvider]
					if !ok {
						entry = config.ProviderEntry{Enabled: true}
					}
					entry.Enabled = true
					entry.AuthMode = state.selectedAuthMode
					entry.SecretRef = finalSecretRef
					entry.DefaultModel = state.selectedModel
					entry.Model = state.selectedModel

					billingMode, runtimeMode := getBillingAndRuntimeMode(state.selectedProvider, state.selectedAuthMode, state.providersConf)
					entry.BillingMode = billingMode
					entry.RuntimeMode = runtimeMode

					state.providersConf.Providers[state.selectedProvider] = entry

					state.providersConf.ActiveProfile = ""
					state.providersConf.ActiveProvider = state.selectedProvider
					state.providersConf.ActiveModel = state.selectedModel
					state.providersConf.ActiveAuthMode = state.selectedAuthMode
					state.providersConf.Active = config.ActiveConfig{
						Provider:    state.selectedProvider,
						Model:       state.selectedModel,
						AuthProfile: "",
					}

					provPath := resolveProvidersPath(state.configPath, state.conf.Providers.ConfigFile)
					err := config.SaveProvidersConfig(provPath, state.providersConf)
					if err != nil {
						state.setError(fmt.Sprintf("Error guardando proveedores: %v", err))
						w.Invalidate()
						return
					}

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

			// RENDER VISUAL LAYOUT
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// Header / Topbar
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.Y = gtx.Dp(64)
					gtx.Constraints.Min.Y = gtx.Dp(64)
					return drawTopbar(gtx, th)
				}),
				// Shell Columns
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							// Column 1: Providers Sidebar
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Max.X = gtx.Dp(240)
								gtx.Constraints.Min.X = gtx.Dp(240)
								return drawLeftColumn(gtx, th, state)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							// Column 2: Credentials & Model lists
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return drawCenterColumn(gtx, th, state)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							// Column 3: Summary, Metrics, Actions
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Max.X = gtx.Dp(280)
								gtx.Constraints.Min.X = gtx.Dp(280)
								return drawRightColumn(gtx, th, state)
							}),
						)
					})
				}),
			)

			log.Println("DEBUG: Calling e.Frame")
			e.Frame(gtx.Ops)
			log.Println("DEBUG: e.Frame returned")
		}
	}
}

// drawTopbar dibuja la barra superior del HUD de configuración de RBot.
// Esta sección representa la cabecera visual y la identidad de la aplicación.
// Contiene:
// - La sección izquierda con el logo de marca "AI" y los títulos "Ajustes Rbot".
// - La sección derecha con el indicador de estado de conexión al Daemon ("CONECTADO A DAEMON")
//   y controles simulados de ventana (minimizar, maximizar y cerrar).
func drawTopbar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	log.Println("DEBUG: drawTopbar start")
	defer log.Println("DEBUG: drawTopbar end")
	cardRect := image.Rectangle{Max: gtx.Constraints.Max}
	
	// Dibuja el fondo sólido de color gris muy oscuro (R:5, G:5, B:5) para toda la barra superior.
	paint.FillShape(gtx.Ops, color.NRGBA{R: 5, G: 5, B: 5, A: 255}, clip.Rect(cardRect).Op())

	// Aplica un margen interno uniforme de 14 Dp para todo el contenido de la barra.
	return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Alinea horizontalmente los elementos, distribuyéndolos de extremo a extremo (SpaceBetween).
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			// Sección izquierda: Logo "AI" y Títulos principal y secundario.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// Dibuja el logotipo decorativo de la marca (Brand Logo).
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawBrandLogo(gtx, th)
					}),
					// Espaciador horizontal de 14 Dp entre el logo y los textos.
					layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
					// Contenedor vertical para apilar los títulos de la cabecera.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							// Título principal: "Ajustes Rbot" en color blanco brillante.
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(th, unit.Sp(16), "Ajustes Rbot")
								lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 255}
								return lbl.Layout(gtx)
							}),
							// Subtítulo secundario informativo.
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Label(th, unit.Sp(9), "RBot Settings · Provider Control HUD")
								lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
								return lbl.Layout(gtx)
							}),
						)
					}),
				)
			}),
			// Sección derecha: Indicador de estado de conexión y botones simulados de ventana.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// Dibuja el punto verde y el texto "CONECTADO A DAEMON".
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawTopStatus(gtx, th)
					}),
					// Espaciador horizontal de 18 Dp antes de la botonera.
					layout.Rigid(layout.Spacer{Width: unit.Dp(18)}.Layout),
					// Contenedor para los tres botones simulados de control de ventana.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							// Botón Minimizar "—"
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return drawWindowBtn(gtx, th, "—", false)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							// Botón Maximizar "□"
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return drawWindowBtn(gtx, th, "□", false)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							// Botón Cerrar "×" (en color rojo)
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return drawWindowBtn(gtx, th, "×", true)
							}),
						)
					}),
				)
			}),
		)
	})
}

// drawBrandLogo dibuja la caja blanca redondeada del logotipo de la marca.
// Ocupa un área cuadrada de 36x36 Dp y muestra el texto "AI" centrado.
func drawBrandLogo(gtx layout.Context, th *material.Theme) layout.Dimensions {
	size := gtx.Dp(36)
	// Define la forma redondeada del contenedor con radio 6px.
	shape := safeRRect(image.Rectangle{Max: image.Pt(size, size)}, 6).Op(gtx.Ops)
	// Rellena la forma con blanco sólido.
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, shape)
	// Centra el texto "AI" en el medio de la caja.
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th, unit.Sp(15), "AI")
		// Texto en azul oscuro/casi negro para crear un fuerte contraste sobre el fondo blanco.
		lbl.Color = color.NRGBA{R: 0x00, G: 0x10, B: 0x18, A: 0xFF}
		return lbl.Layout(gtx)
	})
}

// drawTopStatus dibuja el indicador de conexión al Daemon en la barra superior.
// Consta de un cuadrado verde brillante (neón) y la etiqueta de texto descriptiva.
func drawTopStatus(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		// Punto/Caja verde de estado.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Dp(9)
			shape := safeRRect(image.Rectangle{Max: image.Pt(size, size)}, 2).Op(gtx.Ops)
			// Rellena el punto con verde brillante (R:0, G:255, B:157)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 0xFF}, shape)
			return layout.Dimensions{Size: image.Pt(size, size)}
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		// Texto indicador "CONECTADO A DAEMON".
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th, unit.Sp(11), "CONECTADO A DAEMON")
			lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 204}
			return lbl.Layout(gtx)
		}),
	)
}

// drawWindowBtn dibuja un botón simulado de control de ventana de 24x24 Dp.
// Se dibuja como una caja translúcida gris/blanca muy tenue con el símbolo centrado.
func drawWindowBtn(gtx layout.Context, th *material.Theme, char string, isClose bool) layout.Dimensions {
	size := gtx.Dp(24)
	shape := safeRRect(image.Rectangle{Max: image.Pt(size, size)}, 4).Op(gtx.Ops)
	// Pinta el fondo de la caja del botón con una opacidad muy baja (A: 14).
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, shape)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th, unit.Sp(12), char)
		if isClose {
			// Si representa el botón "Cerrar", el símbolo se dibuja de color coral/rojo.
			lbl.Color = color.NRGBA{R: 255, G: 90, B: 120, A: 230}
		} else {
			// Para los demás botones, se dibuja de color cian/blanco opaco.
			lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
		}
		return lbl.Layout(gtx)
	})
}

// drawPanel dibuja un panel contenedor oscuro con bordes redondeados y estilo glassmorphism.
// Se compone de:
// - Capa inferior (Expanded): Un fondo gris translúcido esmerilado con un borde delgado de 1px.
// - Capa superior (Stacked): Título de la sección en mayúsculas pequeñas y el widget de contenido interno.
func drawPanel(gtx layout.Context, th *material.Theme, title string, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		// Fondo del panel y borde exterior fino (Efecto Glass/Translúcido).
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			cardRect := image.Rectangle{Max: gtx.Constraints.Min}
			shape := safeRRect(cardRect, 6).Op(gtx.Ops)
			// Pinta el relleno esmerilado (R:66, G:66, B:66, A:60)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 66, G: 66, B: 66, A: 60}, shape)
			// Dibuja la línea de contorno blanca muy fina (1px con A:23)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 23}, clip.Stroke{
				Path:  safeRRect(cardRect, 6).Path(gtx.Ops),
				Width: 1,
			}.Op())
			return layout.Dimensions{Size: cardRect.Max}
		}),
		// Contenido del panel con un margen interno (Inset) de 14 Dp.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Título del panel en mayúsculas.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(10), strings.ToUpper(title))
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
							return lbl.Layout(gtx)
						})
					}),
					// Widget principal con el contenido dinámico del panel.
					layout.Rigid(content),
				)
			})
		}),
	)
}

// drawLeftColumn dibuja el panel lateral izquierdo que corresponde a la Columna 1.
// Muestra el título "PROVEEDORES" y renderiza una lista vertical scrollable
// de todos los proveedores de LLM disponibles (Ollama, OpenAI, Anthropic, etc.).
func drawLeftColumn(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	log.Println("DEBUG: drawLeftColumn start")
	defer log.Println("DEBUG: drawLeftColumn end")
	return drawPanel(gtx, th, "Proveedores", func(gtx layout.Context) layout.Dimensions {
		providers := []string{"ollama", "openai", "anthropic", "google_gemini", "openrouter", "deepseek"}
		gtx.Constraints.Min.Y = 0
		if gtx.Constraints.Max.Y < 0 {
			gtx.Constraints.Max.Y = 0
		}
		// Renderiza cada elemento de la lista de proveedores con un margen inferior de 8 Dp.
		return state.providerList.Layout(gtx, len(providers), func(gtx layout.Context, i int) layout.Dimensions {
			name := providers[i]
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawProviderItem(gtx, th, state, name)
			})
		})
	})
}

// drawProviderItem dibuja el botón/tarjeta de cada proveedor individual en la Columna 1.
// Cada tarjeta mide 52 Dp de altura y contiene:
// - El nombre del proveedor (ej. "Ollama", "Openai") con inicial mayúscula.
// - El rol/categoría del proveedor (ej. "local_runtime", "native_provider").
// - Un icono de verificación (check box) que se ilumina en verde cuando está seleccionado.
// Si el proveedor está seleccionado (isActive), se resalta con fondo y contorno cian brillante.
func drawProviderItem(gtx layout.Context, th *material.Theme, state *uiState, name string) layout.Dimensions {
	isActive := state.selectedProvider == name
	btn := state.providerClicks[name]

	// Envuelve todo el elemento en un componente interactivo/clickable.
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		cardRect := image.Rectangle{Max: gtx.Constraints.Max}
		cardRect.Max.Y = gtx.Dp(52) // Forzar altura fija a la tarjeta
		shape := safeRRect(cardRect, 4).Op(gtx.Ops)
		
		// Color de fondo y borde basado en si el elemento está activo o inactivo.
		if isActive {
			// Fondo cian esmerilado y borde cian brillante.
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 245, B: 255, A: 25}, shape)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 245, B: 255, A: 180}, clip.Stroke{
				Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
				Width: 1,
			}.Op())
		} else {
			// Fondo gris oscuro sutil y borde translúcido apagado.
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 10}, shape)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, clip.Stroke{
				Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
				Width: 1,
			}.Op())
		}

		// Espaciado interior de 10 Dp alrededor del contenido de la tarjeta.
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Distribuye el contenido en forma horizontal: los textos a la izquierda (Flexed) y el check a la derecha (Rigid).
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Textos de descripción del proveedor.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Nombre del proveedor (ej: "Openai" o "Anthropic").
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(12), strings.Title(name))
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 224}
							return lbl.Layout(gtx)
						}),
						// Categoría de integración del proveedor.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							category := "native_provider"
							if name == "openrouter" {
								category = "model_gateway"
							} else if name == "ollama" {
								category = "local_runtime"
							}
							lbl := material.Label(th, unit.Sp(9), category)
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
							return lbl.Layout(gtx)
						}),
					)
				}),
				// Casilla de verificación de selección (Check Box) a la derecha.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawCheck(gtx, th, isActive)
				}),
			)
		})
	})
}

// drawCheck dibuja un pequeño recuadro de verificación (22x22 Dp) para indicar estados activos.
// Si está activo (active = true), se pinta de color verde neón (R:0, G:255, B:157) para resaltar la selección.
// Si está inactivo, se muestra vacío con un fondo translúcido muy apagado.
func drawCheck(gtx layout.Context, th *material.Theme, active bool) layout.Dimensions {
	size := gtx.Dp(22)
	shape := safeRRect(image.Rectangle{Max: image.Pt(size, size)}, 4).Op(gtx.Ops)
	if active {
		// Caja verde brillante para estado activo.
		paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 0xFF}, shape)
		return layout.Dimensions{Size: image.Pt(size, size)}
	} else {
		// Caja vacía apagada para elementos no seleccionados.
		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 10}, shape)
		return layout.Dimensions{Size: image.Pt(size, size)}
	}
}

// drawCenterColumn dibuja el panel de la columna del medio (Columna 2).
// Apila verticalmente dos paneles esenciales de configuración:
// 1. Panel Superior: Ajustes de Autenticación y Credenciales (API Key o OAuth por navegador).
// 2. Panel Inferior: Listado de Modelos Disponibles del proveedor seleccionado.
func drawCenterColumn(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	log.Println("DEBUG: drawCenterColumn start")
	defer log.Println("DEBUG: drawCenterColumn end")
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Panel superior de Autenticación y métodos de credenciales.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawPanel(gtx, th, "Autenticación y credenciales", func(gtx layout.Context) layout.Dimensions {
				return drawAuthSection(gtx, th, state)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Panel inferior de Modelos disponibles.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return drawPanel(gtx, th, "Modelos disponibles", func(gtx layout.Context) layout.Dimensions {
				return drawModelsSection(gtx, th, state)
			})
		}),
	)
}

// drawAuthSection renderiza los detalles del panel de credenciales de la Columna 2.
// Consiste en:
// - Un selector horizontal de modos de autenticación (ej: API Key, Iniciar sesión, Sin auth).
// - Un contenedor inferior dinámico que muestra diferentes controles según el modo activo:
//   - "api_key": Un campo de entrada de texto (Editor) para ingresar la API Key
//     e indicador inferior del llavero de contraseñas.
//   - "browser_oauth": Un botón para lanzar el navegador ("ABRIR NAVEGADOR")
//     y el indicador del llavero de la sesión.
//   - "none": Mensaje informativo para proveedores locales (como Ollama) que no requieren clave.
func drawAuthSection(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	authModes := getAuthModesForProvider(state.selectedProvider, state.providersConf)

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Selector horizontal del modo de autenticación.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(authModes)*2)
			for i, mode := range authModes {
				if i > 0 {
					// Espacio horizontal entre los botones selectores.
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
				}
				mode := mode
				btn := state.authModeClicks[mode]
				isActive := state.selectedAuthMode == mode

				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := mode
					// Traduce el identificador del modo a texto amigable para la interfaz.
					if mode == "browser_oauth" {
						lbl = "Iniciar sesión"
					} else if mode == "api_key" {
						lbl = "API Key"
					} else if mode == "none" {
						lbl = "Sin autenticación"
					} else if mode == "adc" {
						lbl = "Google Cloud"
					} else if mode == "service_account" {
						lbl = "Cuenta servicio"
					}

					// Botón de opción selector.
					return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
						cardRect := image.Rectangle{Max: gtx.Constraints.Max}
						cardRect.Max.Y = gtx.Dp(28)
						cardRect.Max.X = gtx.Dp(115)
						shape := safeRRect(cardRect, 4).Op(gtx.Ops)
						
						// Si está seleccionado, cambia a verde brillante, de lo contrario se dibuja translúcido apagado.
						if isActive {
							paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 255}, shape)
						} else {
							paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 11}, shape)
							paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, clip.Stroke{
								Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
								Width: 1,
							}.Op())
						}

						// Centra el texto del botón selector.
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lblVal := material.Label(th, unit.Sp(10), strings.ToUpper(lbl))
							if isActive {
								// Texto negro sobre fondo verde neón.
								lblVal.Color = color.NRGBA{R: 0x02, G: 0x05, B: 0x0B, A: 0xFF}
							} else {
								// Texto apagado sobre botón apagado.
								lblVal.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
							}
							return lblVal.Layout(gtx)
						})
					})
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		// Renderizado dinámico del campo de entrada basado en el modo seleccionado.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch state.selectedAuthMode {
			case "api_key":
				// Entrada para la API Key
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Caja de entrada de texto (Editor)
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cardRect := image.Rectangle{Max: gtx.Constraints.Max}
						cardRect.Max.Y = gtx.Dp(38)
						shape := safeRRect(cardRect, 4).Op(gtx.Ops)
						paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 66}, shape)
						paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, clip.Stroke{
							Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
							Width: 1,
						}.Op())

						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Editor(th, &state.apiKeyEditor, "Clave API (se guardará en Llavero de forma segura)").Layout(gtx)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					// Etiqueta indicadora inferior de la API Key configurada actualmente.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								cardRect := image.Rectangle{Max: gtx.Constraints.Max}
								cardRect.Max.Y = gtx.Dp(32)
								shape := safeRRect(cardRect, 4).Op(gtx.Ops)
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 60}, shape)
								return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Label(th, unit.Sp(10), "SecretRef: "+state.apiKeyEditor.Text())
									lbl.Color = color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 0xFF}
									return lbl.Layout(gtx)
								})
							}),
						)
					}),
				)
			case "browser_oauth":
				// Flujo de autenticación OAuth 2.0 por Navegador Web.
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							// Etiqueta indicadora del llavero del sistema de la sesión.
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								cardRect := image.Rectangle{Max: gtx.Constraints.Max}
								cardRect.Max.Y = gtx.Dp(32)
								shape := safeRRect(cardRect, 4).Op(gtx.Ops)
								paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 60}, shape)
								return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									lbl := material.Label(th, unit.Sp(10), "SessionRef: keyring:"+state.selectedProvider+"_session")
									lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 210}
									return lbl.Layout(gtx)
								})
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							// Botón "ABRIR NAVEGADOR" para lanzar el navegador web.
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(th, &state.loginBtn, "ABRIR NAVEGADOR").Layout(gtx)
							}),
						)
					}),
				)
			case "none":
				// Flujo de Proveedor Local (sin autenticación).
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						cardRect := image.Rectangle{Max: gtx.Constraints.Max}
						cardRect.Max.Y = gtx.Dp(32)
						shape := safeRRect(cardRect, 4).Op(gtx.Ops)
						paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 60}, shape)
						return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(10), "Ollama activo en base_url: http://localhost:11434")
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 210}
							return lbl.Layout(gtx)
						})
					}),
				)
			default:
				// Fallback para métodos raros o externos.
				return material.Caption(th, "Este método está gestionado externamente.").Layout(gtx)
			}
		}),
	)
}

// drawModelsSection dibuja la sección de selección de modelos dentro de la Columna 2.
// Se compone de:
// - Fila superior: Un campo de entrada de texto (Editor) para escribir filtros de búsqueda (e.g. "gpt-4"),
//   y al lado una caja que actúa como contador de modelos filtrados (ej: "2/6").
// - Lista vertical: Un contenedor con límite de altura fija (260 Dp) con scroll vertical
//   que renderiza las tarjetas de los modelos disponibles (filtrados por la consulta).
func drawModelsSection(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	models := getModelsForProvider(state.selectedProvider, state.providersConf)
	query := strings.ToLower(strings.TrimSpace(state.modelSearchEditor.Text()))

	// Filtra los modelos según el texto de búsqueda ingresado en el input.
	filteredModels := []string{}
	for _, mID := range models {
		if query == "" || strings.Contains(strings.ToLower(mID), query) {
			filteredModels = append(filteredModels, mID)
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Fila superior: Caja de búsqueda y contador.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Campo de búsqueda de modelos (Editor).
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					h := gtx.Dp(38)
					gtx.Constraints.Min.Y = h
					gtx.Constraints.Max.Y = h
					cardRect := image.Rectangle{Max: gtx.Constraints.Max}
					shape := safeRRect(cardRect, 4).Op(gtx.Ops)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 66}, shape)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, clip.Stroke{
						Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
						Width: 1,
					}.Op())

					// Centra verticalmente el editor dentro de la tarjeta de 38 Dp.
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Editor(th, &state.modelSearchEditor, "Filtrar modelo...").Layout(gtx)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				// Caja del contador de modelos (ej: "2/4") de tamaño 80x38 Dp.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					w := gtx.Dp(80)
					h := gtx.Dp(38)
					gtx.Constraints.Min = image.Pt(w, h)
					gtx.Constraints.Max = image.Pt(w, h)
					cardRect := image.Rectangle{Max: gtx.Constraints.Max}
					shape := safeRRect(cardRect, 4).Op(gtx.Ops)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 56}, shape)
					paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 13}, clip.Stroke{
						Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
						Width: 1,
					}.Op())

					// Centra el texto del contador de modelos.
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(10), fmt.Sprintf("%d/%d", len(filteredModels), len(models)))
						lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
						return lbl.Layout(gtx)
					})
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		// Listado vertical de tarjetas de modelos (con altura máxima de 260 Dp).
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = 0
			gtx.Constraints.Max.Y = gtx.Dp(260)
			return state.modelList.Layout(gtx, len(filteredModels), func(gtx layout.Context, i int) layout.Dimensions {
				mID := filteredModels[i]
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawModelItem(gtx, th, state, mID)
				})
			})
		}),
	)
}

// drawModelItem dibuja la tarjeta interactiva de un modelo en la lista de la Columna 2.
// Cada tarjeta de modelo mide 46 Dp de altura y contiene:
// - Nombre/ID del modelo (ej: "gpt-4o-mini" o "claude-3-5-sonnet-latest").
// - Metadatos descriptivos pequeños (ej. "Rápido · Económico").
// - La casilla de verificación (Check mark) que se ilumina cuando el modelo está seleccionado.
// Si está activo (isActive), se pinta con color de fondo y borde verde/cian brillante.
func drawModelItem(gtx layout.Context, th *material.Theme, state *uiState, mID string) layout.Dimensions {
	isActive := state.selectedModel == mID
	btn, ok := state.modelClicks[mID]
	if !ok {
		btn = new(widget.Clickable)
		state.modelClicks[mID] = btn
	}

	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		if w <= 0 {
			w = gtx.Dp(260)
		}
		cardRect := image.Rectangle{Max: image.Pt(w, gtx.Dp(46))}
		shape := safeRRect(cardRect, 4).Op(gtx.Ops)
		
		if isActive {
			// Resaltado de modelo seleccionado (Verde neón esmerilado).
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 255, B: 157, A: 20}, shape)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 245, B: 255, A: 120}, clip.Stroke{
				Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
				Width: 1,
			}.Op())
		} else {
			// Fondo inactivo gris/negro.
			paint.FillShape(gtx.Ops, color.NRGBA{R: 15, G: 15, B: 15, A: 170}, shape)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 14}, clip.Stroke{
				Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
				Width: 1,
			}.Op())
		}

		// Espaciado interno de 8 Dp.
		return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// Divide en horizontal: los textos del modelo a la izquierda y el check de estado a la derecha.
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// Textos del modelo.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Nombre del modelo en tamaño 12 Sp.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(12), mID)
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 224}
							return lbl.Layout(gtx)
						}),
						// Metadatos explicativos en tamaño 9 Sp (ej: "Local · General" o "Multimodal").
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							meta := getModelMetaDescription(state.selectedProvider, mID)
							lbl := material.Label(th, unit.Sp(9), meta)
							lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
							return lbl.Layout(gtx)
						}),
					)
				}),
				// Casilla de verificación de selección (Check Box).
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawCheck(gtx, th, isActive)
				}),
			)
		})
	})
}

// getModelMetaDescription retorna una descripción corta y estilizada de las capacidades del modelo
// para enriquecer visualmente las tarjetas en el listado de modelos.
func getModelMetaDescription(provider, mID string) string {
	switch provider {
	case "ollama":
		return "Local · General"
	case "openrouter":
		if strings.Contains(mID, "free") {
			return "Gratis · Enrutado automático"
		}
		return "Pago · Enrutamiento compatible"
	case "openai":
		if strings.Contains(mID, "mini") {
			return "Rápido · Económico"
		}
		return "Multimodal · Avanzado"
	case "anthropic":
		return "Preciso · Computación y código"
	case "google_gemini":
		return "Multimodal · Respuesta ultra rápida"
	default:
		return "Compatible · Proveedor de IA"
	}
}

// drawRightColumn dibuja el panel lateral derecho correspondiente a la Columna 3.
// Apila verticalmente tres paneles principales que cierran el flujo de ajustes:
// 1. Panel Superior: "CONFIGURACION ACTIVA" (Resumen de las opciones seleccionadas).
// 2. Panel Central: "MÉTRICAS" (Indicadores de facturación y entorno del proveedor).
// 3. Panel Inferior: "ACCIONES" (Lanzador de conexión TEST, botón APLICAR cambios e indicadores de éxito).
func drawRightColumn(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	log.Println("DEBUG: drawRightColumn start")
	defer log.Println("DEBUG: drawRightColumn end")
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Panel 1: Configuración Activa (Resumen).
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawPanel(gtx, th, "Configuracion activa", func(gtx layout.Context) layout.Dimensions {
				return drawSummarySection(gtx, th, state)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Panel 2: Métricas de Facturación y Entorno de Ejecución.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawPanel(gtx, th, "Métricas", func(gtx layout.Context) layout.Dimensions {
				return drawMetricsSection(gtx, th, state)
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Panel 3: Acciones e Indicadores del Daemon de fondo.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawPanel(gtx, th, "Acciones", func(gtx layout.Context) layout.Dimensions {
				return drawActionsSection(gtx, th, state)
			})
		}),
	)
}

// drawSummarySection renderiza las tres líneas de resumen dentro del Panel de Configuración Activa.
// Dibuja consecutivamente:
// - Fila 1: Proveedor seleccionado (ej. "ollama", "openai").
// - Fila 2: Modelo activo (ej. "gpt-4o-mini").
// - Fila 3: Método de Autenticación actual (ej. "browser_oauth", "api_key").
func drawSummarySection(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Fila de resumen del Proveedor.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			category := "native_provider"
			if state.selectedProvider == "openrouter" {
				category = "model_gateway"
			} else if state.selectedProvider == "ollama" {
				category = "local_runtime"
			}
			return drawSummaryLine(gtx, th, "P", "Proveedor", state.selectedProvider, category)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Fila de resumen del Modelo.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			meta := getModelMetaDescription(state.selectedProvider, state.selectedModel)
			return drawSummaryLine(gtx, th, "M", "Modelo", state.selectedModel, meta)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Fila de resumen de la Autenticación.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			secretPath := "none"
			if entry, ok := state.providersConf.Providers[state.selectedProvider]; ok && entry.SecretRef != "" {
				secretPath = entry.SecretRef
			}
			return drawSummaryLine(gtx, th, "A", "Autenticación", state.selectedAuthMode, secretPath)
		}),
	)
}

// drawSummaryLine dibuja una tarjeta de fila de datos del panel de resumen (altura fija 46 Dp).
// Ocupa todo el ancho del panel y muestra:
// - El título en mayúsculas de la categoría (ej. "PROVEEDOR").
// - El valor configurado (ej. "openai").
// - Un texto secundario explicativo o ruta del secreto (ej. "keyring:openai_api_key").
func drawSummaryLine(gtx layout.Context, th *material.Theme, icon, title, val, sub string) layout.Dimensions {
	cardRect := image.Rectangle{Max: gtx.Constraints.Max}
	cardRect.Max.Y = gtx.Dp(46)
	shape := safeRRect(cardRect, 4).Op(gtx.Ops)
	// Pinta fondo negro sutil y borde translúcido para estructurar el renglón.
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 56}, shape)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 13}, clip.Stroke{
		Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
		Width: 1,
	}.Op())

	return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Subtítulo pequeño de la propiedad (ej. "MODELO").
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(9), strings.ToUpper(title))
						lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
						return lbl.Layout(gtx)
					}),
					// Valor seleccionado (ej: "gpt-4o").
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(11), val)
						lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 224}
						return lbl.Layout(gtx)
					}),
					// Información adicional secundaria (ej. "Rápido · Económico").
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(8), sub)
						lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 100}
						return lbl.Layout(gtx)
					}),
				)
			}),
		)
	})
}

// drawMetricsSection dibuja el bloque de métricas dentro de la Columna 3.
// Obtiene el modo de facturación (Billing) y el modo de ejecución (Runtime) del proveedor,
// y renderiza dos tarjetas (Metric Cards) dispuestas horizontalmente lado a lado.
func drawMetricsSection(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	billing, runtime := getBillingAndRuntimeMode(state.selectedProvider, state.selectedAuthMode, state.providersConf)

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// Tarjeta de Facturación/Billing a la izquierda.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return drawMetricCard(gtx, th, "Billing", billing)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		// Tarjeta de Ejecución/Runtime a la derecha.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return drawMetricCard(gtx, th, "Runtime", runtime)
		}),
	)
}

// drawMetricCard dibuja una tarjeta de métrica individual (52 Dp de altura).
// Muestra el título superior (ej. "BILLING" o "RUNTIME")
// y el valor correspondiente en formato destacado (ej. "pay_as_you_go" o "local_runtime").
func drawMetricCard(gtx layout.Context, th *material.Theme, title, val string) layout.Dimensions {
	cardRect := image.Rectangle{Max: gtx.Constraints.Max}
	cardRect.Max.Y = gtx.Dp(52)
	shape := safeRRect(cardRect, 4).Op(gtx.Ops)
	// Fondo gris oscuro y contorno fino translúcido de la tarjeta.
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 56}, shape)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 13}, clip.Stroke{
		Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
		Width: 1,
	}.Op())

	return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Etiqueta superior del tipo de métrica.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(9), strings.ToUpper(title))
				lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			// Valor de la métrica (ej. "subscription" o "direct_api").
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(12), val)
				lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 224}
				return lbl.Layout(gtx)
			}),
		)
	})
}

// drawActionsSection dibuja la botonera final de control y estado de la Columna 3.
// Contiene:
// - Fila de botones:
//   - "TEST": Realiza un ping/llamado RPC al Daemon central para comprobar si responde.
//   - "APLICAR": Guarda los cambios locales de configuración en el disco y calienta la recarga en el Daemon.
// - Etiqueta intermedia: Muestra el estado actual del proceso ("Working...", "Ping: OK", etc.).
// - Caja inferior: Un indicador verde de estado final ("Listo · Ajustes de RBot cargados.").
func drawActionsSection(gtx layout.Context, th *material.Theme, state *uiState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Fila de botones de acción principales.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				// Botón de TEST de comunicación RPC.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &state.testBtn, func(gtx layout.Context) layout.Dimensions {
						cardRect := image.Rectangle{Max: gtx.Constraints.Max}
						cardRect.Max.Y = gtx.Dp(32)
						shape := safeRRect(cardRect, 4).Op(gtx.Ops)
						// Fondo gris translúcido con borde cian brillante.
						paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 11}, shape)
						paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 245, B: 255, A: 100}, clip.Stroke{
							Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
							Width: 1,
						}.Op())

						// Centra la etiqueta del botón "TEST".
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(11), "⟲ TEST")
							lbl.Color = color.NRGBA{R: 0x00, G: 0xF5, B: 0xFF, A: 0xFF}
							return lbl.Layout(gtx)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				// Botón "APLICAR" para almacenar la configuración.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &state.saveBtn, func(gtx layout.Context) layout.Dimensions {
						cardRect := image.Rectangle{Max: gtx.Constraints.Max}
						cardRect.Max.Y = gtx.Dp(32)
						shape := safeRRect(cardRect, 4).Op(gtx.Ops)
						// Relleno de color verde neón brillante.
						paint.FillShape(gtx.Ops, color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 255}, shape)

						// Centra el texto del botón "APLICAR".
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Label(th, unit.Sp(11), "✦ APLICAR")
							lbl.Color = color.NRGBA{R: 0x02, G: 0x05, B: 0x0B, A: 0xFF}
							return lbl.Layout(gtx)
						})
					})
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Texto dinámico para mostrar el estado de la última tarea asíncrona lanzada.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th, unit.Sp(10), state.snapshot())
			lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 148}
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Caja inferior estática con indicador verde de éxito / preparación.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cardRect := image.Rectangle{Max: gtx.Constraints.Max}
			cardRect.Max.Y = gtx.Dp(38)
			shape := safeRRect(cardRect, 4).Op(gtx.Ops)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 56}, shape)
			paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 255, B: 255, A: 13}, clip.Stroke{
				Path:  safeRRect(cardRect, 4).Path(gtx.Ops),
				Width: 1,
			}.Op())

			// Margen interno y distribución horizontal para el icono check verde y mensaje de cargado.
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					// Check "✓" en verde neón.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(12), "✓")
						lbl.Color = color.NRGBA{R: 0x00, G: 0xFF, B: 0x9D, A: 0xFF}
						return lbl.Layout(gtx)
					}),
					// Texto descriptivo.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Label(th, unit.Sp(10), " Listo · Ajustes de RBot cargados.")
						lbl.Color = color.NRGBA{R: 234, G: 255, B: 255, A: 200}
						return lbl.Layout(gtx)
					}),
				)
			})
		}),
	)
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

func colorBlock(gtx layout.Context, c color.NRGBA, size image.Point) layout.Dimensions {
	shape := safeRRect(image.Rectangle{Max: size}, 3).Op(gtx.Ops)
	paint.FillShape(gtx.Ops, c, shape)
	return layout.Dimensions{Size: size}
}

func safeRRect(rect image.Rectangle, radii int) clip.RRect {
	if rect.Max.X < rect.Min.X {
		rect.Max.X = rect.Min.X
	}
	if rect.Max.Y < rect.Min.Y {
		rect.Max.Y = rect.Min.Y
	}
	w := rect.Max.X - rect.Min.X
	h := rect.Max.Y - rect.Min.Y
	if w <= 0 || h <= 0 {
		return clip.UniformRRect(image.Rectangle{}, 0)
	}
	maxRadii := w / 2
	if h / 2 < maxRadii {
		maxRadii = h / 2
	}
	r := radii
	if r > maxRadii {
		r = maxRadii
	}
	if r < 0 {
		r = 0
	}
	return clip.UniformRRect(rect, r)
}