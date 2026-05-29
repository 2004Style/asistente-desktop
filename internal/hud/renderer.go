//go:build hud

package hud

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
)

type Particle struct {
	Angle float64
	Speed float64
	Dist  float64
	Size  float64
	Color [4]float64
}

type Renderer struct {
	window         *gtk.Window
	config         *config.Config
	mapper         *EventMapper
	currentUpdate  VisualUpdate
	pulse          float64
	rotations      float64
	particles      []Particle
	width          int
	height         int
	visible        bool
	client         *Client
	sysInfo        *SysInfo
	activeModel    string
	activeProvider string
	lastUserText   string
	lastAgentText  string
	tokensUsed     int
	costUSD        float64
	logoSurface    *cairo.Surface
}

func NewRenderer(conf *config.Config) *Renderer {
	w := conf.Hud.Window.Width
	h := conf.Hud.Window.Height
	if w <= 0 {
		w = 520
	}
	if h <= 0 {
		h = 540 // Altura recomendada para albergar el nuevo panel de rendimiento sin distorsión
	}

	logoSurf, err := cairo.NewSurfaceFromPNG("assets/logo.png")
	var logoSurface *cairo.Surface
	if err == nil {
		logoSurface = logoSurf
	} else {
		log.Printf("[HUD] No se pudo cargar el logo de assets/logo.png: %v", err)
	}

	r := &Renderer{
		config:         conf,
		mapper:         NewEventMapper(conf.Hud.Visual.AudioSmoothing),
		width:          w,
		height:         h,
		visible:        true, // Cambiar a true por defecto al arrancar
		particles:      make([]Particle, 40),
		sysInfo:        NewSysInfo(),
		activeModel:    conf.Providers.ActiveModel,
		activeProvider: conf.Providers.ActiveProvider,
		tokensUsed:     14850,
		costUSD:        10.05,
		logoSurface:    logoSurface,
	}

	if r.activeModel == "" {
		r.activeModel = conf.Model.Model
	}
	if r.activeProvider == "" {
		r.activeProvider = conf.Model.Provider
	}

	// Inicializar partículas
	for i := range r.particles {
		r.particles[i] = Particle{
			Angle: rand.Float64() * 2 * math.Pi,
			Speed: 0.02 + rand.Float64()*0.03,
			Dist:  55 + rand.Float64()*35,
			Size:  1.5 + rand.Float64()*2,
			Color: [4]float64{0.0, 0.8, 1.0, 0.3 + rand.Float64()*0.4},
		}
	}

	return r
}

func (r *Renderer) Start(ctx context.Context, socketPath string) {
	// 1. Inicializar canal de eventos y cliente
	r.client = NewClient(socketPath)
	r.client.Start(ctx)

	// Iniciar colector de estadísticas del sistema
	r.sysInfo.Start(ctx)

	// Goroutine periódica para consultar el modelo y proveedor activo mediante IPC
	go func() {
		time.Sleep(1 * time.Second)
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		socket := db.ExpandPath(r.config.Runtime.SocketPath)
		for {
			resp, err := ipc.SendCommandRPC(socket, "agent.status", nil, "hud-agent-status")
			if err == nil && resp.Result != nil {
				if m, ok := resp.Result.(map[string]interface{}); ok {
					model, _ := m["model"].(string)
					provider, _ := m["provider"].(string)
					glib.IdleAdd(func() bool {
						if model != "" {
							r.activeModel = model
						}
						if provider != "" {
							r.activeProvider = provider
						}
						return false
					})
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	// Canal para pasar eventos mapeados al bucle principal de UI
	updatesChan := make(chan VisualUpdate, 100)

	// Goroutine para leer eventos crudos y mapearlos a actualizaciones visuales
	go r.mapper.Process(ctx, r.client.Events(), updatesChan)

	// Goroutine que escucha actualizaciones visuales y programa cambios en el hilo de GTK
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case up, ok := <-updatesChan:
				if !ok {
					return
				}
				glib.IdleAdd(func() bool {
					r.handleVisualUpdate(up)
					return false
				})
			}
		}
	}()

	// 2. Inicializar GTK y crear ventana
	gtk.Init(nil)

	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatalf("Error creando la ventana HUD: %v", err)
	}
	r.window = win

	win.SetTitle("RBot HUD")
	win.SetDefaultSize(r.width, r.height)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Habilitar la captura de eventos de clic en la ventana usando la constante de C
	win.AddEvents(int(C.GDK_BUTTON_PRESS_MASK))

	// Borderless
	if r.config.Hud.Window.Borderless {
		win.SetDecorated(false)
	}

	// Always on top
	if r.config.Hud.Window.AlwaysOnTop {
		win.SetKeepAbove(true)
	}

	// No focus
	if r.config.Hud.Window.NoFocus {
		win.SetAcceptFocus(false)
		win.SetFocusOnMap(false)
	}

	// Habilitar RGBA para transparencias
	if r.config.Hud.Window.Transparent {
		screen := win.GetScreen()
		visual, err := screen.GetRGBAVisual()
		if err == nil && visual != nil && screen.IsComposited() {
			win.SetVisual(visual)
		}
		win.SetAppPaintable(true)
	}

	// Configurar Click-Through si está activo
	if r.config.Hud.Window.ClickThrough {
		win.Connect("realize", func() {
			gdkWin, err := win.GetWindow()
			if err == nil && gdkWin != nil {
				cRegion, err := cairo.RegionCreate()
				if err == nil && cRegion != nil {
					// Usar Cgo para establecer la región vacía
					C.gdk_window_input_shape_combine_region(
						(*C.GdkWindow)(unsafe.Pointer(gdkWin.Native())),
						(*C.cairo_region_t)(unsafe.Pointer(cRegion.Native())),
						0,
						0,
					)
				}
			}
		})
	}

	// Conectar al callback de dibujo (Draw)
	win.Connect("draw", func(w *gtk.Window, cr *cairo.Context) bool {
		r.drawHUD(w, cr)
		return true
	})

	// Capturar clics de ratón en el botón de Configuración y en el botón de Cerrar
	win.Connect("button-press-event", func(w *gtk.Window, event *gdk.Event) bool {
		eventButton := gdk.EventButtonNewFromEvent(event)
		if eventButton.Type() == gdk.EVENT_BUTTON_PRESS && eventButton.Button() == 1 { // Clic izquierdo
			xGo := eventButton.X()
			yGo := eventButton.Y()
			winW := float64(w.GetAllocatedWidth())

			// 1. Clic en botón cerrar de la topbar (X: winW - 35 .. winW - 13, Y: 12..28)
			if xGo >= (winW - 35.0) && xGo <= (winW - 13.0) && yGo >= 12.0 && yGo <= 28.0 {
				log.Println("[HUD] Cerrando la ventana del HUD por clic en botón Cerrar...")
				glib.IdleAdd(func() bool {
					w.Close()
					return false
				})
				return true
			}

			// 2. Clic en el botón de Configuración (alineación responsiva de columna derecha)
			paddingX := 16.0
			gap := 12.0
			leftW := 330.0
			rightW := 145.0

			leftX := paddingX
			rightX := winW - paddingX - rightW
			if rightX < leftX+leftW+gap {
				rightX = leftX + leftW + gap
			}

			topbarH := 40.0
			paddingY := 16.0
			hudY := topbarH + paddingY

			providerH := 240.0
			btnW := rightW - 16.0
			btnX := rightX + 8.0
			buttonH := 20.0
			btnY := hudY + providerH - buttonH - 8.0

			// Si el clic es dentro de los límites del botón de configuración
			if xGo >= btnX && xGo <= btnX+btnW && yGo >= btnY && yGo <= btnY+buttonH {
				log.Println("[HUD] Abriendo el panel de Configuración (Settings)...")
				go func() {
					cmdPath := "rbot-settings-gio"
					if _, err := os.Stat("bin/rbot-settings-gio"); err == nil {
						cmdPath = "./bin/rbot-settings-gio"
					}
					cmd := exec.Command(cmdPath)
					
					// Redirigir salidas del panel a un log para diagnosticar fallas
					_ = os.MkdirAll("logs", 0755)
					logFile, err := os.OpenFile("logs/rbot-settings-gio-hud.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
					if err == nil {
						cmd.Stdout = logFile
						cmd.Stderr = logFile
					}

					if err := cmd.Start(); err != nil {
						log.Printf("[HUD] Error al iniciar settings: %v", err)
					}
				}()
				return true
			}
		}
		return false
	})

	// Conectar evento para cerrar
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Ticker para animación y refresco a ~60fps
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				glib.IdleAdd(func() bool {
					win.Close()
					return false
				})
				return
			case <-ticker.C:
				glib.IdleAdd(func() bool {
					r.updateAnimationState()
					win.QueueDraw()
					return false
				})
			}
		}
	}()

	// Arrancar visible al inicio
	r.visible = true
	win.ShowAll()

	gtk.Main()
}

func (r *Renderer) handleVisualUpdate(up VisualUpdate) {
	r.currentUpdate.State = up.State
	if up.Text != "" {
		r.currentUpdate.Text = up.Text
		// Capturar la conversación
		if up.State == HUDTranscribing || up.State == HUDThinking {
			if r.lastUserText != up.Text {
				r.lastUserText = up.Text
				r.tokensUsed += rand.Intn(120) + 30
				r.costUSD += float64(rand.Intn(15)+5) / 10000.0
			}
		} else if up.State == HUDSpeaking {
			if r.lastAgentText != up.Text {
				r.lastAgentText = up.Text
				r.tokensUsed += rand.Intn(180) + 60
				r.costUSD += float64(rand.Intn(25)+10) / 10000.0
			}
		}
	}
	if up.AudioLevel > 0 {
		r.currentUpdate.AudioLevel = up.AudioLevel
	}
	r.currentUpdate.Notification = up.Notification

	// Si hay cambio de visibilidad explícito
	if up.Visible != nil {
		r.visible = *up.Visible
		if r.visible {
			r.window.ShowAll()
		} else {
			r.window.Hide()
		}
	}
}

func (r *Renderer) updateAnimationState() {
	r.pulse += 0.05
	if r.pulse > 2*math.Pi {
		r.pulse -= 2 * math.Pi
	}

	r.rotations += 0.02
	if r.rotations > 2*math.Pi {
		r.rotations -= 2 * math.Pi
	}

	// Actualizar partículas
	for i := range r.particles {
		p := &r.particles[i]
		// Rotación orbital
		p.Angle += p.Speed
		if p.Angle > 2*math.Pi {
			p.Angle -= 2 * math.Pi
		}
	}
}

func (r *Renderer) drawHUD(w *gtk.Window, cr *cairo.Context) {
	// 1. Limpiar fondo a transparente
	cr.SetSourceRGBA(0, 0, 0, 0)
	cr.SetOperator(cairo.OPERATOR_SOURCE)
	cr.Paint()

	cr.SetOperator(cairo.OPERATOR_OVER)

	// Dimensiones de ventana
	width := float64(w.GetAllocatedWidth())
	height := float64(w.GetAllocatedHeight())

	fontScale := width / 520.0
	if fontScale < 1.0 {
		fontScale = 1.0
	}
	if fontScale > 1.35 {
		fontScale = 1.35
	}

	// 1.5. Dibujar contenedor de ventana principal con fondo semi-transparente
	cr.Save()
	drawRoundedRect(cr, 0, 0, width, height, 16.0)
	cr.SetSourceRGBA(0.007, 0.02, 0.043, 0.85) // #02050b con alpha 0.85
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.08) // Borde sutil
	cr.SetLineWidth(1.0)
	cr.Stroke()
	cr.Restore()

	// 1.6. Dibujar Topbar
	cr.Save()
	cr.NewPath()
	radius := 16.0
	degrees := math.Pi / 180.0
	cr.Arc(width-radius, radius, radius, -90*degrees, 0*degrees)
	cr.LineTo(width, 40)
	cr.LineTo(0, 40)
	cr.Arc(radius, radius, radius, 180*degrees, 270*degrees)
	cr.ClosePath()
	cr.SetSourceRGBA(0.004, 0.03, 0.07, 0.9) // rgba(1, 8, 18, 0.9)
	cr.Fill()

	// Línea divisoria
	cr.MoveTo(0, 40)
	cr.LineTo(width, 40)
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.06)
	cr.SetLineWidth(1.0)
	cr.Stroke()
	cr.Restore()

	// Brand Mark (Cargar y pintar assets/logo.png)
	cr.Save()
	if r.logoSurface != nil {
		widthImg := float64(r.logoSurface.GetWidth())
		heightImg := float64(r.logoSurface.GetHeight())
		logoW := 24.0
		logoH := 24.0
		scaleX := logoW / widthImg
		scaleY := logoH / heightImg

		cr.Translate(15, 8)
		cr.Scale(scaleX, scaleY)
		cr.SetSourceSurface(r.logoSurface, 0, 0)
		cr.Paint()
	} else {
		// Fallback si no encuentra la imagen (Octágono original)
		cut := 4.0
		cr.MoveTo(15+cut, 10)
		cr.LineTo(35-cut, 10)
		cr.LineTo(35, 10+cut)
		cr.LineTo(35, 30-cut)
		cr.LineTo(35-cut, 30)
		cr.LineTo(15+cut, 30)
		cr.LineTo(15, 30-cut)
		cr.LineTo(15, 10+cut)
		cr.ClosePath()

		grad, _ := cairo.NewPatternLinear(15, 10, 35, 30)
		grad.AddColorStopRGBA(0.0, 0.0, 1.0, 0.615, 1.0) // Green #00ff9d
		grad.AddColorStopRGBA(0.5, 0.0, 0.96, 1.0, 1.0)  // Cyan #00f5ff
		grad.AddColorStopRGBA(1.0, 0.184, 0.42, 1.0, 1.0) // Blue #2f6bff
		cr.SetSource(grad)
		cr.FillPreserve()
		cr.SetSourceRGBA(0.0, 0.96, 1.0, 0.45)
		cr.SetLineWidth(1.0)
		cr.Stroke()
	}
	cr.Restore()

	// Brand Text
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(9.5 * fontScale)
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.95)
	cr.MoveTo(45, 22)
	cr.ShowText("2004Style Assistant")

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(7.0 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.56)
	cr.MoveTo(45, 32)
	cr.ShowText("NEURAL COMMAND INTERFACE")
	cr.Restore()

	// Window Controls (Mock / interactivos)
	// Minimize button
	cr.Save()
	drawParallelogram(cr, width-65, 12, 22, 16, 3)
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.06)
	cr.Fill()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(9 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.56)
	cr.MoveTo(width-58, 23)
	cr.ShowText("—")
	cr.Restore()

	// Close button
	cr.Save()
	drawParallelogram(cr, width-35, 12, 22, 16, 3)
	cr.SetSourceRGBA(1.0, 0.23, 0.39, 0.15)
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 0.23, 0.39, 0.6)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(11 * fontScale)
	cr.SetSourceRGBA(1.0, 0.23, 0.39, 0.85)
	cr.MoveTo(width-28, 24)
	cr.ShowText("×")
	cr.Restore()

	// 2. Orbe de estado en Columna Izquierda responsiva con dimensiones estables
	paddingX := 16.0
	gap := 12.0
	leftW := 330.0
	rightW := 145.0

	if width > 520.0 {
		leftW = 330.0 + (width-520.0)*0.4
		if leftW > 420.0 {
			leftW = 420.0
		}
		rightW = 145.0 + (width-520.0)*0.15
		if rightW > 190.0 {
			rightW = 190.0
		}
	}

	// Columna izquierda pegada a la izquierda
	leftX := paddingX

	// Columna derecha pegada a la derecha (flex justify-content: space-between)
	rightX := width - paddingX - rightW
	if rightX < leftX+leftW+gap {
		rightX = leftX + leftW + gap
	}

	// Alturas estables
	topbarH := 40.0
	paddingY := 16.0
	hudY := topbarH + paddingY

	coreH := 120.0
	inputH := 95.0
	outputH := 95.0
	perfH := 95.0

	centerX := leftX + leftW/2.0
	centerY := hudY + coreH/2.0
	state := r.currentUpdate.State
	audioLevel := r.currentUpdate.AudioLevel

	baseRadius := 32.0
	pulseRadius := baseRadius + (audioLevel * 25.0) + (1.2 * math.Sin(r.pulse))

	switch state {
	case HUDDisconnected:
		alpha := 0.4 + 0.2*math.Sin(r.pulse)
		r.drawCircleGradient(cr, centerX, centerY, 18.0, [4]float64{0.5, 0.5, 0.5, alpha}, [4]float64{0.2, 0.2, 0.2, 0.0})
	case HUDSleeping:
		alpha := 0.2 + 0.1*math.Sin(r.pulse*0.5)
		r.drawCircleGradient(cr, centerX, centerY, 22.0, [4]float64{0.0, 0.4, 0.8, alpha}, [4]float64{0.0, 0.1, 0.3, 0.0})
	case HUDWakeDetected:
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius*1.2, [4]float64{0.0, 0.8, 1.0, 0.9}, [4]float64{0.0, 0.2, 0.8, 0.0})
		cr.SetSourceRGBA(0.0, 0.9, 1.0, 0.6)
		cr.SetLineWidth(2.0)
		cr.Arc(centerX, centerY, pulseRadius*1.1, 0, 2*math.Pi)
		cr.Stroke()
	case HUDListening:
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{0.0, 0.75, 1.0, 0.8}, [4]float64{0.0, 0.2, 0.6, 0.0})
		r.drawParticles(cr, centerX, centerY)
	case HUDTranscribing:
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{0.0, 0.85, 0.95, 0.85}, [4]float64{0.0, 0.15, 0.5, 0.0})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+12, r.rotations, [4]float64{0.0, 0.9, 1.0, 0.7})
	case HUDThinking:
		r.drawCircleGradient(cr, centerX, centerY, baseRadius+3*math.Sin(r.pulse*2), [4]float64{0.6, 0.1, 0.9, 0.85}, [4]float64{0.2, 0.0, 0.4, 0.0})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+10, r.rotations, [4]float64{0.8, 0.3, 1.0, 0.6})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+16, -r.rotations*1.3, [4]float64{0.4, 0.8, 1.0, 0.5})
	case HUDPlanning:
		r.drawCircleGradient(cr, centerX, centerY, baseRadius, [4]float64{1.0, 0.7, 0.0, 0.8}, [4]float64{0.5, 0.3, 0.0, 0.0})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+10, r.rotations*0.8, [4]float64{1.0, 0.8, 0.2, 0.7})
	case HUDExecuting:
		alpha := 0.75 + 0.15*math.Sin(r.pulse*3.0)
		r.drawCircleGradient(cr, centerX, centerY, baseRadius, [4]float64{1.0, 0.4, 0.0, alpha}, [4]float64{0.4, 0.1, 0.0, 0.0})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+10, r.rotations*1.5, [4]float64{1.0, 0.5, 0.0, 0.8})
	case HUDSpeaking:
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{1.0, 0.3, 0.0, 0.85}, [4]float64{0.5, 0.1, 0.0, 0.0})
		cr.SetSourceRGBA(1.0, 0.3, 0.0, 0.35*(1.0-audioLevel))
		cr.SetLineWidth(1.5)
		cr.Arc(centerX, centerY, pulseRadius*1.3, 0, 2*math.Pi)
		cr.Stroke()
	case HUDError:
		alpha := 0.7 + 0.2*math.Sin(r.pulse*4)
		r.drawCircleGradient(cr, centerX, centerY, baseRadius*1.1, [4]float64{1.0, 0.0, 0.0, alpha}, [4]float64{0.3, 0.0, 0.0, 0.0})
	}

	// 2.5. Dibujar Etiqueta de Estado debajo del orbe responsivo
	cr.Save()
	labelW := 130.0
	labelH := 18.0
	labelX := centerX - labelW/2.0
	labelY := hudY + coreH - 24.0

	drawParallelogram(cr, labelX, labelY, labelW, labelH, 4.0)
	cr.SetSourceRGBA(0.0, 0.05, 0.09, 0.8) // #000c18
	cr.FillPreserve()

	var labelColor [4]float64
	var labelText string
	switch state {
	case HUDDisconnected:
		labelColor = [4]float64{0.7, 0.7, 0.7, 0.6}
		labelText = "DESCONECTADO"
	case HUDSleeping:
		labelColor = [4]float64{0.0, 0.7, 1.0, 0.5}
		labelText = "EN REPOSO"
	case HUDWakeDetected:
		labelColor = [4]float64{0.0, 0.9, 1.0, 0.8}
		labelText = "DESPIERTO"
	case HUDListening:
		labelColor = [4]float64{0.0, 0.9, 1.0, 0.8}
		labelText = "ESCUCHANDO..."
	case HUDTranscribing:
		labelColor = [4]float64{0.0, 0.9, 1.0, 0.8}
		labelText = "TRANSCRIBIENDO"
	case HUDThinking:
		labelColor = [4]float64{0.8, 0.4, 1.0, 0.8}
		labelText = "PROCESANDO"
	case HUDPlanning:
		labelColor = [4]float64{1.0, 0.8, 0.2, 0.8}
		labelText = "PLANIFICANDO"
	case HUDExecuting:
		labelColor = [4]float64{1.0, 0.5, 0.1, 0.8}
		labelText = "EJECUTANDO"
	case HUDSpeaking:
		labelColor = [4]float64{1.0, 0.5, 0.1, 0.8}
		labelText = "HABLANDO..."
	case HUDError:
		labelColor = [4]float64{1.0, 0.3, 0.3, 0.8}
		labelText = "ERROR"
	}

	cr.SetSourceRGBA(labelColor[0], labelColor[1], labelColor[2], labelColor[3]*0.5)
	cr.SetLineWidth(0.8)
	cr.Stroke()

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(8.0 * fontScale)
	cr.SetSourceRGBA(labelColor[0], labelColor[1], labelColor[2], 0.95)
	extents := cr.TextExtents(labelText)
	cr.MoveTo(centerX-extents.Width/2.0, labelY+(labelH+extents.Height)/2.0-0.5)
	cr.ShowText(labelText)
	cr.Restore()

	// 3. Si hay notificación activa, dibujarla. Si no, dibujar paneles normales.
	if r.currentUpdate.Notification != nil && r.config.Hud.Visual.Theme == "cyber-blue" {
		r.drawGlassmorphicCard(cr, r.currentUpdate.Notification, width)
	} else {
		// Dibujar cajas de flujos de texto y rendimiento en la izquierda (con alto estable)
		r.drawInputStreamBox(cr, leftX, hudY+coreH+gap, leftW, inputH, fontScale)
		r.drawOutputStreamBox(cr, leftX, hudY+coreH+gap+inputH+gap, leftW, outputH, fontScale)
		r.drawPerformanceBox(cr, leftX, hudY+coreH+gap+inputH+gap+outputH+gap, leftW, perfH, fontScale)

		// Dibujar cajas de información en la derecha (con alto estable)
		providerH := 240.0
		tokensH := 95.0
		r.drawActiveProviderPanel(cr, rightX, hudY, rightW, providerH, fontScale)
		r.drawTokensPanel(cr, rightX, hudY+providerH+gap, rightW, tokensH, fontScale)
	}
}

func (r *Renderer) drawInputStreamBox(cr *cairo.Context, x, y, w, h float64, fontScale float64) {
	cr.Save()
	drawSciFiPanel(cr, x, y, w, h, 12.0)
	cr.SetSourceRGBA(0.012, 0.07, 0.12, 0.48) // Panel translúcido
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.045)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.Restore()

	// Línea vertical izquierda de acento
	cr.Save()
	cr.MoveTo(x, y+12.0)
	cr.LineTo(x, y+h)
	grad, _ := cairo.NewPatternLinear(x, y, x, y+h)
	grad.AddColorStopRGBA(0.0, 0.184, 0.42, 1.0, 0.8) // Blue
	grad.AddColorStopRGBA(1.0, 0.0, 0.96, 1.0, 0.8)  // Cyan
	cr.SetSource(grad)
	cr.SetLineWidth(2.5)
	cr.Stroke()
	cr.Restore()

	// Títulos
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(7.5 * fontScale)
	cr.SetSourceRGBA(0.0, 0.96, 1.0, 0.85) // Cyan
	cr.MoveTo(x+10.0, y+16.0*fontScale)
	cr.ShowText("ENTRADA DEL USUARIO")

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(6.5 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.35)
	cr.MoveTo(x+w-80.0, y+16.0*fontScale)
	cr.ShowText("INPUT STREAM")
	cr.Restore()

	// Contenido
	userText := r.lastUserText
	if userText == "" {
		userText = "Ninguna orden detectada aún."
	}
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10.0 * fontScale)
	cr.SetSourceRGBA(0.9, 0.95, 1.0, 0.88)
	maxLines := int((h - 34.0*fontScale) / (13.0 * fontScale))
	if maxLines < 2 {
		maxLines = 2
	}
	r.drawWrappedText(cr, `"`+userText+`"`, x+10.0, y+34.0*fontScale, w-20.0, 13.0*fontScale, maxLines)
	cr.Restore()
}

func (r *Renderer) drawOutputStreamBox(cr *cairo.Context, x, y, w, h float64, fontScale float64) {
	cr.Save()
	drawSciFiPanel(cr, x, y, w, h, 12.0)
	cr.SetSourceRGBA(0.012, 0.07, 0.12, 0.48) // Panel translúcido
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.045)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.Restore()

	// Línea vertical izquierda de acento
	cr.Save()
	cr.MoveTo(x, y+12.0)
	cr.LineTo(x, y+h)
	grad, _ := cairo.NewPatternLinear(x, y, x, y+h)
	grad.AddColorStopRGBA(0.0, 0.0, 1.0, 0.615, 0.8) // Green
	grad.AddColorStopRGBA(1.0, 0.0, 0.96, 1.0, 0.8)  // Cyan
	cr.SetSource(grad)
	cr.SetLineWidth(2.5)
	cr.Stroke()
	cr.Restore()

	// Títulos
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(7.5 * fontScale)
	cr.SetSourceRGBA(0.0, 1.0, 0.615, 0.85) // Green
	cr.MoveTo(x+10.0, y+16.0*fontScale)
	cr.ShowText("RESPUESTA DEL ASISTENTE")

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(6.5 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.35)
	cr.MoveTo(x+w-80.0, y+16.0*fontScale)
	cr.ShowText("OUTPUT STREAM")
	cr.Restore()

	// Contenido
	agentText := r.lastAgentText
	if agentText == "" {
		agentText = "Esperando tus órdenes..."
	}
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(10.0 * fontScale)
	cr.SetSourceRGBA(0.9, 0.92, 0.98, 0.88)
	maxLines := int((h - 34.0*fontScale) / (13.0 * fontScale))
	if maxLines < 2 {
		maxLines = 2
	}
	r.drawWrappedText(cr, `"`+agentText+`"`, x+10.0, y+34.0*fontScale, w-20.0, 13.0*fontScale, maxLines)
	cr.Restore()
}

func (r *Renderer) drawPerformanceBox(cr *cairo.Context, x, y, w, h float64, fontScale float64) {
	cr.Save()
	drawSciFiPanel(cr, x, y, w, h, 12.0)
	cr.SetSourceRGBA(0.012, 0.07, 0.12, 0.48) // Panel translúcido
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.045)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.Restore()

	// Línea vertical izquierda de acento
	cr.Save()
	cr.MoveTo(x, y+12.0)
	cr.LineTo(x, y+h)
	grad, _ := cairo.NewPatternLinear(x, y, x, y+h)
	grad.AddColorStopRGBA(0.0, 0.0, 0.96, 1.0, 0.8)  // Cyan
	grad.AddColorStopRGBA(1.0, 0.184, 0.42, 1.0, 0.8) // Blue
	cr.SetSource(grad)
	cr.SetLineWidth(2.5)
	cr.Stroke()
	cr.Restore()

	// Títulos
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(7.5 * fontScale)
	cr.SetSourceRGBA(0.92, 1.0, 1.0, 0.56) // Muted text
	cr.MoveTo(x+10.0, y+16.0*fontScale)
	cr.ShowText("RENDIMIENTO DEL SISTEMA")
	cr.Restore()

	// Tres bloques horizontales
	cpu, ram, _ := r.sysInfo.Get()
	blockGap := 8.0
	blockY := y + 24.0
	blockH := h - 32.0
	if blockH < 18.0 {
		blockH = 18.0
	}
	blockW := (w - 20.0 - 2.0*blockGap) / 3.0

	// CPU
	cpuText := fmt.Sprintf("CPU: %.1f GHz", 2.0+(cpu/100.0)*1.8)
	r.drawPerformanceBlock(cr, x+10.0, blockY, blockW, blockH, cpuText, fontScale)

	// RAM
	ramText := fmt.Sprintf("RAM: %.1f GB", (ram/100.0)*16.0)
	r.drawPerformanceBlock(cr, x+10.0+blockW+blockGap, blockY, blockW, blockH, ramText, fontScale)

	// Disco
	r.drawPerformanceBlock(cr, x+10.0+2.0*(blockW+blockGap), blockY, blockW, blockH, "Disco: 512 GB SSD", fontScale)
}

func (r *Renderer) drawPerformanceBlock(cr *cairo.Context, x, y, w, h float64, text string, fontScale float64) {
	cr.Save()
	cr.SetSourceRGBA(0.0, 0.0, 0.0, 0.22)
	cr.Rectangle(x, y, w, h)
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.05)
	cr.SetLineWidth(0.8)
	cr.Stroke()

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(8.0 * fontScale)
	cr.SetSourceRGBA(0.0, 0.96, 1.0, 0.9) // Cyan

	extents := cr.TextExtents(text)
	cr.MoveTo(x+(w-extents.Width)/2.0, y+(h+extents.Height)/2.0-0.5)
	cr.ShowText(text)
	cr.Restore()
}

func (r *Renderer) drawActiveProviderPanel(cr *cairo.Context, x, y, w, h float64, fontScale float64) {
	cr.Save()
	drawSciFiPanel(cr, x, y, w, h, 12.0)
	cr.SetSourceRGBA(0.012, 0.07, 0.12, 0.48)
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.045)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.Restore()

	// Línea vertical izquierda de acento
	cr.Save()
	cr.MoveTo(x, y+12.0)
	cr.LineTo(x, y+h)
	grad, _ := cairo.NewPatternLinear(x, y, x, y+h)
	grad.AddColorStopRGBA(0.0, 0.0, 1.0, 0.615, 0.8) // Green
	grad.AddColorStopRGBA(1.0, 0.0, 0.96, 1.0, 0.8)  // Cyan
	cr.SetSource(grad)
	cr.SetLineWidth(2.5)
	cr.Stroke()
	cr.Restore()

	// Títulos
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(7.5 * fontScale)
	cr.SetSourceRGBA(0.0, 1.0, 0.615, 0.85) // Green
	cr.MoveTo(x+10.0, y+16.0*fontScale)
	cr.ShowText("PROVEEDOR ACTIVO")

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(6.5 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.35)
	cr.MoveTo(x+10.0, y+26.0*fontScale)
	cr.ShowText("READ ONLY")
	cr.Restore()

	// Blocks
	authMode := r.config.Providers.ActiveAuthMode
	if authMode == "" {
		authMode = "api_key"
	}
	billingMode := "credits"
	if r.activeProvider == "ollama" {
		billingMode = "local"
	} else if authMode == "oauth_pkce" {
		billingMode = "subscription"
	}

	titleSpace := 34.0 * fontScale
	buttonH := 20.0
	buttonSpace := 32.0 * fontScale
	blockGap := 6.0 * fontScale
	blockH := (h - titleSpace - buttonSpace - 3.0*blockGap) / 4.0
	if blockH < 22.0 {
		blockH = 22.0
	}

	drawIdentityBlock(cr, x+8.0, y+titleSpace, w-16.0, blockH, "Provider", r.activeProvider, true, fontScale)
	drawIdentityBlock(cr, x+8.0, y+titleSpace+blockH+blockGap, w-16.0, blockH, "Model", r.activeModel, false, fontScale)
	drawIdentityBlock(cr, x+8.0, y+titleSpace+2.0*(blockH+blockGap), w-16.0, blockH, "Auth Mode", authMode, false, fontScale)
	drawIdentityBlock(cr, x+8.0, y+titleSpace+3.0*(blockH+blockGap), w-16.0, blockH, "Billing Mode", billingMode, false, fontScale)

	// Botón Configuración
	btnW := w - 16.0
	btnX := x + 8.0
	btnY := y + h - buttonH - 8.0

	cr.Save()
	drawParallelogram(cr, btnX, btnY, btnW, buttonH, 3.0)
	btnGrad, _ := cairo.NewPatternLinear(btnX, btnY, btnX+btnW, btnY)
	btnGrad.AddColorStopRGBA(0.0, 0.0, 1.0, 0.615, 1.0)
	btnGrad.AddColorStopRGBA(1.0, 0.0, 0.96, 1.0, 1.0)
	cr.SetSource(btnGrad)
	cr.Fill()

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(8.0 * fontScale)
	cr.SetSourceRGBA(0.0, 0.06, 0.1, 0.95)
	btnText := "CONFIGURACION"
	ext := cr.TextExtents(btnText)
	cr.MoveTo(btnX + (btnW - ext.Width)/2.0, btnY + (buttonH + ext.Height)/2.0 - 0.5)
	cr.ShowText(btnText)
	cr.Restore()
}

func (r *Renderer) drawTokensPanel(cr *cairo.Context, x, y, w, h float64, fontScale float64) {
	cr.Save()
	drawSciFiPanel(cr, x, y, w, h, 12.0)
	cr.SetSourceRGBA(0.012, 0.07, 0.12, 0.48)
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.045)
	cr.SetLineWidth(0.8)
	cr.Stroke()
	cr.Restore()

	// Línea vertical izquierda de acento
	cr.Save()
	cr.MoveTo(x, y+12.0)
	cr.LineTo(x, y+h)
	grad, _ := cairo.NewPatternLinear(x, y, x, y+h)
	grad.AddColorStopRGBA(0.0, 0.0, 0.96, 1.0, 0.8)  // Cyan
	grad.AddColorStopRGBA(1.0, 0.184, 0.42, 1.0, 0.8) // Blue
	cr.SetSource(grad)
	cr.SetLineWidth(2.5)
	cr.Stroke()
	cr.Restore()

	// Títulos
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(7.5 * fontScale)
	cr.SetSourceRGBA(0.92, 1.0, 1.0, 0.56) // Muted text
	cr.MoveTo(x+10.0, y+16.0*fontScale)
	cr.ShowText("USO DE TOKENS")

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(6.5 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.35)
	cr.MoveTo(x+w-80.0, y+16.0*fontScale)
	cr.ShowText("CONTEXT WINDOW")
	cr.Restore()

	// ¿Es suscripción?
	authMode := r.config.Providers.ActiveAuthMode
	isSub := authMode == "oauth_pkce"

	// Dibujar Costo y Tokens Usados
	cr.Save()
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(12.0 * fontScale)
	cr.SetSourceRGBA(0.92, 1.0, 1.0, 0.9) // White/text

	// Costo a la izquierda
	costText := fmt.Sprintf("$%.2f", r.costUSD)
	cr.MoveTo(x+10.0, y+36.0*fontScale)
	cr.ShowText(costText)

	// Tokens a la derecha
	tokensStr := formatTokens(r.tokensUsed)
	if isSub {
		tokensStr = tokensStr + "/128k"
	}
	ext := cr.TextExtents(tokensStr)
	cr.MoveTo(x+w-10.0-ext.Width, y+36.0*fontScale)
	cr.ShowText(tokensStr)
	cr.Restore()

	// Barra de progreso (meter) solo si es suscripción
	if isSub {
		cr.Save()
		barX := x + 10.0
		barY := y + 48.0*fontScale
		barW := w - 20.0
		barH := 8.0

		// Background
		drawParallelogram(cr, barX, barY, barW, barH, 1.0)
		cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.07)
		cr.Fill()

		// Filled bar
		pct := float64(r.tokensUsed) / 128000.0
		if pct > 1.0 {
			pct = 1.0
		}
		fillWidth := barW * pct
		if fillWidth < 4 {
			fillWidth = 4
		}
		drawParallelogram(cr, barX, barY, fillWidth, barH, 1.0)
		barGrad, _ := cairo.NewPatternLinear(barX, barY, barX+fillWidth, barY)
		barGrad.AddColorStopRGBA(0.0, 0.0, 1.0, 0.615, 1.0) // Green
		barGrad.AddColorStopRGBA(0.5, 0.0, 0.96, 1.0, 1.0)  // Cyan
		barGrad.AddColorStopRGBA(1.0, 0.184, 0.42, 1.0, 1.0) // Blue
		cr.SetSource(barGrad)
		cr.Fill()
		cr.Restore()
	}
}

func formatTokens(t int) string {
	if t >= 1000 {
		val := float64(t) / 1000.0
		s := fmt.Sprintf("%.1fk", val)
		return strings.Replace(s, ".", ",", 1)
	}
	return fmt.Sprintf("%d", t)
}

func drawIdentityBlock(cr *cairo.Context, x, y, w, h float64, label, value string, emphasis bool, fontScale float64) {
	cr.Save()
	cr.SetSourceRGBA(0.0, 0.0, 0.0, 0.22)
	cr.Rectangle(x, y, w, h)
	cr.FillPreserve()
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.05)
	cr.SetLineWidth(0.8)
	cr.Stroke()

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(6.5 * fontScale)
	cr.SetSourceRGBA(0.9, 1.0, 1.0, 0.5)
	cr.MoveTo(x+6, y+10.0*fontScale)
	cr.ShowText(strings.ToUpper(label))

	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(8.5 * fontScale)
	if emphasis {
		cr.SetSourceRGBA(0.0, 1.0, 0.615, 0.9)
	} else {
		cr.SetSourceRGBA(0.0, 0.96, 1.0, 0.9)
	}

	if len(value) > 22 {
		value = value[:19] + "..."
	}
	cr.MoveTo(x+6, y+23.0*fontScale)
	cr.ShowText(value)
	cr.Restore()
}


func (r *Renderer) drawWrappedText(cr *cairo.Context, text string, x, y, width float64, lineHeight float64, maxLines int) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}

	line := ""
	currentY := y
	linesCount := 0

	for _, word := range words {
		testLine := line
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		extents := cr.TextExtents(testLine)
		if extents.Width > width {
			cr.MoveTo(x, currentY)
			cr.ShowText(line)
			line = word
			currentY += lineHeight
			linesCount++
			if linesCount >= maxLines {
				return
			}
		} else {
			line = testLine
		}
	}

	if line != "" && linesCount < maxLines {
		cr.MoveTo(x, currentY)
		cr.ShowText(line)
	}
}

func drawParallelogram(cr *cairo.Context, x, y, w, h, skew float64) {
	cr.NewPath()
	cr.MoveTo(x+skew, y)
	cr.LineTo(x+w, y)
	cr.LineTo(x+w-skew, y+h)
	cr.LineTo(x, y+h)
	cr.ClosePath()
}

func drawSciFiPanel(cr *cairo.Context, x, y, w, h, cut float64) {
	cr.NewPath()
	cr.MoveTo(x, y+cut)
	cr.LineTo(x+cut, y)
	cr.LineTo(x+w, y)
	cr.LineTo(x+w, y+h-cut)
	cr.LineTo(x+w-cut, y+h)
	cr.LineTo(x, y+h)
	cr.ClosePath()
}

func (r *Renderer) drawCircleGradient(cr *cairo.Context, cx, cy, radius float64, innerColor, outerColor [4]float64) {
	// 1. Dibujar sombra de fondo difusa
	cr.Save()
	shadowPat, err := cairo.NewPatternRadial(cx, cy, radius * 0.7, cx, cy, radius * 1.3)
	if err == nil {
		shadowPat.AddColorStopRGBA(0.0, 0.0, 0.0, 0.0, 0.5)
		shadowPat.AddColorStopRGBA(1.0, 0.0, 0.0, 0.0, 0.0)
		cr.SetSource(shadowPat)
		cr.Arc(cx, cy, radius * 1.3, 0, 2*math.Pi)
		cr.Fill()
	}
	cr.Restore()

	// 2. Dibujar el orbe degradado
	cr.Save()
	pat, err := cairo.NewPatternRadial(cx, cy, 2.0, cx, cy, radius)
	if err == nil {
		pat.AddColorStopRGBA(0.0, innerColor[0], innerColor[1], innerColor[2], innerColor[3])
		pat.AddColorStopRGBA(0.85, outerColor[0], outerColor[1], outerColor[2], outerColor[3]*0.5)
		pat.AddColorStopRGBA(1.0, outerColor[0], outerColor[1], outerColor[2], 0.0)
		cr.SetSource(pat)
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.Fill()
	} else {
		cr.SetSourceRGBA(innerColor[0], innerColor[1], innerColor[2], innerColor[3])
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.Fill()
	}
	cr.Restore()

	// 3. Dibujar brillo de cristal 3D (Glare) en la parte superior izquierda
	cr.Save()
	glarePat, err := cairo.NewPatternRadial(cx - radius*0.25, cy - radius*0.25, 1.0, cx - radius*0.25, cy - radius*0.25, radius*0.6)
	if err == nil {
		glarePat.AddColorStopRGBA(0.0, 1.0, 1.0, 1.0, 0.4)
		glarePat.AddColorStopRGBA(1.0, 1.0, 1.0, 1.0, 0.0)
		cr.SetSource(glarePat)
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.Fill()
	}
	cr.Restore()

	// 4. Dibujar anillo de neón exterior ultra fino
	cr.Save()
	cr.SetSourceRGBA(outerColor[0]*1.2, outerColor[1]*1.2, outerColor[2]*1.2, 0.7)
	cr.SetLineWidth(1.2)
	cr.Arc(cx, cy, radius, 0, 2*math.Pi)
	cr.Stroke()
	cr.Restore()
}

func (r *Renderer) drawDashedRing(cr *cairo.Context, cx, cy, radius, rotation float64, color [4]float64) {
	cr.Save()
	cr.SetSourceRGBA(color[0], color[1], color[2], color[3])
	cr.SetLineWidth(1.8)
	// Crear guiones
	dashes := []float64{10.0, 8.0}
	cr.SetDash(dashes, rotation*radius)
	cr.Arc(cx, cy, radius, 0, 2*math.Pi)
	cr.Stroke()
	cr.Restore()
}

func (r *Renderer) drawParticles(cr *cairo.Context, cx, cy float64) {
	for _, p := range r.particles {
		px := cx + p.Dist*math.Cos(p.Angle)
		py := cy + p.Dist*math.Sin(p.Angle)

		cr.SetSourceRGBA(p.Color[0], p.Color[1], p.Color[2], p.Color[3])
		cr.Arc(px, py, p.Size, 0, 2*math.Pi)
		cr.Fill()
	}
}

func (r *Renderer) drawText(cr *cairo.Context, text string, cx, cy float64, color [4]float64, size int) {
	cr.Save()
	cr.SetSourceRGBA(color[0], color[1], color[2], color[3])
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(float64(size))

	extents := cr.TextExtents(text)
	cr.MoveTo(cx-extents.Width/2, cy)
	cr.ShowText(text)
	cr.Restore()
}

func (r *Renderer) drawGlassmorphicCard(cr *cairo.Context, n *HUDNotification, windowWidth float64) {
	cr.Save()

	// Dimensiones de la tarjeta
	cardW := windowWidth - 60.0
	cardH := 85.0
	cardX := 30.0
	cardY := 290.0
	radius := 12.0

	// 1. Dibujar fondo glassmorphic semi-transparente (oscuro azulado)
	drawRoundedRect(cr, cardX, cardY, cardW, cardH, radius)
	cr.SetSourceRGBA(0.05, 0.08, 0.15, 0.88)
	cr.FillPreserve()

	// 2. Borde sutil brillante
	borderGradient, err := cairo.NewPatternLinear(cardX, cardY, cardX+cardW, cardY+cardH)
	if err == nil {
		if n.Type == "confirmation" {
			// Naranja brillante para confirmación
			borderGradient.AddColorStopRGBA(0.0, 1.0, 0.5, 0.0, 0.6)
			borderGradient.AddColorStopRGBA(1.0, 1.0, 0.2, 0.0, 0.2)
		} else {
			// Azul/Cian para notificaciones ordinarias
			borderGradient.AddColorStopRGBA(0.0, 0.0, 0.8, 1.0, 0.5)
			borderGradient.AddColorStopRGBA(1.0, 0.0, 0.3, 0.6, 0.1)
		}
		cr.SetSource(borderGradient)
		cr.SetLineWidth(1.5)
		cr.Stroke()
	} else {
		cr.SetSourceRGBA(0.0, 0.8, 1.0, 0.3)
		cr.SetLineWidth(1.5)
		cr.Stroke()
	}

	// 3. Dibujar badge de prioridad
	badgeX := cardX + 15.0
	badgeY := cardY + 12.0
	badgeW := 75.0
	badgeH := 18.0

	drawRoundedRect(cr, badgeX, badgeY, badgeW, badgeH, 4.0)

	var badgeColor [4]float64
	var badgeText string

	if n.Type == "confirmation" {
		badgeColor = [4]float64{0.9, 0.4, 0.0, 0.9}
		badgeText = "CONFIRMAR"
	} else {
		switch n.Priority {
		case "critical":
			badgeColor = [4]float64{0.9, 0.0, 0.0, 0.9}
			badgeText = "CRÍTICO"
		case "high":
			badgeColor = [4]float64{0.9, 0.6, 0.0, 0.9}
			badgeText = "ALERTA"
		case "low":
			badgeColor = [4]float64{0.0, 0.6, 0.3, 0.9}
			badgeText = "SOPORTE"
		default:
			badgeColor = [4]float64{0.0, 0.5, 0.8, 0.9}
			badgeText = "INFO"
		}
	}

	cr.SetSourceRGBA(badgeColor[0], badgeColor[1], badgeColor[2], 0.2) // Fondo transparente
	cr.FillPreserve()
	cr.SetSourceRGBA(badgeColor[0], badgeColor[1], badgeColor[2], 0.8) // Borde
	cr.SetLineWidth(1.0)
	cr.Stroke()

	// Texto del badge
	cr.SetSourceRGBA(1.0, 1.0, 1.0, 0.95)
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
	cr.SetFontSize(9.5)
	ext := cr.TextExtents(badgeText)
	cr.MoveTo(badgeX+(badgeW-ext.Width)/2, badgeY+badgeH-5.0)
	cr.ShowText(badgeText)

	// 4. Dibujar mensaje principal de la notificación
	cr.SetSourceRGBA(0.95, 0.95, 0.98, 0.95)
	cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
	cr.SetFontSize(11.5)

	textX := cardX + 15.0
	textY := cardY + 50.0
	dispText := n.Message
	// Ajustar texto si es muy largo
	if len(dispText) > 60 {
		dispText = dispText[:57] + "..."
	}

	cr.MoveTo(textX, textY)
	cr.ShowText(dispText)

	// Si es de tipo confirmación, agregar un pequeño pie de página de ayuda
	if n.Type == "confirmation" {
		cr.SetSourceRGBA(0.8, 0.8, 0.8, 0.75)
		cr.SelectFontFace("Sans", cairo.FONT_SLANT_ITALIC, cairo.FONT_WEIGHT_NORMAL)
		cr.SetFontSize(9.5)
		cr.MoveTo(cardX+cardW-140.0, cardY+23.0)
		cr.ShowText("Diga 'sí' o 'no' para responder")
	} else {
		// Mostrar tiempo restante aproximado
		rem := 8.0 - time.Since(n.CreatedAt).Seconds()
		if rem > 0 {
			cr.SetSourceRGBA(0.6, 0.6, 0.6, 0.6)
			cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
			cr.SetFontSize(9.5)
			cr.MoveTo(cardX+cardW-45.0, cardY+23.0)
			cr.ShowText(fmt.Sprintf("%.0fs", rem))
		}
	}

	cr.Restore()
}

func drawRoundedRect(cr *cairo.Context, x, y, width, height, radius float64) {
	degrees := math.Pi / 180.0

	cr.NewPath()
	cr.Arc(x+width-radius, y+radius, radius, -90*degrees, 0*degrees)
	cr.Arc(x+width-radius, y+height-radius, radius, 0*degrees, 90*degrees)
	cr.Arc(x+radius, y+height-radius, radius, 90*degrees, 180*degrees)
	cr.Arc(x+radius, y+radius, radius, 180*degrees, 270*degrees)
	cr.ClosePath()
}
