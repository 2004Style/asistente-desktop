package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Estructuras para JSON-RPC 2.0
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorDetail    `json:"error,omitempty"`
}

type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolDefinition representa una herramienta expuesta por un servidor MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Client representa un cliente individual conectado a un servidor MCP por stdio
type Client struct {
	Name       string
	Command    string
	Args       []string
	Cmd        *exec.Cmd
	Stdin      io.WriteCloser
	IsActive   bool
	activeMu   sync.RWMutex
	pending    map[int64]chan *Response
	pendingMu  sync.Mutex
	reqCounter int64
	writeMu    sync.Mutex
}

// NewClient crea una instancia de un cliente MCP
func NewClient(name string, command string, args []string) *Client {
	return &Client{
		Name:    name,
		Command: command,
		Args:    args,
		pending: make(map[int64]chan *Response),
	}
}

// Start inicia el subproceso del servidor y los canales de escucha
func (c *Client) Start() error {
	c.activeMu.Lock()
	if c.IsActive {
		c.activeMu.Unlock()
		return fmt.Errorf("el cliente ya está activo")
	}

	log.Printf("[MCP Client '%s'] Iniciando %s con argumentos %v", c.Name, c.Command, c.Args)

	cmd := exec.Command(c.Command, c.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.activeMu.Unlock()
		return fmt.Errorf("error al abrir stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		c.activeMu.Unlock()
		return fmt.Errorf("error al abrir stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		c.activeMu.Unlock()
		return fmt.Errorf("error al abrir stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		c.activeMu.Unlock()
		return fmt.Errorf("error al iniciar comando: %v", err)
	}

	c.Cmd = cmd
	c.Stdin = stdin
	c.IsActive = true

	// Escuchar stdout
	go c.readLoop(stdout)
	// Escuchar stderr para evitar bloqueos
	go c.stderrLoop(stderr)

	// Liberamos el bloqueo de escritura antes del handshake para evitar interbloqueo (deadlock)
	// con c.activeMu.RLock() en sendRequestWithContext/sendNotification
	c.activeMu.Unlock()

	// Realizar handshake
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "rbot-go-mcp",
			"version": "1.0.0",
		},
	}

	_, err = c.sendRequestWithContext(ctx, "initialize", initParams)
	if err != nil {
		c.Stop()
		return fmt.Errorf("fallo en handshake 'initialize': %v", err)
	}

	// Enviar notificación de inicialización
	err = c.sendNotification("notifications/initialized", map[string]interface{}{})
	if err != nil {
		c.Stop()
		return fmt.Errorf("fallo en notificación 'initialized': %v", err)
	}

	log.Printf("[MCP Client '%s'] Inicializado con éxito.", c.Name)
	return nil
}

// Stop apaga el subproceso ordenadamente
func (c *Client) Stop() {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	c.stopUnsafe()
}

func (c *Client) stopUnsafe() {
	if !c.IsActive {
		return
	}
	c.IsActive = false

	if c.Stdin != nil {
		c.Stdin.Close()
	}

	if c.Cmd != nil && c.Cmd.Process != nil {
		// Matar inmediatamente sin esperar — no bloquea el programa
		_ = c.Cmd.Process.Kill()
		go func() { _ = c.Cmd.Wait() }()
	}

	c.pendingMu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[int64]chan *Response)
	c.pendingMu.Unlock()
}

func (c *Client) readLoop(stdout io.ReadCloser) {
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		c.activeMu.RLock()
		active := c.IsActive
		c.activeMu.RUnlock()
		if !active {
			break
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Si no es un JSON-RPC válido, podría ser un log de stdout
			log.Printf("[%s STDOUT LOG] %s", c.Name, string(line))
			continue
		}

		// Enrutar respuesta a la petición pendiente
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			ch <- &resp
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("[MCP Client '%s'] Error en scanner stdout: %v", c.Name, err)
	}
	c.Stop()
}

func (c *Client) stderrLoop(stderr io.ReadCloser) {
	defer stderr.Close()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		log.Printf("[%s STDERR LOG] %s", c.Name, scanner.Text())
	}
}

func (c *Client) sendRequestWithContext(ctx context.Context, method string, params interface{}) (*Response, error) {
	c.activeMu.RLock()
	active := c.IsActive
	c.activeMu.RUnlock()
	if !active {
		return nil, fmt.Errorf("cliente MCP inactivo")
	}

	id := atomic.AddInt64(&c.reqCounter, 1)
	req := Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	c.writeMu.Lock()
	if c.Stdin != nil {
		_, err = c.Stdin.Write(append(data, '\n'))
	} else {
		err = fmt.Errorf("stdin cerrado")
	}
	c.writeMu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("cliente apagado durante la petición")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp error (código %d): %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *Client) sendNotification(method string, params interface{}) error {
	c.activeMu.RLock()
	active := c.IsActive
	c.activeMu.RUnlock()
	if !active {
		return fmt.Errorf("cliente MCP inactivo")
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.Stdin == nil {
		return fmt.Errorf("stdin cerrado")
	}

	_, err = c.Stdin.Write(append(data, '\n'))
	return err
}

// ListTools recupera las herramientas expuestas por el servidor
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	resp, err := c.sendRequestWithContext(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("error al decodificar list tools result: %v", err)
	}

	return result.Tools, nil
}

// CallTool ejecuta una herramienta con argumentos dados
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	}

	resp, err := c.sendRequestWithContext(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("error al decodificar call tool result: %v", err)
	}

	var output string
	for _, item := range result.Content {
		if item.Type == "text" {
			output += item.Text + "\n"
		}
	}

	if result.IsError {
		return "", fmt.Errorf("el servidor MCP reportó un error:\n%s", output)
	}

	return output, nil
}

// ServerManager coordina y administra múltiples servidores MCP activos
type ServerManager struct {
	Clients map[string]*Client
	Mu      sync.Mutex
}

// NewServerManager crea una instancia del ServerManager
func NewServerManager() *ServerManager {
	return &ServerManager{
		Clients: make(map[string]*Client),
	}
}

// Bootstrap levanta los servidores configurados en el JSON indicado
func (s *ServerManager) Bootstrap(configPath string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Leer archivo de configuración
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Sin configuración inicial
		}
		return fmt.Errorf("error al leer config MCP: %v", err)
	}

	var config struct {
		McpServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("error de formato JSON en config MCP: %v", err)
	}

	for name, srv := range config.McpServers {
		// Expandir ~ en los argumentos para que sea portable
		expandedArgs := make([]string, len(srv.Args))
		home, _ := os.UserHomeDir()
		for i, arg := range srv.Args {
			if strings.HasPrefix(arg, "~") && home != "" {
				expandedArgs[i] = filepath.Join(home, arg[1:])
			} else {
				expandedArgs[i] = arg
			}
		}

		client := NewClient(name, srv.Command, expandedArgs)
		if err := client.Start(); err != nil {
			log.Printf("[MCP Manager] No se pudo arrancar el servidor '%s': %v", name, err)
			continue
		}
		s.Clients[name] = client
	}

	return nil
}

// CloseAll detiene todos los servidores activos
func (s *ServerManager) CloseAll() {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	for _, client := range s.Clients {
		client.Stop()
	}
	s.Clients = make(map[string]*Client)
}
