package browser

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

// WindowInfoBasic contiene información básica sobre una ventana del escritorio.
type WindowInfoBasic struct {
	ID      string
	Address string
	Title   string
	Class   string
	Focused bool
}

// WindowListFunc es una función que devuelve la lista de ventanas abiertas.
type WindowListFunc func(ctx context.Context) ([]WindowInfoBasic, error)

// WindowFocusFunc es una función que enfoca una ventana por su dirección o ID.
type WindowFocusFunc func(ctx context.Context, address string) error

// defaultWindowListFunc obtiene ventanas usando hyprctl (Hyprland) o xdotool (X11).
func defaultWindowListFunc(ctx context.Context) ([]WindowInfoBasic, error) {
	// Intentar hyprctl primero (Hyprland)
	if _, err := exec.LookPath("hyprctl"); err == nil {
		return listWindowsHyprctl(ctx)
	}
	// Fallback a xdotool
	if _, err := exec.LookPath("xdotool"); err == nil {
		return listWindowsXdotool(ctx)
	}
	return nil, nil
}

// listWindowsHyprctl obtiene ventanas con hyprctl clients.
func listWindowsHyprctl(ctx context.Context) ([]WindowInfoBasic, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parsear JSON de hyprctl como array de mapas
	var raw []map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	// Obtener ventana activa para marcar focused
	activeAddr := ""
	cmdActive := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j")
	if activeOut, err := cmdActive.Output(); err == nil {
		var active map[string]interface{}
		if json.Unmarshal(activeOut, &active) == nil {
			if addr, ok := active["address"].(string); ok {
				activeAddr = addr
			}
		}
	}

	var windows []WindowInfoBasic
	for _, c := range raw {
		addr, _ := c["address"].(string)
		title, _ := c["title"].(string)
		class, _ := c["class"].(string)
		focused := addr == activeAddr

		windows = append(windows, WindowInfoBasic{
			ID:      addr,
			Address: addr,
			Title:   title,
			Class:   class,
			Focused: focused,
		})
	}
	return windows, nil
}

// listWindowsXdotool obtiene ventanas con xdotool.
func listWindowsXdotool(ctx context.Context) ([]WindowInfoBasic, error) {
	cmd := exec.CommandContext(ctx, "xdotool", "search", "--onlyvisible", "--name", "")
	out, err := cmd.Output()
	if err != nil {
		// xdotool devuelve error si no hay ventanas - no es fatal
		return nil, nil
	}

	var windows []WindowInfoBasic
	ids := strings.Fields(strings.TrimSpace(string(out)))

	// Obtener ventana activa
	activeID := ""
	if activeOut, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").Output(); err == nil {
		activeID = strings.TrimSpace(string(activeOut))
	}

	for _, id := range ids {
		// Obtener título
		titleCmd := exec.CommandContext(ctx, "xdotool", "getwindowname", id)
		titleOut, err := titleCmd.Output()
		if err != nil {
			continue
		}
		title := strings.TrimSpace(string(titleOut))

		// Obtener clase
		classCmd := exec.CommandContext(ctx, "xprop", "-id", id, "WM_CLASS")
		classOut, _ := classCmd.Output()
		class := extractXpropClass(string(classOut))

		windows = append(windows, WindowInfoBasic{
			ID:      id,
			Address: id,
			Title:   title,
			Class:   class,
			Focused: id == activeID,
		})
	}
	return windows, nil
}

// extractXpropClass extrae la clase WM de la salida de xprop.
func extractXpropClass(xpropOutput string) string {
	// Formato: WM_CLASS(STRING) = "instance", "ClassName"
	parts := strings.SplitN(xpropOutput, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	classes := strings.Split(parts[1], ",")
	if len(classes) >= 2 {
		// Tomar la segunda parte (ClassName)
		return strings.Trim(strings.TrimSpace(classes[1]), `"`)
	}
	return strings.Trim(strings.TrimSpace(parts[1]), `"`)
}

// defaultWindowFocusFunc enfoca una ventana por su dirección.
func defaultWindowFocusFunc(ctx context.Context, address string) error {
	// Intentar hyprctl primero
	if _, err := exec.LookPath("hyprctl"); err == nil {
		cmd := exec.CommandContext(ctx, "hyprctl", "dispatch", "focuswindow", "address:"+address)
		return cmd.Run()
	}
	// Fallback a xdotool
	if _, err := exec.LookPath("xdotool"); err == nil {
		cmd := exec.CommandContext(ctx, "xdotool", "windowfocus", "--sync", address)
		return cmd.Run()
	}
	return nil
}

// BrowserMatch contiene información sobre la coincidencia de una ventana de navegador.
type BrowserMatch struct {
	Window     WindowInfoBasic
	Confidence float64
	Reason     string
}

// knownBrowserClasses contiene las clases de ventana de navegadores conocidos.
var knownBrowserClasses = []string{
	"brave-browser", "brave", "BraveBrowser",
	"firefox", "Firefox",
	"google-chrome", "google-chrome-stable", "Google-chrome",
	"chromium", "Chromium",
	"vivaldi", "Vivaldi",
	"microsoft-edge", "Microsoft-edge",
	"opera", "Opera",
}

// isBrowserClass comprueba si una clase de ventana corresponde a un navegador conocido.
func isBrowserClass(class string) bool {
	classLower := strings.ToLower(class)
	for _, b := range knownBrowserClasses {
		if strings.ToLower(b) == classLower || strings.Contains(classLower, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

// FindBestBrowserWindow busca la ventana de navegador que mejor coincide con el objetivo.
func FindBestBrowserWindow(
	listWindows WindowListFunc,
	targetTitle string,
	targetClass string,
) (*BrowserMatch, error) {
	ctx := context.Background()
	windows, err := listWindows(ctx)
	if err != nil {
		return nil, err
	}

	if len(windows) == 0 {
		return nil, nil
	}

	var best *BrowserMatch
	targetLower := strings.ToLower(targetTitle)

	for _, w := range windows {
		score := 0.0
		reasons := []string{}

		classLower := strings.ToLower(w.Class)
		titleLower := strings.ToLower(w.Title)

		// +0.50 si la clase es un navegador conocido
		if isBrowserClass(w.Class) {
			score += 0.50
			reasons = append(reasons, "clase de navegador conocido")
		} else if targetClass != "" && strings.Contains(classLower, strings.ToLower(targetClass)) {
			score += 0.50
			reasons = append(reasons, "clase coincide con objetivo")
		}

		// +0.40 si el título contiene el objetivo
		if targetLower != "" && strings.Contains(titleLower, targetLower) {
			score += 0.40
			reasons = append(reasons, "título contiene objetivo")
		}

		// +0.20 si la ventana está enfocada
		if w.Focused {
			score += 0.20
			reasons = append(reasons, "ventana activa")
		}

		// -0.30 si el título es una pestaña vacía pero el objetivo es específico
		if targetLower != "" {
			isNewTab := strings.Contains(titleLower, "nueva pestaña") || strings.Contains(titleLower, "new tab")
			if isNewTab {
				score -= 0.30
				reasons = append(reasons, "pestaña vacía penalizada")
			} else if !strings.Contains(titleLower, targetLower) {
				// Penalización fuerte si no es la pestaña que buscamos y no está vacía
				score -= 0.50
				reasons = append(reasons, "título no coincide con objetivo")
			}
		}

		if score > 0 {
			match := &BrowserMatch{
				Window:     w,
				Confidence: score,
				Reason:     strings.Join(reasons, ", "),
			}
			if best == nil || match.Confidence > best.Confidence {
				best = match
			}
		}
	}

	return best, nil
}
