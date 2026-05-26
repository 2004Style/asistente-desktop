package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"rbot/internal/runtime"
)

// StartEventIPCServer inicia el servidor de socket Unix para streaming de eventos en formato NDJSON.
// Se aplican permisos restrictivos de usuario: directorio a 0700 y socket a 0600.
func StartEventIPCServer(ctx context.Context, socketPath string, bus *runtime.EventBus) error {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error al crear directorio runtime para eventos %s: %w", dir, err)
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al limpiar socket de eventos stale %s: %w", socketPath, err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("error al abrir socket de eventos Unix %s: %w", socketPath, err)
	}

	// Configurar permisos del archivo del socket a 0600 (sólo legible/escribible por el propietario)
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("error al establecer permisos restrictivos en socket de eventos %s: %w", socketPath, err)
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					log.Printf("[Event IPC] Error al aceptar conexión de cliente: %v", err)
					continue
				}
			}

			go handleEventClient(ctx, conn, bus)
		}
	}()

	return nil
}

func handleEventClient(ctx context.Context, conn net.Conn, bus *runtime.EventBus) {
	defer conn.Close()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Detectar si el cliente cierra el canal leyendo un byte nulo
	disconnect := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		_, err := conn.Read(buf)
		if err != nil {
			close(disconnect)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-disconnect:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			// Escribir en formato NDJSON
			_, err = conn.Write(append(data, '\n'))
			if err != nil {
				return
			}
		}
	}
}
