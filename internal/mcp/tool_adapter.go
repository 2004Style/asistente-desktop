package mcp

import (
	"context"
	"database/sql"
	"time"

	"rbot/internal/executor"
)

// MCPToolAdapter adapta una herramienta dinámica expuesta por un servidor MCP
// para que cumpla con la interfaz executor.ToolHandler.
type MCPToolAdapter struct {
	Client   *Client
	Def      ToolDefinition
	DB       *sql.DB
	FullName string
}

// NewMCPToolAdapter crea un nuevo adaptador para una herramienta MCP con su nombre completo prefijado.
func NewMCPToolAdapter(client *Client, def ToolDefinition, db *sql.DB, fullName string) *MCPToolAdapter {
	return &MCPToolAdapter{
		Client:   client,
		Def:      def,
		DB:       db,
		FullName: fullName,
	}
}

// Name devuelve el nombre completo adaptado (ej: mcp__server__name).
func (a *MCPToolAdapter) Name() string {
	return a.FullName
}

// Description devuelve la descripción de la herramienta MCP.
func (a *MCPToolAdapter) Description() string {
	return a.Def.Description
}

// Category devuelve la categoría de herramientas MCP.
func (a *MCPToolAdapter) Category() string {
	return "mcp"
}

// RiskLevel consulta el nivel de riesgo en la base de datos o por defecto retorna "medium".
func (a *MCPToolAdapter) RiskLevel() string {
	if a.DB == nil {
		return "medium"
	}
	var risk string
	// Buscamos por el nombre adaptado (mcp__name) y por el nombre original en caso de compatibilidad
	query := `
		SELECT risk_level FROM mcp_tools 
		WHERE name = ? OR name = ? 
		LIMIT 1;
	`
	err := a.DB.QueryRow(query, a.Name(), a.Def.Name).Scan(&risk)
	if err != nil || risk == "" {
		return "medium"
	}
	return risk
}

// Schema devuelve el esquema de parámetros de la herramienta.
func (a *MCPToolAdapter) Schema() map[string]interface{} {
	return a.Def.InputSchema
}

// Execute realiza la llamada remota al servidor MCP para ejecutar la herramienta.
func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	started := time.Now()

	// Llamar a la herramienta remota con su nombre original (sin prefijo mcp__)
	output, err := a.Client.CallTool(ctx, a.Def.Name, args)
	if err != nil {
		return &executor.ToolResult{
			Success:    false,
			Error:      err.Error(),
			StartedAt:  started,
			FinishedAt: time.Now(),
		}, nil
	}

	return &executor.ToolResult{
		Success:    true,
		Text:       output,
		StartedAt:  started,
		FinishedAt: time.Now(),
	}, nil
}
