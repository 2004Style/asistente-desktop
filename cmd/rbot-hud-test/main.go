//go:build hud

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
*/
import "C"

import (
	"fmt"
	"log"
	"math"
	"time"
	"unsafe"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
)

func setWindowInputShape(gdkWin uintptr, region uintptr) {
	C.gdk_window_input_shape_combine_region(
		(*C.GdkWindow)(unsafe.Pointer(gdkWin)),
		(*C.cairo_region_t)(unsafe.Pointer(region)),
		0,
		0,
	)
}

func main() {
	gtk.Init(nil)

	// Crear ventana
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatalf("Error creando ventana: %v", err)
	}

	win.SetTitle("RBot HUD Test")
	win.SetDefaultSize(400, 300)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// 1. Quitar bordes (borderless/decorated = false)
	win.SetDecorated(false)

	// 2. Always on top (keep above)
	win.SetKeepAbove(true)

	// 3. No focus (no capturar foco de teclado ni ventana activa)
	win.SetAcceptFocus(false)
	win.SetFocusOnMap(false)

	// 4. Habilitar RGBA para transparencia
	screen := win.GetScreen()
	visual, err := screen.GetRGBAVisual()
	if err != nil {
		log.Printf("Advertencia obteniendo RGBA visual: %v", err)
	}
	if visual != nil && screen.IsComposited() {
		win.SetVisual(visual)
	}
	win.SetAppPaintable(true)

	// 5. Configurar Click-Through (Input Shape)
	// Haremos que la ventana sea 100% click-through (el cursor pasa a través)
	win.Connect("realize", func() {
		gdkWin, err := win.GetWindow()
		if err != nil {
			log.Printf("Error obteniendo ventana GDK: %v", err)
			return
		}
		if gdkWin != nil {
			// Crear una región Cairo vacía
			cRegion, err := cairo.RegionCreate()
			if err != nil {
				log.Printf("Error al crear cairo region: %v", err)
				return
			}
			// Usar Cgo para llamar a la función de GDK
			setWindowInputShape(gdkWin.Native(), cRegion.Native())
			fmt.Println("[HUD Test] Región de input shape configurada (click-through activo).")
		}
	})

	// Variable para animación
	pulse := 0.0

	// 6. Conectar al callback de dibujo (Draw)
	win.Connect("draw", func(w *gtk.Window, cr *cairo.Context) bool {
		// Limpiar fondo a 100% transparente
		cr.SetSourceRGBA(0, 0, 0, 0)
		cr.SetOperator(cairo.OPERATOR_SOURCE)
		cr.Paint()

		// Cambiar operador a OVER para dibujar figuras visibles
		cr.SetOperator(cairo.OPERATOR_OVER)

		// Dibujar un orbe (círculo) degradado y animado en el centro
		width := float64(w.GetAllocatedWidth())
		height := float64(w.GetAllocatedHeight())
		centerX := width / 2
		centerY := height / 2

		// Radio base 60 + pulso senoidal
		radius := 60.0 + 15.0*math.Sin(pulse)

		// Dibujar orbe principal con degradado radial
		pat, err := cairo.NewPatternRadial(centerX, centerY, 5, centerX, centerY, radius)
		if err == nil {
			pat.AddColorStopRGBA(0, 0.0, 0.7, 1.0, 0.8) // Azul claro brillante en el centro
			pat.AddColorStopRGBA(1, 0.0, 0.2, 0.8, 0.0) // Azul oscuro transparente al borde

			cr.SetSource(pat)
			cr.Arc(centerX, centerY, radius, 0, 2*math.Pi)
			cr.Fill()
		}

		// Dibujar un anillo exterior más fino
		cr.SetSourceRGBA(0.0, 0.8, 1.0, 0.4)
		cr.SetLineWidth(2.0)
		cr.Arc(centerX, centerY, radius+10, 0, 2*math.Pi)
		cr.Stroke()

		// Escribir texto descriptivo
		cr.SetSourceRGBA(1, 1, 1, 0.9)
		cr.SelectFontFace("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
		cr.SetFontSize(14)

		text := "RBot HUD Test: Click-Through / Siempre Visible"
		extents := cr.TextExtents(text)
		cr.MoveTo(centerX-extents.Width/2, centerY+radius+35)
		cr.ShowText(text)

		return true
	})

	// 7. Cerrar con Ctrl+C o cerrando ventana
	win.Connect("destroy", gtk.MainQuit)

	// Loop de animación (actualizar pulso a 60 FPS)
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		for range ticker.C {
			pulse += 0.05
			if pulse > 2*math.Pi {
				pulse -= 2 * math.Pi
			}
			// Redibujar
			win.QueueDraw()
		}
	}()

	win.ShowAll()
	fmt.Println("[HUD Test] Iniciando HUD de prueba. Deberías ver un orbe azul animado en tu pantalla. Puedes hacer click a través de él.")
	gtk.Main()
}
