package desktop

import (
	"fmt"
	"os/exec"
)

// LaunchApplication lanza un comando de aplicación de manera desacoplada del agente.
// Si hyprctl está disponible (Hyprland), lo usa para que la ventana se dibuje correctamente.
func LaunchApplication(command string) error {
	if command == "" {
		return fmt.Errorf("comando de ejecución vacío")
	}

	// Comprobar si hyprctl está disponible en el PATH (específico de Hyprland)
	if _, err := exec.LookPath("hyprctl"); err == nil {
		cmd := exec.Command("hyprctl", "dispatch", "exec", command)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Fallback estándar en Linux
	cmd := exec.Command("bash", "-c", command+" &")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al lanzar aplicación en segundo plano: %v", err)
	}

	return nil
}

// OpenURL abre una dirección URL en el navegador por defecto o en los binarios de navegador conocidos
// para evitar problemas de asociaciones mime erróneas (ej: que xdg-open abra AnyDesk u otra app).
func OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("url vacía")
	}

	// Lista de navegadores conocidos por orden de preferencia
	browsers := []string{
		"google-chrome-stable",
		"google-chrome",
		"firefox",
		"brave-browser",
		"brave",
		"chromium",
	}

	for _, b := range browsers {
		if path, err := exec.LookPath(b); err == nil {
			cmd := exec.Command(path, url)
			if err := cmd.Start(); err == nil {
				return nil
			}
		}
	}

	if _, err := exec.LookPath("xdg-open"); err != nil {
		return fmt.Errorf("xdg-open no disponible en el sistema")
	}

	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al abrir URL con xdg-open: %v", err)
	}

	return nil
}
