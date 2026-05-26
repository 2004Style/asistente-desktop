package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
	"rbot/internal/runtime"
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
		log.Fatalf("Error al cargar la configuración: %v", err)
	}

	// Obtener rutas de sockets y directorios
	socketPath := db.ExpandPath(conf.Runtime.SocketPath)
	eventSocketPath := db.ExpandPath(conf.Runtime.EventSocketPath)

	// 1. Detección de Instancia Única y limpieza de socket stale
	if err := runtime.CheckInstanceLock(socketPath); err != nil {
		log.Fatalf("Error de exclusión de instancia: %v", err)
	}

	// Inicializar Base de Datos
	sqlitePath := db.ExpandPath(conf.Database.Path)
	database, err := db.InitDB(sqlitePath)
	if err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}

	// Instanciar daemon
	daemon := runtime.NewDaemon(conf, database)

	// Contexto de cancelación para signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Arrancar el daemon
	if err := daemon.Start(ctx); err != nil {
		log.Fatalf("Error al iniciar el daemon: %v", err)
	}

	// Iniciar servidores de sockets Unix (IPC)
	err = ipc.StartIPCServer(ctx, socketPath, func(req ipc.RequestJSONRPC) ipc.ResponseJSONRPC {
		var params map[string]interface{}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return ipc.NewErrorResponseJSONRPC(req.ID, ipc.InvalidParamsCode, "Error deserializando parámetros params: "+err.Error(), nil)
			}
		}
		
		result, err := daemon.HandleCommand(req.Method, params)
		if err != nil {
			return ipc.NewErrorResponseJSONRPC(req.ID, ipc.InternalErrorCode, err.Error(), nil)
		}
		return ipc.NewResponseJSONRPC(req.ID, result)
	})
	if err != nil {
		daemon.Stop()
		log.Fatalf("Error al arrancar el servidor IPC de comandos: %v", err)
	}

	err = ipc.StartEventIPCServer(ctx, eventSocketPath, daemon.EventBus)
	if err != nil {
		daemon.Stop()
		log.Fatalf("Error al arrancar el servidor IPC de eventos: %v", err)
	}

	log.Printf("[Daemon] RBotd persistente iniciado.")
	log.Printf("[Daemon] Socket de comandos: %s (0600)", socketPath)
	log.Printf("[Daemon] Socket de eventos:  %s (0600)", eventSocketPath)
	log.Println("[Daemon] Escuchando peticiones...")

	// Esperar señal del sistema para apagado limpio
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigs:
		log.Printf("[Daemon] Señal de terminación recibida (%v). Apagando...", sig)
	case <-ctx.Done():
	}

	daemon.Stop()
	log.Println("[Daemon] RBotd detenido con éxito.")
}
