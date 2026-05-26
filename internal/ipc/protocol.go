package ipc

import (
	"encoding/json"
)

// RequestJSONRPC representa una solicitud estructurada según el estándar JSON-RPC 2.0
type RequestJSONRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"` // string, number o null
}

// ErrorJSONRPC representa la sección de error dentro de la respuesta JSON-RPC 2.0
type ErrorJSONRPC struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ResponseJSONRPC representa la respuesta estándar según el protocolo JSON-RPC 2.0
type ResponseJSONRPC struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *ErrorJSONRPC `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

// Códigos de error estándar definidos por la especificación de JSON-RPC 2.0
const (
	ParseErrorCode     = -32700
	InvalidRequestCode = -32600
	MethodNotFoundCode = -32601
	InvalidParamsCode  = -32602
	InternalErrorCode  = -32603
)

// NewErrorJSONRPC instancia una estructura de error
func NewErrorJSONRPC(code int, message string, data interface{}) *ErrorJSONRPC {
	return &ErrorJSONRPC{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewResponseJSONRPC crea una respuesta de éxito en el estándar JSON-RPC 2.0
func NewResponseJSONRPC(id interface{}, result interface{}) ResponseJSONRPC {
	return ResponseJSONRPC{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

// NewErrorResponseJSONRPC crea una respuesta fallida en el estándar JSON-RPC 2.0
func NewErrorResponseJSONRPC(id interface{}, code int, message string, data interface{}) ResponseJSONRPC {
	return ResponseJSONRPC{
		JSONRPC: "2.0",
		Error:   NewErrorJSONRPC(code, message, data),
		ID:      id,
	}
}
