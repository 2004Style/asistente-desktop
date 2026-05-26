package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/hud"
)

func main() {
	// Definir ruta del config rbot.yaml (Modo Producción / XDG Base Directory)
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")

	// Modo Desarrollo: prioritario si está en la carpeta raíz
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		configPath = "config/rbot.yaml"
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error al cargar la configuración en el HUD: %v", err)
	}

	if !conf.Hud.Enabled {
		log.Println("HUD desactivado en la configuración. Saliendo...")
		return
	}

	eventSocketPath := db.ExpandPath(conf.Hud.EventSocketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capturar señales para apagado limpio
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("Señal recibida en HUD (%v). Apagando...", sig)
		cancel()
	}()

	log.Printf("Iniciando RBot HUD...")
	log.Printf("Socket de Eventos: %s", eventSocketPath)

	renderer := hud.NewRenderer(conf)
	renderer.Start(ctx, eventSocketPath)

	log.Println("RBot HUD detenido con éxito.")
}
