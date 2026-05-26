package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
)

// StartIPCServer inicia el servidor Unix Domain Socket para recibir comandos usando el protocolo JSON-RPC 2.0.
// Se configuran permisos de usuario estrictos: directorio a 0700 y archivo de socket a 0600.
func StartIPCServer(ctx context.Context, socketPath string, handler func(req RequestJSONRPC) ResponseJSONRPC) error {
	dir := filepath.Dir(socketPath)
	// Crear el directorio contenedor con permisos estrictos (0700: sólo lectura/escritura/ejecución para el propietario)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error al crear el directorio runtime %s: %w", dir, err)
	}

	// Eliminar el archivo del socket anterior si quedó huérfano
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error al limpiar socket stale %s: %w", socketPath, err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("error al escuchar en el socket unix %s: %w", socketPath, err)
	}

	// Configurar permisos del archivo del socket a 0600 (sólo lectura/escritura para el propietario)
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("error al establecer permisos restrictivos en el socket %s: %w", socketPath, err)
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
					log.Printf("[IPC Server] Error al aceptar conexión: %v", err)
					continue
				}
			}

			go handleConnection(conn, handler)
		}
	}()

	return nil
}

func handleConnection(conn net.Conn, handler func(req RequestJSONRPC) ResponseJSONRPC) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	data, err := reader.ReadBytes('\n')
	if err != nil {
		writeErrorResponse(conn, nil, ParseErrorCode, "Error de parseo leyendo del socket", err.Error())
		return
	}

	var req RequestJSONRPC
	if err := json.Unmarshal(data, &req); err != nil {
		writeErrorResponse(conn, nil, ParseErrorCode, "JSON mal formado", err.Error())
		return
	}

	// Validar conformidad básica con el protocolo JSON-RPC 2.0
	if req.JSONRPC != "2.0" {
		writeErrorResponse(conn, req.ID, InvalidRequestCode, "Solicitud inválida: Se requiere 'jsonrpc' con valor '2.0'", nil)
		return
	}

	if req.Method == "" {
		writeErrorResponse(conn, req.ID, InvalidRequestCode, "Solicitud inválida: Se requiere el campo 'method'", nil)
		return
	}

	// Invocar el handler del negocio y escribir la respuesta obtenida
	res := handler(req)
	writeJSONResponse(conn, res)
}

func writeJSONResponse(conn net.Conn, res ResponseJSONRPC) {
	data, err := json.Marshal(res)
	if err != nil {
		log.Printf("[IPC Server] Error al serializar respuesta JSON-RPC: %v", err)
		return
	}
	_, _ = conn.Write(append(data, '\n'))
}

func writeErrorResponse(conn net.Conn, id interface{}, code int, message string, data interface{}) {
	res := NewErrorResponseJSONRPC(id, code, message, data)
	writeJSONResponse(conn, res)
}
