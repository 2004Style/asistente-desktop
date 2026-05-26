package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

// SendCommandRPC envía una solicitud estructurada según el protocolo JSON-RPC 2.0
// al daemon a través de su socket Unix y devuelve la respuesta recibida.
func SendCommandRPC(socketPath string, method string, params interface{}, id interface{}) (ResponseJSONRPC, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return ResponseJSONRPC{}, fmt.Errorf("no se pudo conectar al daemon en %s: %w (¿está corriendo el servicio?)", socketPath, err)
	}
	defer conn.Close()

	var rawParams json.RawMessage
	if params != nil {
		rawBytes, err := json.Marshal(params)
		if err != nil {
			return ResponseJSONRPC{}, fmt.Errorf("error serializando parámetros de la petición: %w", err)
		}
		rawParams = rawBytes
	}

	req := RequestJSONRPC{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
		ID:      id,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return ResponseJSONRPC{}, fmt.Errorf("error serializando estructura JSON-RPC: %w", err)
	}

	// Enviar petición delimitada por salto de línea
	_, err = conn.Write(append(payload, '\n'))
	if err != nil {
		return ResponseJSONRPC{}, fmt.Errorf("error enviando bytes al socket: %w", err)
	}

	reader := bufio.NewReader(conn)
	respBytes, err := reader.ReadBytes('\n')
	if err != nil {
		return ResponseJSONRPC{}, fmt.Errorf("error leyendo respuesta desde el socket: %w", err)
	}

	var res ResponseJSONRPC
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return ResponseJSONRPC{}, fmt.Errorf("error de estructura al decodificar respuesta JSON-RPC: %w", err)
	}

	return res, nil
}
