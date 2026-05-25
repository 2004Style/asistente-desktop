package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type HyprlandClient struct {
	Address   string `json:"address"`
	Class     string `json:"class"`
	Title     string `json:"title"`
	Workspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Pid int `json:"pid"`
}

// GetActiveClients retorna la lista de clientes (ventanas) actualmente abiertas en Hyprland
func GetActiveClients(ctx context.Context) ([]HyprlandClient, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "clients", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error ejecutando hyprctl: %w", err)
	}

	var clients []HyprlandClient
	if err := json.Unmarshal(out, &clients); err != nil {
		return nil, fmt.Errorf("error parseando JSON de hyprctl: %w", err)
	}
	return clients, nil
}

// FindBrowserWithTab busca si hay un navegador abierto y si su título coincide con un hint
func FindBrowserWithTab(ctx context.Context, titleHint string) (*HyprlandClient, error) {
	clients, err := GetActiveClients(ctx)
	if err != nil {
		return nil, err
	}

	titleHint = strings.ToLower(titleHint)
	for _, c := range clients {
		class := strings.ToLower(c.Class)
		if strings.Contains(class, "brave") || strings.Contains(class, "firefox") || strings.Contains(class, "chrome") || strings.Contains(class, "vivaldi") || strings.Contains(class, "edge") {
			if titleHint == "" || strings.Contains(strings.ToLower(c.Title), titleHint) {
				return &c, nil
			}
		}
	}
	return nil, nil
}

// FocusWindow cambia el foco a una ventana específica usando su address en Hyprland
func FocusWindow(ctx context.Context, address string) error {
	cmd := exec.CommandContext(ctx, "hyprctl", "dispatch", "focuswindow", "address:"+address)
	return cmd.Run()
}
