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
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"rbot/internal/config"
)

type Particle struct {
	Angle float64
	Speed float64
	Dist  float64
	Size  float64
	Color [4]float64
}

type Renderer struct {
	window        *gtk.Window
	config        *config.Config
	mapper        *EventMapper
	currentUpdate VisualUpdate
	pulse         float64
	rotations     float64
	particles     []Particle
	width         int
	height        int
	visible       bool
	client        *Client
}

func NewRenderer(conf *config.Config) *Renderer {
	w := conf.Hud.Window.Width
	h := conf.Hud.Window.Height
	if w <= 0 {
		w = 520
	}
	if h <= 0 {
		h = 420
	}

	r := &Renderer{
		config:    conf,
		mapper:    NewEventMapper(conf.Hud.Visual.AudioSmoothing),
		width:     w,
		height:    h,
		visible:   false,
		particles: make([]Particle, 40),
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

	// Por defecto ocultar ventana al inicio
	r.visible = false

	gtk.Main()
}

func (r *Renderer) handleVisualUpdate(up VisualUpdate) {
	r.currentUpdate.State = up.State
	if up.Text != "" {
		r.currentUpdate.Text = up.Text
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
	_ = float64(w.GetAllocatedHeight())
	centerX := width / 2
	centerY := 160.0 // Mantener el orbe centrado verticalmente más arriba para dejar espacio a textos y notificaciones

	// 2. Dibujar el orbe y efectos según estado
	state := r.currentUpdate.State
	audioLevel := r.currentUpdate.AudioLevel

	baseRadius := 45.0
	// El audio level pulsa el tamaño del orbe
	pulseRadius := baseRadius + (audioLevel * 40.0) + (2.0 * math.Sin(r.pulse))

	switch state {
	case HUDDisconnected:
		// Orbe gris parpadeante discreto
		alpha := 0.4 + 0.2*math.Sin(r.pulse)
		r.drawCircleGradient(cr, centerX, centerY, 20.0, [4]float64{0.5, 0.5, 0.5, alpha}, [4]float64{0.2, 0.2, 0.2, 0.0})
		r.drawText(cr, "RBot no está activo. Reconectando...", centerX, centerY+60.0, [4]float64{0.7, 0.7, 0.7, 0.8}, 13)

	case HUDSleeping:
		// Orbe azul oscuro muy apagado parpadeando lento
		alpha := 0.2 + 0.1*math.Sin(r.pulse*0.5)
		r.drawCircleGradient(cr, centerX, centerY, 25.0, [4]float64{0.0, 0.4, 0.8, alpha}, [4]float64{0.0, 0.1, 0.3, 0.0})

	case HUDWakeDetected:
		// Expansión rápida inicial azul brillante
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius*1.2, [4]float64{0.0, 0.8, 1.0, 0.9}, [4]float64{0.0, 0.2, 0.8, 0.0})
		// Anillo exterior
		cr.SetSourceRGBA(0.0, 0.9, 1.0, 0.6)
		cr.SetLineWidth(2.0)
		cr.Arc(centerX, centerY, pulseRadius*1.1, 0, 2*math.Pi)
		cr.Stroke()

	case HUDListening:
		// Orbe cian vibrante
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{0.0, 0.75, 1.0, 0.8}, [4]float64{0.0, 0.2, 0.6, 0.0})
		// Dibujar partículas orbitales
		r.drawParticles(cr, centerX, centerY)
		r.drawText(cr, "Escuchando...", centerX, centerY+70.0, [4]float64{0.0, 0.9, 1.0, 0.9}, 14)

	case HUDTranscribing:
		// Orbe cian con texto de transcripción
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{0.0, 0.85, 0.95, 0.85}, [4]float64{0.0, 0.15, 0.5, 0.0})
		// Anillo exterior girando
		r.drawDashedRing(cr, centerX, centerY, baseRadius+15, r.rotations, [4]float64{0.0, 0.9, 1.0, 0.7})
		if r.currentUpdate.Text != "" {
			r.drawText(cr, `"`+r.currentUpdate.Text+`"`, centerX, centerY+70.0, [4]float64{1.0, 1.0, 1.0, 0.95}, 14)
		}

	case HUDThinking:
		// Orbe púrpura misterioso con doble anillo rotatorio
		r.drawCircleGradient(cr, centerX, centerY, baseRadius+4*math.Sin(r.pulse*2), [4]float64{0.6, 0.1, 0.9, 0.85}, [4]float64{0.2, 0.0, 0.4, 0.0})
		// Doble anillo
		r.drawDashedRing(cr, centerX, centerY, baseRadius+12, r.rotations, [4]float64{0.8, 0.3, 1.0, 0.6})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+20, -r.rotations*1.3, [4]float64{0.4, 0.8, 1.0, 0.5})
		r.drawText(cr, "Procesando orden...", centerX, centerY+70.0, [4]float64{0.8, 0.4, 1.0, 0.9}, 14)

	case HUDPlanning:
		// Orbe amarillo/ámbar de planeamiento
		r.drawCircleGradient(cr, centerX, centerY, baseRadius, [4]float64{1.0, 0.7, 0.0, 0.8}, [4]float64{0.5, 0.3, 0.0, 0.0})
		// Engranajes/anillos amarillo/naranja
		r.drawDashedRing(cr, centerX, centerY, baseRadius+12, r.rotations*0.8, [4]float64{1.0, 0.8, 0.2, 0.7})
		r.drawText(cr, "Planificando tareas...", centerX, centerY+70.0, [4]float64{1.0, 0.8, 0.2, 0.9}, 14)

	case HUDExecuting:
		// Orbe naranja parpadeando rápido
		alpha := 0.75 + 0.15*math.Sin(r.pulse*3.0)
		r.drawCircleGradient(cr, centerX, centerY, baseRadius, [4]float64{1.0, 0.4, 0.0, alpha}, [4]float64{0.4, 0.1, 0.0, 0.0})
		r.drawDashedRing(cr, centerX, centerY, baseRadius+12, r.rotations*1.5, [4]float64{1.0, 0.5, 0.0, 0.8})
		r.drawText(cr, "Ejecutando herramienta...", centerX, centerY+70.0, [4]float64{1.0, 0.5, 0.1, 0.95}, 14)

	case HUDSpeaking:
		// Orbe rojo/naranja que vibra fuertemente con el nivel de audio
		r.drawCircleGradient(cr, centerX, centerY, pulseRadius, [4]float64{1.0, 0.3, 0.0, 0.85}, [4]float64{0.5, 0.1, 0.0, 0.0})
		// Ondas expansivas concéntricas
		cr.SetSourceRGBA(1.0, 0.3, 0.0, 0.35*(1.0-audioLevel))
		cr.SetLineWidth(1.5)
		cr.Arc(centerX, centerY, pulseRadius*1.3, 0, 2*math.Pi)
		cr.Stroke()
		if r.currentUpdate.Text != "" {
			// Limitar longitud del texto mostrado en pantalla para evitar desbordes
			dispText := r.currentUpdate.Text
			if len(dispText) > 65 {
				dispText = dispText[:62] + "..."
			}
			r.drawText(cr, dispText, centerX, centerY+70.0, [4]float64{1.0, 0.9, 0.8, 0.95}, 13)
		}

	case HUDError:
		// Halo rojo vibrante
		alpha := 0.7 + 0.2*math.Sin(r.pulse*4)
		r.drawCircleGradient(cr, centerX, centerY, baseRadius*1.1, [4]float64{1.0, 0.0, 0.0, alpha}, [4]float64{0.3, 0.0, 0.0, 0.0})
		if r.currentUpdate.Text != "" {
			r.drawText(cr, "Error: "+r.currentUpdate.Text, centerX, centerY+70.0, [4]float64{1.0, 0.3, 0.3, 0.95}, 13)
		} else {
			r.drawText(cr, "Ocurrió un error", centerX, centerY+70.0, [4]float64{1.0, 0.3, 0.3, 0.95}, 13)
		}
	}

	// 3. Dibujar la tarjeta de Notificación o Confirmación (Glassmorphism)
	if r.currentUpdate.Notification != nil && r.config.Hud.Visual.Theme == "cyber-blue" {
		r.drawGlassmorphicCard(cr, r.currentUpdate.Notification, width)
	}
}

func (r *Renderer) drawCircleGradient(cr *cairo.Context, cx, cy, radius float64, innerColor, outerColor [4]float64) {
	pat, err := cairo.NewPatternRadial(cx, cy, 2.0, cx, cy, radius)
	if err != nil {
		// Fallback color plano
		cr.SetSourceRGBA(innerColor[0], innerColor[1], innerColor[2], innerColor[3])
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.Fill()
		return
	}
	pat.AddColorStopRGBA(0.0, innerColor[0], innerColor[1], innerColor[2], innerColor[3])
	pat.AddColorStopRGBA(1.0, outerColor[0], outerColor[1], outerColor[2], outerColor[3])
	cr.SetSource(pat)
	cr.Arc(cx, cy, radius, 0, 2*math.Pi)
	cr.Fill()
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
