package agent

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rbot/internal/desktop"
	"rbot/internal/files"
	"rbot/internal/intent"
	"rbot/internal/mcp"
	"rbot/internal/ollama"
	"rbot/internal/personality"
	"rbot/internal/security"
	"rbot/internal/skills"
)

type DirectAction struct {
	ToolName string
	Args     map[string]interface{}
}

type Orchestrator struct {
	DB           *sql.DB
	Ollama       *ollama.Client
	MCP          *mcp.ServerManager
	BlockedPaths []string
	AllowedRoots []string
	AgentName    string
	IsVoiceMode  bool
}

func NewOrchestrator(db *sql.DB, ollamaClient *ollama.Client, mcpManager *mcp.ServerManager, blockedPaths []string, allowedRoots []string, agentName string) *Orchestrator {
	if agentName == "" {
		agentName = "RBot"
	}
	return &Orchestrator{
		DB:           db,
		Ollama:       ollamaClient,
		MCP:          mcpManager,
		BlockedPaths: blockedPaths,
		AllowedRoots: allowedRoots,
		AgentName:    agentName,
	}
}

// BuildSystemPrompt genera el prompt con memoria de usuario, habilidades/skills activas y metadatos del sistema en tiempo real
func (o *Orchestrator) BuildSystemPrompt(skillContexts []string) string {
	var memoryParts []string
	// Limitar a top 5 memorias recientes/relevantes
	rows, err := o.DB.Query("SELECT category, key, value FROM user_memory ORDER BY updated_at DESC LIMIT 5")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat, key, val string
			if err := rows.Scan(&cat, &key, &val); err == nil {
				memoryParts = append(memoryParts, fmt.Sprintf("- [%s] %s: %s", cat, key, val))
			}
		}
	}

	memoryStr := "No hay datos recordados relevantes."
	if len(memoryParts) > 0 {
		memoryStr = strings.Join(memoryParts, "\n")
	}

	// 1. Obtener fecha y hora actual
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 2. Obtener lista de aplicaciones disponibles limitadas
	var appsParts []string
	appRows, err := o.DB.Query("SELECT display_name, executable FROM app_launchers WHERE is_available = 1 LIMIT 15")
	if err == nil {
		defer appRows.Close()
		for appRows.Next() {
			var dispName, execName string
			if err := appRows.Scan(&dispName, &execName); err == nil {
				appsParts = append(appsParts, fmt.Sprintf("- %s (%s)", dispName, execName))
			}
		}
	}
	appsStr := "No hay aplicaciones de escritorio indexadas."
	if len(appsParts) > 0 {
		appsStr = strings.Join(appsParts, "\n")
	}

	skillsSection := ""
	if len(skillContexts) > 0 {
		skillsSection = fmt.Sprintf("\n[HABILIDADES / SKILLS RELEVANTES]\nSigue fielmente las siguientes instrucciones para procesar la orden:\n%s\n", strings.Join(skillContexts, "\n"))
	}

	return fmt.Sprintf(`Eres el asistente personal de escritorio de %s. Operas en Linux y ayudas a controlar aplicaciones, ventanas, workspaces, navegador, música, archivos, terminal, procesos, servicios, Docker, proyectos de desarrollo y búsquedas de información.

Tu personalidad está inspirada en un mayordomo tecnológico elegante: sereno, preciso, discreto, técnico, confiable y con humor seco muy ocasional.

Tu objetivo no es solo ejecutar órdenes, sino operar con criterio.

Reglas de comportamiento:
1. Observa antes de actuar.
2. Reutiliza ventanas, pestañas, procesos y rutas existentes cuando sea posible.
3. Evita duplicar aplicaciones, navegadores, terminales o pestañas.
4. Verifica el resultado después de actuar.
5. Si una acción es destructiva o riesgosa, pide confirmación.
6. Si el usuario es ambiguo, resuelve con contexto o muestra candidatos.
7. Si no puedes verificar algo, dilo claramente.
8. No uses sudo sin permiso.
9. No borres, sobrescribas, muevas en masa, mates procesos, reinicies servicios ni instales paquetes sin confirmación.
10. No muestres secretos, tokens, claves privadas ni valores sensibles.

Estilo de respuesta:
- Acciones simples: breve y seguro.
- Diagnósticos: claro y técnico.
- Errores: sereno y útil.
- Riesgos: cauteloso.
- Finalización: elegante y corta.
- Usa lenguaje natural en español sin revelar URLs crudas ni rutas absolutas.

Frases base (ejemplos a seguir):
- "Entendido. Procediendo."
- "Estoy revisando el entorno."
- "He localizado el objetivo."
- "No recomiendo continuar sin verificar esto primero."
- "No realizaré cambios destructivos sin confirmación."
- "Operación completada."
- "Todo quedó en orden."
- "He encontrado un inconveniente. Tengo una posible causa."

Evita:
- "Comando recibido."
- "Sistema inicializado."
- "Ejecutando protocolo."
- Frases excesivamente robóticas.
- Bromas constantes.
- Confirmaciones innecesarias para acciones simples.

[HABILIDADES / SKILLS RELEVANTES]
%s

[METADATOS DEL SISTEMA (TIEMPO REAL)]
- Fecha y hora actual: %s
- Aplicaciones comunes en el sistema:
%s

[DATOS RECORDADOS DEL USUARIO]
%s
`, "2004Style", skillsSection, currentTime, appsStr, memoryStr)
}

// GetAvailableTools consolida herramientas internas y MCP en formato compatible con Ollama
func (o *Orchestrator) GetAvailableTools(ctx context.Context) []ollama.Tool {
	var list []ollama.Tool

	// 1. Herramientas internas
	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "desktop.open_app",
			Description: "Abre una aplicación o programa instalado en el escritorio del sistema (ej: firefox, brave, chrome, code, spotify, etc.).",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"app": map[string]interface{}{
						"type":        "string",
						"description": "Nombre del programa o aplicación a abrir.",
					},
				},
				Required: []string{"app"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "desktop.close_app",
			Description: "Cierra o termina una aplicación o programa en ejecución (ej: firefox, chrome, code, nautilus, spotify, etc.).",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"app": map[string]interface{}{
						"type":        "string",
						"description": "Nombre del programa o aplicación a cerrar.",
					},
				},
				Required: []string{"app"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "browser.open_url",
			Description: "Abre un sitio web en el navegador por defecto del sistema (ej: youtube.com, whatsapp.com, google.com).",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Sitio web o dirección URL exacta a abrir. DEBE ser un dominio válido (ej: youtube.com) o URL completa (ej: https://github.com), NUNCA frases, espacios ni contexto natural.",
					},
				},
				Required: []string{"url"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "browser.read_url",
			Description: "Lee y extrae el contenido de texto de una página web o dirección URL para poder resumirla o responder preguntas sobre ella.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "La dirección URL del sitio web a leer.",
					},
				},
				Required: []string{"url"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.search_index",
			Description: "Busca la ruta física de un archivo o carpeta en el disco.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Nombre del archivo o carpeta a buscar.",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.read_file",
			Description: "Lee el contenido de un archivo de texto o lista los archivos de un directorio/carpeta del sistema.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta o nombre del archivo o carpeta a leer.",
					},
				},
				Required: []string{"path"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "memory.remember",
			Description: "Guarda información sobre el usuario (preferencias, gustos, nombre) en la base de datos.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Clave del dato a recordar.",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Valor del dato.",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Categoría del dato.",
					},
				},
				Required: []string{"key", "value", "category"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "browser.search",
			Description: "Realiza una búsqueda en internet en el navegador web por defecto del sistema.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Texto o consulta a buscar en internet.",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "browser.youtube_search",
			Description: "Busca videos en YouTube en el navegador web por defecto del sistema.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Texto o consulta a buscar en YouTube.",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "browser.youtube_play",
			Description: "Busca y reproduce un video o música en YouTube abriendo la página del video directamente en el navegador.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Nombre de la canción, artista o video a reproducir en YouTube.",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "desktop.open_folder",
			Description: "Abre una carpeta/directorio del sistema en Visual Studio Code o en el explorador de archivos Nautilus.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta de la carpeta (ej: Descargas, Documentos, o ruta absoluta) a abrir.",
					},
					"app": map[string]interface{}{
						"type":        "string",
						"description": "Aplicación con la que abrir la carpeta: 'vscode' (para Visual Studio Code) o 'nautilus' (para explorador de archivos).",
						"enum":        []string{"vscode", "nautilus"},
					},
				},
				Required: []string{"path"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.create_file",
			Description: "Crea un nuevo archivo de texto con el contenido especificado.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta relativa o absoluta del archivo a crear.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Contenido textual que se escribirá en el archivo.",
					},
				},
				Required: []string{"path", "content"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.delete_file",
			Description: "Elimina permanentemente un archivo o directorio especificado en el sistema.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta relativa o absoluta del archivo o directorio a eliminar.",
					},
				},
				Required: []string{"path"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.list_directory",
			Description: "Lista el contenido detallado de un directorio.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta relativa o absoluta del directorio a listar.",
					},
				},
				Required: []string{"path"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "files.create_directory",
			Description: "Crea una nueva carpeta o directorio en la ruta especificada.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Ruta del directorio a crear.",
					},
				},
				Required: []string{"path"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "system.datetime",
			Description: "Obtiene la fecha y hora actual en tiempo real.",
			Parameters: ollama.Parameters{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "system.clipboard_copy",
			Description: "Copia un texto especificado al portapapeles del sistema.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Texto a copiar al portapapeles.",
					},
				},
				Required: []string{"text"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "system.notify",
			Description: "Envía una notificación visual de escritorio.",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Título de la notificación.",
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Mensaje de la notificación.",
					},
				},
				Required: []string{"title", "message"},
			},
		},
	})

	list = append(list, ollama.Tool{
		Type: "function",
		Function: ollama.FunctionDefinition{
			Name:        "system.run_command",
			Description: "Ejecuta un comando en la terminal de Linux y retorna el resultado (salida estándar).",
			Parameters: ollama.Parameters{
				Type: "object",
				Properties: map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Comando bash a ejecutar.",
					},
				},
				Required: []string{"command"},
			},
		},
	})

	// 2. Herramientas MCP
	o.MCP.Mu.Lock()
	defer o.MCP.Mu.Unlock()
	for serverName, client := range o.MCP.Clients {
		if !client.IsActive {
			continue
		}
		tools, err := client.ListTools(ctx)
		if err != nil {
			log.Printf("[MCP] Error al listar herramientas para %s: %v", serverName, err)
			continue
		}

		for _, t := range tools {
			prefixedName := fmt.Sprintf("mcp__%s__%s", serverName, t.Name)
			list = append(list, ollama.Tool{
				Type: "function",
				Function: ollama.FunctionDefinition{
					Name:        prefixedName,
					Description: fmt.Sprintf("[MCP %s] %s", serverName, t.Description),
					Parameters: ollama.Parameters{
						Type:       "object",
						Properties: t.InputSchema["properties"].(map[string]interface{}),
						Required:   interfaceSliceToStringSlice(t.InputSchema["required"]),
					},
				},
			})
		}
	}

	return list
}

func interfaceSliceToStringSlice(i interface{}) []string {
	if i == nil {
		return nil
	}
	slice, ok := i.([]interface{})
	if !ok {
		return nil
	}
	var res []string
	for _, v := range slice {
		if s, ok := v.(string); ok {
			res = append(res, s)
		}
	}
	return res
}

// Chat realiza un paso conversacional resolviendo llamadas a herramientas si Ollama las requiere.
func (o *Orchestrator) Chat(ctx context.Context, userInput string, history []ollama.Message) (string, error) {
	// Cargar skills activas que coincidan con la entrada del usuario mediante IntentRouter
	router := intent.NewRouter(o.DB)
	candidates := router.Match(userInput)

	var skillContexts []string
	if len(candidates) > 0 {
		topCandidates := intent.TopN(candidates, 3)
		log.Printf("[Orchestrator] Cargando %d habilidad(es) top para el prompt:", len(topCandidates))
		for _, s := range topCandidates {
			log.Printf(" - %s (Confianza: %.2f)", s.SkillName, s.Confidence)
			body, err := skills.LoadSkillBody(o.DB, s.SkillName)
			if err == nil {
				skillContexts = append(skillContexts, fmt.Sprintf("Habilidad: %s\n%s", s.SkillName, body))
			}
		}
	}

	// 1. Detectar intenciones directas y ejecutar inmediatamente si corresponde para evitar fallos del LLM
	directActions := o.detectDirectIntents(userInput)
	if len(directActions) > 0 {
		log.Printf("[Orchestrator] Se detectaron %d acciones directas para '%s'. Saltando Ollama.", len(directActions), userInput)

		var results []string
		var lastErr error

		for _, action := range directActions {
			toolName := action.ToolName
			args := action.Args

			// Validar seguridad
			var targetPath string
			if p, ok := args["path"].(string); ok {
				targetPath = p
			}

			allowed, requiresConfirm, reason := security.ValidateToolAction(o.DB, toolName, targetPath, o.BlockedPaths)
			if toolName == "system.run_command" {
				if cmdStr, ok := args["command"].(string); ok {
					if security.IsCommandCritical(cmdStr) {
						requiresConfirm = true
						reason = fmt.Sprintf("el comando '%s' es crítico y requiere confirmación explícita", cmdStr)
					}
				}
			}

			var resultStr string
			var execErr error
			var success int = 0
			var confirmedByUser int = 0

			if !allowed {
				resultStr = fmt.Sprintf("Error: Acción denegada por seguridad. %s", reason)
				execErr = fmt.Errorf("denegado: %s", reason)
			} else {
				if requiresConfirm {
					confirmedByUser = 1
					if o.IsVoiceMode {
						if o.hasUserConfirmedConversational(history, userInput) {
							resultStr, execErr = o.executeTool(ctx, toolName, args)
						} else {
							resultStr = fmt.Sprintf("Error: Confirmación conversacional requerida para '%s'.", toolName)
							execErr = fmt.Errorf("confirmación conversacional requerida: %s", reason)
							success = 0
						}
					} else {
						if o.askConfirmationInteractive(toolName, fmt.Sprintf("%v", args), reason) {
							resultStr, execErr = o.executeTool(ctx, toolName, args)
						} else {
							resultStr = "Error: Acción cancelada por el usuario."
							execErr = fmt.Errorf("cancelado por el usuario")
							success = 0
						}
					}
				} else {
					resultStr, execErr = o.executeTool(ctx, toolName, args)
				}
			}

			if execErr == nil {
				success = 1
			} else {
				lastErr = execErr
				if resultStr == "" {
					resultStr = fmt.Sprintf("Error al ejecutar herramienta: %v", execErr)
				}
			}

			// Loguear acción
			var source string = "internal"
			if strings.HasPrefix(toolName, "mcp__") {
				source = "mcp"
			}
			argsBytes, _ := json.Marshal(args)
			_, _ = o.DB.Exec(`
				INSERT INTO action_log (user_input, tool_name, tool_source, arguments_json, result_json, success, error, required_confirmation, confirmed_by_user, duration_ms)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
			`, userInput, toolName, source, string(argsBytes), resultStr, success, fmt.Sprintf("%v", execErr), boolToInt(requiresConfirm), confirmedByUser, 0)

			results = append(results, fmt.Sprintf("Acción %s: %s", toolName, resultStr))
		}

		// Generar confirmación conversacional inmediata para acciones mecánicas y rápidas
		// Esto evita llamadas lentas al LLM (Ollama) de 10-15 segundos solo para confirmar la ejecución.
		var confirmationText string
		if lastErr != nil {
			state := personality.StateError
			if strings.Contains(lastErr.Error(), "confirmación conversacional requerida") || strings.Contains(lastErr.Error(), "cancelado por el usuario") {
				state = personality.StateConfirming
			}
			confirmationText = personality.ComposeResponse(personality.ResponseContext{
				State: state,
				Error: lastErr,
				AgentName: o.AgentName,
			})
			return confirmationText, nil
		}

		if len(directActions) == 1 {
			act := directActions[0]
			var target string
			if app, ok := act.Args["app"].(string); ok {
				target = filepath.Base(app)
			} else if urlArg, ok := act.Args["url"].(string); ok {
				target = urlArg
			}

			confirmationText = personality.ComposeResponse(personality.ResponseContext{
				State:    personality.StateDone,
				Risk:     personality.RiskLow,
				ToolName: act.ToolName,
				Target:   target,
				AgentName: o.AgentName,
			})
		} else {
			confirmationText = personality.ComposeResponse(personality.ResponseContext{
				State:    personality.StateDone,
				Risk:     personality.RiskLow,
				AgentName: o.AgentName,
			})
		}

		return confirmationText, nil
	}

	// Construir historial con el mensaje de sistema y el nuevo prompt
	var messages []ollama.Message
	messages = append(messages, ollama.Message{
		Role:    "system",
		Content: o.BuildSystemPrompt(skillContexts),
	})

	// Cargar historial previo
	messages = append(messages, history...)

	// Agregar entrada del usuario
	messages = append(messages, ollama.Message{
		Role:    "user",
		Content: userInput,
	})

	// Obtener herramientas
	tools := o.GetAvailableTools(ctx)

	log.Printf("[Orchestrator] Iniciando ciclo de llamadas a Ollama...")
	start := time.Now()

	maxSteps := 5
	for step := 0; step < maxSteps; step++ {
		log.Printf("[Orchestrator] Enviando chat a Ollama (Paso %d/%d)...", step+1, maxSteps)
		respMessage, err := o.Ollama.Chat(messages, tools)
		if err != nil {
			return "", err
		}

		// Si no requiere tool calls, retornar la respuesta de texto final del modelo
		if len(respMessage.ToolCalls) == 0 {
			return respMessage.Content, nil
		}

		// Procesar cada llamada a herramienta en este paso
		var toolResults []string
		for _, tc := range respMessage.ToolCalls {
			toolName := tc.Function.Name
			args := tc.Function.Arguments
			argsJSON, _ := json.Marshal(args)

			log.Printf("[Orchestrator] Ollama solicitó ejecutar herramienta: %s con args %s", toolName, string(argsJSON))

			// Validar seguridad
			var targetPath string
			if p, ok := args["path"].(string); ok {
				targetPath = p
			}

			allowed, requiresConfirm, reason := security.ValidateToolAction(o.DB, toolName, targetPath, o.BlockedPaths)
			if toolName == "system.run_command" {
				if cmdStr, ok := args["command"].(string); ok {
					if security.IsCommandCritical(cmdStr) {
						requiresConfirm = true
						reason = fmt.Sprintf("el comando '%s' es crítico y requiere confirmación explícita", cmdStr)
					}
				}
			}

			var resultStr string
			var execErr error
			var success int = 0
			var confirmedByUser int = 0

			if !allowed {
				resultStr = fmt.Sprintf("Error: Acción denegada por seguridad. %s", reason)
				execErr = fmt.Errorf("denegado: %s", reason)
			} else {
				if requiresConfirm {
					confirmedByUser = 1
					if o.IsVoiceMode {
						if o.hasUserConfirmedConversational(messages, userInput) {
							resultStr, execErr = o.executeTool(ctx, toolName, args)
						} else {
							resultStr = fmt.Sprintf("Error: Confirmación conversacional requerida para '%s'.", toolName)
							execErr = fmt.Errorf("confirmación conversacional requerida: %s", reason)
							success = 0
						}
					} else {
						if o.askConfirmationInteractive(toolName, string(argsJSON), reason) {
							resultStr, execErr = o.executeTool(ctx, toolName, args)
						} else {
							resultStr = "Error: Acción cancelada por el usuario."
							execErr = fmt.Errorf("cancelado por el usuario")
							success = 0
						}
					}
				} else {
					resultStr, execErr = o.executeTool(ctx, toolName, args)
				}
			}

			if execErr == nil {
				success = 1
			} else {
				resultStr = fmt.Sprintf("Error al ejecutar herramienta: %v", execErr)
			}

			// Loguear acción en base de datos
			var source string = "internal"
			if strings.HasPrefix(toolName, "mcp__") {
				source = "mcp"
			}
			duration := time.Since(start).Milliseconds()
			_, _ = o.DB.Exec(`
				INSERT INTO action_log (user_input, tool_name, tool_source, arguments_json, result_json, success, error, required_confirmation, confirmed_by_user, duration_ms)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
			`, userInput, toolName, source, string(argsJSON), resultStr, success, fmt.Sprintf("%v", execErr), boolToInt(requiresConfirm), confirmedByUser, duration)

			toolResults = append(toolResults, fmt.Sprintf("Herramienta '%s' ejecutada. Resultado: %s", toolName, resultStr))
		}

		// Enviar los resultados de vuelta como contexto
		resultSummary := strings.Join(toolResults, "\n")

		// Añadir la respuesta del asistente (que invocó herramientas) al historial
		messages = append(messages, *respMessage)

		// Añadir los resultados de las herramientas en un mensaje de usuario para la siguiente iteración
		messages = append(messages, ollama.Message{
			Role:    "user",
			Content: fmt.Sprintf("System notification: The tools were executed. Results:\n%s\nProcesa estos resultados y decide si debes llamar a otra herramienta o dar una respuesta conversacional final en español al usuario (tratándolo de 'señor' y de forma directa/corta).", resultSummary),
		})
	}

	return "Lo siento señor, he superado el límite de pasos permitidos para esta solicitud.", nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (o *Orchestrator) askConfirmationInteractive(toolName, argsJSON, reason string) bool {
	fmt.Printf("\n⚠️ [CONFIRMACIÓN REQUERIDA] %s\n", reason)
	fmt.Printf("Herramienta: %s\nArgumentos: %s\n", toolName, argsJSON)
	fmt.Print("¿Deseas permitir esta acción? (s/n): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "s" || input == "si" || input == "y" || input == "yes"
}

func (o *Orchestrator) executeTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if strings.HasPrefix(toolName, "mcp__") {
		// Enrutar a MCP
		partes := strings.SplitN(strings.TrimPrefix(toolName, "mcp__"), "__", 2)
		if len(partes) < 2 {
			return "", fmt.Errorf("nombre de herramienta MCP inválido: %s", toolName)
		}
		serverName := partes[0]
		realToolName := partes[1]

		o.MCP.Mu.Lock()
		client, ok := o.MCP.Clients[serverName]
		o.MCP.Mu.Unlock()

		if !ok || !client.IsActive {
			return "", fmt.Errorf("el servidor MCP '%s' no está activo", serverName)
		}

		return client.CallTool(ctx, realToolName, args)
	}

	// Herramientas internas
	switch toolName {
	case "desktop.open_app":
		app, _ := args["app"].(string)
		if app == "" {
			return "", fmt.Errorf("argumento 'app' requerido")
		}

		appLower := strings.ToLower(strings.TrimSpace(app))

		// Usar coincidencia inteligente para obtener el comando ejecutable real
		var command string
		if matchedApp, ok := o.findBestAppMatch(appLower); ok {
			if matchedApp == "navegador" {
				err := desktop.OpenURL("https://google.com")
				if err != nil {
					return "", err
				}
				return "Navegador predeterminado abierto.", nil
			}

			// Buscar el comando asociado en la base de datos
			err := o.DB.QueryRow("SELECT command FROM app_launchers WHERE executable = ? OR name = ? LIMIT 1", matchedApp, matchedApp).Scan(&command)
			if err != nil {
				command = matchedApp
			}
			app = matchedApp
		} else {
			command = app
		}

		// Validar si el ejecutable del comando existe en el sistema
		firstWord := strings.Fields(command)[0]
		if _, err := exec.LookPath(firstWord); err != nil {
			if info, statErr := os.Stat(firstWord); statErr != nil || info.IsDir() {
				return "", fmt.Errorf("no se encontró la aplicación o programa '%s' en el sistema", app)
			}
		}

		err := desktop.LaunchApplication(command)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Aplicación '%s' lanzada correctamente.", app), nil

	case "desktop.close_app":
		app, _ := args["app"].(string)
		if app == "" {
			return "", fmt.Errorf("argumento 'app' requerido")
		}

		appLower := strings.ToLower(strings.TrimSpace(app))
		if appLower == "navegador" || appLower == "chrome" || appLower == "google-chrome" || appLower == "firefox" {
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:google-chrome").Run()
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:firefox").Run()
			_ = exec.Command("pkill", "-x", "chrome").Run()
			_ = exec.Command("pkill", "-x", "google-chrome").Run()
			_ = exec.Command("pkill", "-x", "firefox").Run()
			return "Navegador web cerrado.", nil
		}

		if appLower == "vscode" || appLower == "code" || appLower == "visual studio code" {
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:Code").Run()
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:code").Run()
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:code-oss").Run()
			_ = exec.Command("pkill", "-x", "code").Run()
			return "Visual Studio Code cerrado.", nil
		}

		if appLower == "nautilus" || appLower == "gestor de archivos" || appLower == "archivos" {
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:nautilus").Run()
			_ = exec.Command("pkill", "-x", "nautilus").Run()
			return "Gestor de archivos cerrado.", nil
		}

		if matchedApp, ok := o.findBestAppMatch(appLower); ok {
			_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:"+matchedApp).Run()
			_ = exec.Command("pkill", "-x", matchedApp).Run()
			_ = exec.Command("pkill", "-f", matchedApp).Run()
			return fmt.Sprintf("Aplicación '%s' cerrada.", matchedApp), nil
		}

		_ = exec.Command("hyprctl", "dispatch", "closewindow", "class:"+app).Run()
		_ = exec.Command("pkill", "-x", app).Run()
		return fmt.Sprintf("Aplicación '%s' cerrada.", app), nil

	case "browser.search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("argumento 'query' requerido")
		}
		targetURL := fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(query))
		err := desktop.OpenURL(targetURL)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Buscando '%s' en internet.", query), nil

	case "browser.youtube_search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("argumento 'query' requerido")
		}
		targetURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))
		err := desktop.OpenURL(targetURL)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Buscando '%s' en YouTube.", query), nil

	case "browser.youtube_play":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("argumento 'query' requerido")
		}
		targetURL := getFirstYouTubeVideo(query)
		err := desktop.OpenURL(targetURL)
		if err != nil {
			return "", err
		}
		if strings.Contains(targetURL, "/watch?v=") {
			return fmt.Sprintf("Reproduciendo '%s' en YouTube.", query), nil
		}
		return fmt.Sprintf("Abriendo búsqueda de '%s' en YouTube.", query), nil

	case "desktop.open_folder":
		path, _ := args["path"].(string)
		app, _ := args["app"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, false)
		if err != nil {
			return "", err
		}

		appLower := strings.ToLower(strings.TrimSpace(app))
		if appLower == "vscode" || appLower == "code" {
			err = desktop.LaunchApplication("code " + resolvedPath)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Carpeta '%s' abierta en VS Code.", resolvedPath), nil
		}

		err = desktop.LaunchApplication("nautilus " + resolvedPath)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Carpeta '%s' abierta en el explorador de archivos.", resolvedPath), nil

	case "browser.open_url":
		urlVal, ok := args["url"].(string)
		if !ok {
			return "", fmt.Errorf("argumento 'url' faltante o inválido para browser.open_url")
		}
		
		// Limpiar URL si el LLM metió espacios o texto natural
		if strings.Contains(urlVal, " ") {
			// Intentar extraer solo la parte que parece un dominio/url (la primera palabra válida)
			parts := strings.Split(urlVal, " ")
			for _, p := range parts {
				if strings.Contains(p, ".") || strings.HasPrefix(p, "http") {
					urlVal = p
					break
				}
			}
			// Si todavía tiene espacios, tomar solo la primera palabra
			if strings.Contains(urlVal, " ") {
				urlVal = strings.Split(urlVal, " ")[0]
			}
		}

		if !strings.HasPrefix(urlVal, "http://") && !strings.HasPrefix(urlVal, "https://") {
			if strings.Contains(urlVal, "whatsapp") {
				urlVal = "https://web.whatsapp.com"
			} else {
				urlVal = "https://" + urlVal
			}
		}

		err := desktop.OpenURL(urlVal)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Navegador abierto en la URL: %s", urlVal), nil

	case "browser.read_url":
		targetURL, _ := args["url"].(string)
		if targetURL == "" {
			return "", fmt.Errorf("argumento 'url' requerido")
		}

		if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
			targetURL = "https://" + targetURL
		}

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			return "", fmt.Errorf("error al crear petición: %v", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("error al conectar con la página web: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("la página devolvió un código de estado error: %d", resp.StatusCode)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error al leer contenido: %v", err)
		}

		htmlStr := string(bodyBytes)
		reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
		htmlStr = reScript.ReplaceAllString(htmlStr, "")
		reStyle := regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
		htmlStr = reStyle.ReplaceAllString(htmlStr, "")
		reTags := regexp.MustCompile(`<.*?>`)
		textStr := reTags.ReplaceAllString(htmlStr, " ")
		reSpaces := regexp.MustCompile(`\s+`)
		textStr = reSpaces.ReplaceAllString(textStr, " ")
		textStr = strings.TrimSpace(textStr)
		if len(textStr) > 4000 {
			textStr = textStr[:4000] + "... [Contenido truncado]"
		}

		if textStr == "" {
			return "No se pudo extraer texto legible de la página.", nil
		}
		return textStr, nil

	case "files.search_index":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("argumento 'query' requerido")
		}

		path, err := files.FindFileOrDirectory(o.DB, query, o.AllowedRoots, o.BlockedPaths)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Encontrado en: %s", path), nil

	case "files.read_file":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, false)
		if err != nil {
			return "", err
		}
		path = resolvedPath

		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}

		// Si es un directorio, en lugar de error listamos su contenido para que Ollama haga un resumen
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", fmt.Errorf("error al leer el directorio: %v", err)
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Directorio: %s\nContenido:\n", path))
			for _, entry := range entries {
				entryType := "Archivo"
				if entry.IsDir() {
					entryType = "Carpeta"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", entryType, entry.Name()))
			}
			return sb.String(), nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}

		// Limitar el contenido leído para no saturar contexto
		text := string(content)
		if len(text) > 4000 {
			text = text[:4000] + "\n... (contenido truncado por longitud)"
		}
		return text, nil

	case "files.create_file":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, true)
		if err != nil {
			return "", err
		}

		err = os.MkdirAll(filepath.Dir(resolvedPath), 0755)
		if err != nil {
			return "", fmt.Errorf("error al crear directorios padres: %v", err)
		}

		err = os.WriteFile(resolvedPath, []byte(content), 0644)
		if err != nil {
			return "", fmt.Errorf("error al escribir el archivo: %v", err)
		}
		return fmt.Sprintf("Archivo '%s' creado correctamente con %d bytes.", resolvedPath, len(content)), nil

	case "files.delete_file":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, false)
		if err != nil {
			return "", err
		}

		err = os.RemoveAll(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("error al eliminar: %v", err)
		}
		return fmt.Sprintf("Archivo o directorio '%s' eliminado correctamente del sistema.", resolvedPath), nil

	case "files.list_directory":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, false)
		if err != nil {
			return "", err
		}

		entries, err := os.ReadDir(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("error al leer el directorio: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Directorio: %s\nContenido:\n", resolvedPath))
		for _, entry := range entries {
			entryType := "Archivo"
			if entry.IsDir() {
				entryType = "Carpeta"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", entryType, entry.Name()))
		}
		return sb.String(), nil

	case "files.create_directory":
		path, _ := args["path"].(string)
		if path == "" {
			return "", fmt.Errorf("argumento 'path' requerido")
		}

		resolvedPath, err := o.resolvePathSmart(path, true)
		if err != nil {
			return "", err
		}

		err = os.MkdirAll(resolvedPath, 0755)
		if err != nil {
			return "", fmt.Errorf("error al crear directorio: %v", err)
		}
		return fmt.Sprintf("Directorio '%s' creado correctamente.", resolvedPath), nil

	case "system.datetime":
		return fmt.Sprintf("Fecha y hora actual: %s", time.Now().Format("2006-01-02 15:04:05")), nil

	case "system.clipboard_copy":
		text, _ := args["text"].(string)
		if text == "" {
			return "", fmt.Errorf("argumento 'text' requerido")
		}

		var cmd *exec.Cmd
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			return "", fmt.Errorf("no se encontró ningún comando de portapapeles ('wl-copy' o 'xclip')")
		}

		cmd.Stdin = strings.NewReader(text)
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("error al copiar al portapapeles: %v", err)
		}
		return "Texto copiado al portapapeles correctamente.", nil

	case "system.notify":
		title, _ := args["title"].(string)
		message, _ := args["message"].(string)
		if title == "" || message == "" {
			return "", fmt.Errorf("argumentos 'title' y 'message' requeridos")
		}

		if _, err := exec.LookPath("notify-send"); err != nil {
			return "", fmt.Errorf("notify-send no instalado")
		}

		cmd := exec.Command("notify-send", title, message)
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("error al enviar notificación: %v", err)
		}
		return "Notificación de escritorio enviada.", nil

	case "system.run_command":
		command, _ := args["command"].(string)
		if command == "" {
			return "", fmt.Errorf("argumento 'command' requerido")
		}

		cmd := exec.Command("bash", "-c", command)
		output, err := cmd.CombinedOutput()
		outStr := string(output)
		if err != nil {
			return fmt.Sprintf("Comando ejecutado con código de error (%v). Salida:\n%s", err, outStr), nil
		}
		if outStr == "" {
			outStr = "(comando ejecutado sin salida en consola)"
		}
		return outStr, nil

	case "memory.remember":
		key, _ := args["key"].(string)
		val, _ := args["value"].(string)
		cat, _ := args["category"].(string)

		if key == "" || val == "" || cat == "" {
			return "", fmt.Errorf("argumentos 'key', 'value' y 'category' requeridos")
		}

		query := `
		INSERT INTO user_memory (key, value, category, source)
		VALUES (?, ?, ?, 'conversation')
		ON CONFLICT(key, category) DO UPDATE SET
			value = excluded.value,
			updated_at = datetime('now');
		`
		_, err := o.DB.Exec(query, strings.ToLower(key), val, strings.ToLower(cat))
		if err != nil {
			return "", fmt.Errorf("error al guardar en memoria: %v", err)
		}

		return fmt.Sprintf("He recordado que tu %s (%s) es: %s", key, cat, val), nil
	}

	return "", fmt.Errorf("herramienta desconocida: %s", toolName)
}

// detectDirectIntents intenta clasificar la entrada del usuario en una o más acciones directas
// para evitar que el LLM cometa fallos o haga preguntas innecesarias.
func (o *Orchestrator) detectDirectIntents(userInput string) []DirectAction {
	inputLower := strings.ToLower(strings.TrimSpace(userInput))
	inputLower = normalizeSpelling(inputLower)
	// Quitar puntuación común
	inputLower = strings.TrimFunc(inputLower, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == '¿' || r == '¡' || r == ';' || r == ':'
	})

	// --- FASE 3: Detección temprana de intenciones que DEBEN pasar al LLM ---
	// Frases de lectura/resumen/contenido de archivos necesitan que el LLM lea Y responda.
	// NUNCA deben ser interceptadas por el detector directo.
	isFileReadOrSummary := false
	fileReadKeywords := []string{
		"resumen", "resume", "resumir", "resumeme", "resúmeme",
		"contenido de", "contenido del",
		"lee el archivo", "leer el archivo", "leeme el archivo", "léeme el archivo",
		"lee el contenido", "leer el contenido",
		"dime qué dice", "dime que dice", "qué dice el archivo", "que dice el archivo",
		"dame un resumen", "hazme un resumen",
	}
	for _, kw := range fileReadKeywords {
		if strings.Contains(inputLower, kw) {
			isFileReadOrSummary = true
			break
		}
	}
	if isFileReadOrSummary {
		log.Printf("[Orchestrator] Frase de lectura/resumen detectada ('%s'). Delegando al LLM.\n", userInput)
		return nil
	}

	// Simplificar saludos iniciales o conectores comunes
	prefixes := []string{
		"por favor ", "puedes ", "hola ", "rbot ", "ronald ", "oye ", "ey ",
		"necesito que ", "quiero que ", "hazme el favor de ",
		"quiero escuchar ", "quiero oír ", "quiero oir ", "me gustaría escuchar ", "me gustaria escuchar ",
		"colocame algo de ", "colócame algo de ", "ponme algo de ",
		"reprodúceme algo de ", "reproduceme algo de ", "reproduseme algo de ",
		"reprodúceme ", "reproduceme ", "reproduseme ",
		"colocame ", "colócame ", "ponme ", "colcoame ", "colcame ",
		"buscame ", "búscame ", "abreme ", "ábreme ", "ejecutame ", "ejecútame ",
		"muestrame ", "muéstrame ", "reproduce ", "reproducir ", "pon ", "poner ",
		"coloca ", "colocar ", "busca ", "buscar ", "abre ", "abrir ",
		"ejecuta ", "ejecutar ", "lanza ", "lanzar ",
		"la carpeta ", "el directorio ", "las carpetas ", "los directorios ",
		"el archivo ", "un archivo ", "los archivos ", "el programa ", "la aplicación ",
		"la aplicacion ", "un programa ", "una aplicación ", "una aplicacion ",
		"el sitio web de ", "la pagina de ", "la página de ", "la web de ", "el sitio ",
		"música de ", "musica de ", "canción de ", "cancion de ", "un video de ", "videos de ",
	}
	cleaned := inputLower
	for {
		changed := false
		for _, p := range prefixes {
			if strings.HasPrefix(cleaned, p) {
				cleaned = strings.TrimPrefix(cleaned, p)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	// Simplificar palabras comunes al final
	suffixes := []string{
		" por favor", " gracias", " ahora", " inmediatamente",
	}
	for {
		changed := false
		for _, s := range suffixes {
			if strings.HasSuffix(cleaned, s) {
				cleaned = strings.TrimSuffix(cleaned, s)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	// --- 1. Caso especial: comandos directos hablados o escritos "al pie de la letra"
	if strings.Contains(cleaned, "desktop.open_app") {
		app := extractArg(cleaned, "app")
		if app == "" {
			app = extractBetweenQuotesOrParens(cleaned)
		}
		if app != "" {
			return []DirectAction{{ToolName: "desktop.open_app", Args: map[string]interface{}{"app": app}}}
		}
	}
	if strings.Contains(cleaned, "browser.open_url") {
		url := extractArg(cleaned, "url")
		if url == "" {
			url = extractBetweenQuotesOrParens(cleaned)
		}
		if url != "" {
			return []DirectAction{{ToolName: "browser.open_url", Args: map[string]interface{}{"url": url}}}
		}
	}
	if strings.Contains(cleaned, "files.read_file") {
		path := extractArg(cleaned, "path")
		if path == "" {
			path = extractBetweenQuotesOrParens(cleaned)
		}
		if path != "" {
			return []DirectAction{{ToolName: "files.read_file", Args: map[string]interface{}{"path": path}}}
		}
	}
	if strings.Contains(cleaned, "files.search_index") {
		query := extractArg(cleaned, "query")
		if query == "" {
			query = extractBetweenQuotesOrParens(cleaned)
		}
		if query != "" {
			return []DirectAction{{ToolName: "files.search_index", Args: map[string]interface{}{"query": query}}}
		}
	}

	// --- 2. Detección de múltiples acciones
	// GUARD: No splitear frases que usan "y" como conector natural, no como separador de acciones
	isNaturalYConnector := false
	naturalYPatterns := []string{
		"y dame", "y hazme", "y dime", "y resume", "y resumeme", "y resúmeme",
		"y explica", "y explícame", "y cuéntame", "y cuentame",
	}
	for _, nyp := range naturalYPatterns {
		if strings.Contains(inputLower, nyp) {
			isNaturalYConnector = true
			break
		}
	}

	if !isNaturalYConnector && (strings.Contains(cleaned, " y ") || strings.Contains(cleaned, " e ")) {
		isCloseCommandPhrase := false
		for _, p := range []string{"cierra ", "cerrar ", "mata ", "matar ", "apaga ", "apagar ", "termina ", "terminar "} {
			if strings.HasPrefix(cleaned, p) {
				isCloseCommandPhrase = true
				break
			}
		}

		partsStr := regexp.MustCompile(`\s+e\s+`).ReplaceAllString(cleaned, " y ")
		parts := strings.Split(partsStr, " y ")

		var actions []DirectAction
		allMatched := true

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			partLower := strings.ToLower(part)
			partCleaned := partLower
			isPartClose := isCloseCommandPhrase

			for _, p := range []string{"cierra ", "cerrar ", "mata ", "matar ", "apaga ", "apagar ", "termina ", "terminar "} {
				if strings.HasPrefix(partCleaned, p) {
					partCleaned = strings.TrimPrefix(partCleaned, p)
					partCleaned = strings.TrimSpace(partCleaned)
					isPartClose = true
				}
			}

			// Intentar quitar prefijos de apertura/limpieza
			for _, p := range []string{
				"abre ", "abrir ", "lanza ", "lanzar ", "ejecuta ", "ejecutar ",
				"la carpeta ", "el directorio ", "el archivo ", "un archivo ",
				"el programa ", "la aplicación ", "la aplicacion ",
			} {
				if strings.HasPrefix(partCleaned, p) {
					partCleaned = strings.TrimPrefix(partCleaned, p)
					partCleaned = strings.TrimSpace(partCleaned)
				}
			}

			if !isPartClose {
				isWeb := false
				webSites := []string{"youtube", "whatsapp", "google", "facebook", "github", "gmail", "outlook", "netflix", "wikipedia", "chatgpt"}
				for _, ws := range webSites {
					if strings.Contains(partCleaned, ws) {
						isWeb = true
						break
					}
				}
				if strings.Contains(partCleaned, ".") || isWeb {
					actions = append(actions, DirectAction{
						ToolName: "browser.open_url",
						Args:     map[string]interface{}{"url": partCleaned},
					})
					continue
				}
			}

			// ¿Es una app? (Prioritario frente a carpetas)
			if matchedApp, ok := o.findBestAppMatch(partCleaned); ok {
				tool := "desktop.open_app"
				if isPartClose {
					tool = "desktop.close_app"
				}
				actions = append(actions, DirectAction{
					ToolName: tool,
					Args:     map[string]interface{}{"app": matchedApp},
				})
				continue
			}

			// ¿Es una carpeta o archivo en el disco? (solo si no es de cierre)
			if !isPartClose {
				isFolderExplicit := false
				cleanPart := partCleaned
				if strings.HasPrefix(partCleaned, "carpeta ") {
					cleanPart = strings.TrimPrefix(partCleaned, "carpeta ")
					isFolderExplicit = true
				} else if strings.HasPrefix(partCleaned, "directorio ") {
					cleanPart = strings.TrimPrefix(partCleaned, "directorio ")
					isFolderExplicit = true
				}
				cleanPart = strings.TrimSpace(cleanPart)

				if cleanPart != "" {
					if dirPath, ok := o.findDirectoryPath(cleanPart); ok {
						app := "nautilus"
						if strings.Contains(inputLower, "code") || strings.Contains(inputLower, "vscode") {
							app = "vscode"
						}
						actions = append(actions, DirectAction{
							ToolName: "desktop.open_folder",
							Args:     map[string]interface{}{"path": dirPath, "app": app},
						})
						continue
					}
					if isFolderExplicit {
						home, _ := os.UserHomeDir()
						if home != "" {
							testPath := filepath.Join(home, cleanPart)
							if info, err := os.Stat(testPath); err == nil && info.IsDir() {
								app := "nautilus"
								if strings.Contains(inputLower, "code") || strings.Contains(inputLower, "vscode") {
									app = "vscode"
								}
								actions = append(actions, DirectAction{
									ToolName: "desktop.open_folder",
									Args:     map[string]interface{}{"path": testPath, "app": app},
								})
								continue
							}
						}
					}
				}
			}

			// Si no coincide con nada pero es un cierre explícito genérico
			if isPartClose && partCleaned != "" {
				actions = append(actions, DirectAction{
					ToolName: "desktop.close_app",
					Args:     map[string]interface{}{"app": partCleaned},
				})
				continue
			}

			allMatched = false
			break
		}

		if allMatched && len(actions) > 0 {
			return actions
		}
	}

	// --- 3. Detección individual clásica optimizada
	// 3.1. YouTube Play / Search / Reproducción de Música
	isYouTubeOpenOnly := (cleaned == "youtube" || cleaned == "youtube.com") &&
		(strings.HasPrefix(inputLower, "abre") || strings.HasPrefix(inputLower, "abrir") || strings.HasPrefix(inputLower, "entra") || strings.HasPrefix(inputLower, "ir a"))

	isPlayMusicIntent := false
	playKeywords := []string{
		"reproduce", "reproducir", "pon ", "poner ", "coloca", "colocar", "escucha", "escuchar", "toca", "tocar",
		"colcoame", "colocame", "ponme", "reproduceme", "tócame", "tocame", "música", "musica", "cancion", "canción",
		"canciones", "musa", "phonk", "cumbia",
	}
	for _, kw := range playKeywords {
		if strings.Contains(inputLower, kw) {
			isPlayMusicIntent = true
			break
		}
	}

	if !isYouTubeOpenOnly && isPlayMusicIntent {
		// Determinar si es petición de música genérica
		isGenericMusicRequest := false
		genericMusicTerms := []string{
			"musica", "música", "algo de musica", "algo de música", "canciones", "cancion", "canción", "musa",
			"algo de musa", "algo", "algun tema", "algún tema", "alguna cancion", "alguna canción", "",
		}
		for _, term := range genericMusicTerms {
			if cleaned == term {
				isGenericMusicRequest = true
				break
			}
		}

		if isGenericMusicRequest {
			return []DirectAction{{ToolName: "browser.youtube_play", Args: map[string]interface{}{"query": "cumbia o bachata"}}}
		}

		// Extraer consulta limpia quitando sufijos de youtube
		query := cleaned
		for _, suf := range []string{" en youtube", " en el youtube", " en youtube.com", " youtube"} {
			if strings.HasSuffix(query, suf) {
				query = strings.TrimSuffix(query, suf)
				break
			}
		}
		for _, pref := range []string{"en youtube ", "en el youtube ", "en youtube.com "} {
			if strings.HasPrefix(query, pref) {
				query = strings.TrimPrefix(query, pref)
				break
			}
		}
		query = strings.TrimSpace(query)
		if query != "" {
			isSearch := strings.Contains(inputLower, "busca") || strings.Contains(inputLower, "buscar")
			if isSearch {
				return []DirectAction{{ToolName: "browser.youtube_search", Args: map[string]interface{}{"query": query}}}
			} else {
				return []DirectAction{{ToolName: "browser.youtube_play", Args: map[string]interface{}{"query": query}}}
			}
		}
	}

	// 3.2. Búsquedas generales en internet
	isWebSearch := false
	searchKeywords := []string{"en google", "en internet", "en el navegador", "en la web", "en duckduckgo"}
	for _, kw := range searchKeywords {
		if strings.Contains(inputLower, kw) {
			isWebSearch = true
			break
		}
	}
	if isWebSearch || strings.HasPrefix(inputLower, "busca ") || strings.HasPrefix(inputLower, "buscar ") || strings.HasPrefix(inputLower, "googlea ") {
		query := cleaned
		for _, suf := range []string{" en google", " en internet", " en el navegador", " en la web", " en duckduckgo"} {
			if strings.HasSuffix(query, suf) {
				query = strings.TrimSuffix(query, suf)
				break
			}
		}
		query = strings.TrimSpace(query)
		if query != "" && !strings.Contains(inputLower, "youtube") {
			return []DirectAction{{ToolName: "browser.search", Args: map[string]interface{}{"query": query}}}
		}
	}

	// 3.3. Intentar cerrar aplicaciones individuales
	isCloseCommandPhrase := false
	for _, p := range []string{"cierra ", "cerrar ", "mata ", "matar ", "apaga ", "apagar ", "termina ", "terminar "} {
		if strings.HasPrefix(inputLower, p) {
			isCloseCommandPhrase = true
			break
		}
	}
	if isCloseCommandPhrase {
		target := cleaned
		for _, p := range []string{"cierra el ", "cerrar el ", "cierra la ", "cerrar la ", "cierra un ", "cerrar un ", "cierra ", "cerrar ", "mata el ", "matar el ", "mata la ", "matar la ", "mata ", "matar "} {
			if strings.HasPrefix(target, p) {
				target = strings.TrimPrefix(target, p)
				break
			}
		}
		target = strings.TrimSpace(target)
		if target != "" {
			if target == "navegador" || target == "browser" || target == "internet" {
				return []DirectAction{{ToolName: "desktop.close_app", Args: map[string]interface{}{"app": "navegador"}}}
			}
			if matchedApp, ok := o.findBestAppMatch(target); ok {
				return []DirectAction{{ToolName: "desktop.close_app", Args: map[string]interface{}{"app": matchedApp}}}
			}
			return []DirectAction{{ToolName: "desktop.close_app", Args: map[string]interface{}{"app": target}}}
		}
	}

	// 3.4. Detección individual de URLs/Sitios Web
	isWeb := false
	webSites := []string{"youtube", "whatsapp", "google", "facebook", "github", "gmail", "outlook", "netflix", "wikipedia", "chatgpt"}
	for _, ws := range webSites {
		if strings.Contains(cleaned, ws) {
			isWeb = true
			break
		}
	}
	// Excluir extensiones de archivo comunes para no confundirlas con URLs
	isFileExtension := false
	fileExtensions := []string{".txt", ".pdf", ".md", ".go", ".py", ".json", ".yaml", ".yml",
		".csv", ".log", ".sh", ".html", ".css", ".js", ".ts", ".java", ".c", ".cpp",
		".h", ".rs", ".rb", ".php", ".xml", ".toml", ".conf", ".cfg", ".ini",
		".docx", ".xlsx", ".pptx", ".odt", ".ods", ".png", ".jpg", ".jpeg",
		".gif", ".svg", ".mp3", ".mp4", ".wav", ".flac", ".zip", ".tar", ".gz"}
	for _, ext := range fileExtensions {
		if strings.HasSuffix(cleaned, ext) {
			isFileExtension = true
			break
		}
	}
	hasDot := strings.Contains(cleaned, ".")
	if isWeb {
		return []DirectAction{{ToolName: "browser.open_url", Args: map[string]interface{}{"url": cleaned}}}
	}
	if hasDot && !isFileExtension {
		return []DirectAction{{ToolName: "browser.open_url", Args: map[string]interface{}{"url": cleaned}}}
	}

	// 3.5. Intentar abrir aplicaciones individuales (Prioridad 1)
	if matchedApp, ok := o.findBestAppMatch(cleaned); ok {
		return []DirectAction{{ToolName: "desktop.open_app", Args: map[string]interface{}{"app": matchedApp}}}
	}

	if cleaned == "navegador" || cleaned == "el navegador" || cleaned == "browser" || cleaned == "internet" {
		return []DirectAction{{ToolName: "desktop.open_app", Args: map[string]interface{}{"app": "navegador"}}}
	}

	// 3.6. Intentar abrir carpetas/directorios (Prioridad 2)
	isFolderExplicit := strings.HasPrefix(inputLower, "abre la carpeta") || strings.HasPrefix(inputLower, "abrir la carpeta") || strings.HasPrefix(inputLower, "carpeta ") || strings.HasPrefix(inputLower, "directorio ")
	cleanFolderTarget := cleaned
	if strings.HasPrefix(cleanFolderTarget, "carpeta ") {
		cleanFolderTarget = strings.TrimPrefix(cleanFolderTarget, "carpeta ")
	} else if strings.HasPrefix(cleanFolderTarget, "directorio ") {
		cleanFolderTarget = strings.TrimPrefix(cleanFolderTarget, "directorio ")
	}
	cleanFolderTarget = strings.TrimSpace(cleanFolderTarget)

	if cleanFolderTarget != "" {
		if dirPath, err := o.resolvePathSmart(cleanFolderTarget, false); err == nil {
			app := "nautilus"
			if strings.Contains(inputLower, "code") || strings.Contains(inputLower, "vscode") {
				app = "vscode"
			}
			return []DirectAction{{
				ToolName: "desktop.open_folder",
				Args: map[string]interface{}{
					"path": dirPath,
					"app":  app,
				},
			}}
		}

		if isFolderExplicit {
			home, _ := os.UserHomeDir()
			if home != "" {
				testPath := filepath.Join(home, cleanFolderTarget)
				if info, err := os.Stat(testPath); err == nil && info.IsDir() {
					app := "nautilus"
					if strings.Contains(inputLower, "code") || strings.Contains(inputLower, "vscode") {
						app = "vscode"
					}
					return []DirectAction{{
						ToolName: "desktop.open_folder",
						Args: map[string]interface{}{
							"path": testPath,
							"app":  app,
						},
					}}
				}
			}
		}
	}

	// 3.7. Lectura de archivos: ya se redirige al LLM en la FASE 3 al inicio.
	// Cualquier frase con "lee", "contenido de", "resumen" ya fue capturada arriba.
	// Solo queda el caso de lectura pura sin resumen (ya raro a este punto).

	// 3.8. Buscar archivos en el índice
	isFileSearchExplicit := strings.HasPrefix(inputLower, "busca ") || strings.HasPrefix(inputLower, "buscar ") || strings.HasPrefix(inputLower, "encuentra ") || strings.HasPrefix(inputLower, "encontrar ")
	if isFileSearchExplicit && strings.Contains(inputLower, "archivo") {
		cleanSearchTarget := cleaned
		for _, kw := range []string{"busca el archivo ", "buscar el archivo ", "busca ", "buscar ", "encuentra ", "encontrar "} {
			if strings.HasPrefix(cleanSearchTarget, kw) {
				cleanSearchTarget = strings.TrimPrefix(cleanSearchTarget, kw)
				break
			}
		}
		cleanSearchTarget = strings.TrimSpace(cleanSearchTarget)
		if cleanSearchTarget != "" {
			return []DirectAction{{ToolName: "files.search_index", Args: map[string]interface{}{"query": cleanSearchTarget}}}
		}
	}

	return nil
}

func extractArg(input, argName string) string {
	patterns := []string{
		argName + "='",
		argName + "=\"",
		argName + "=",
		argName + ":'",
		argName + ":\"",
		argName + ":",
	}
	for _, p := range patterns {
		if idx := strings.Index(input, p); idx != -1 {
			val := input[idx+len(p):]
			val = strings.TrimFunc(val, func(r rune) bool {
				return r == '\'' || r == '"' || r == ')' || r == '(' || r == ' ' || r == '}' || r == ']'
			})
			return val
		}
	}
	return ""
}

func extractBetweenQuotesOrParens(input string) string {
	firstQuote := strings.IndexAny(input, `'"(`)
	if firstQuote == -1 {
		return ""
	}
	closingChar := ""
	switch input[firstQuote] {
	case '\'':
		closingChar = "'"
	case '"':
		closingChar = `"`
	case '(':
		closingChar = ")"
	}
	remaining := input[firstQuote+1:]
	lastQuote := strings.Index(remaining, closingChar)
	if lastQuote == -1 {
		return strings.TrimSpace(remaining)
	}
	return strings.TrimSpace(remaining[:lastQuote])
}

func (o *Orchestrator) findDirectoryPath(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	var path string
	// 1. Intentar coincidencia exacta en la base de datos (con tipo directory)
	query := "SELECT path FROM path_entries WHERE name = ? AND type = 'directory' AND exists_now = 1 LIMIT 1"
	err := o.DB.QueryRow(query, name).Scan(&path)
	if err == nil {
		return path, true
	}

	// 2. Intentar coincidencia parcial por LIKE
	queryLike := "SELECT path FROM path_entries WHERE name LIKE ? AND type = 'directory' AND exists_now = 1 ORDER BY open_count DESC LIMIT 1"
	err = o.DB.QueryRow(queryLike, "%"+name+"%").Scan(&path)
	if err == nil {
		return path, true
	}

	// 3. Casos especiales comunes si no se ha indexado aún
	home, _ := os.UserHomeDir()
	if home != "" {
		targetLower := strings.ToLower(name)
		if targetLower == "descargas" {
			return filepath.Join(home, "Descargas"), true
		} else if targetLower == "documentos" {
			return filepath.Join(home, "Documentos"), true
		} else if targetLower == "escritorio" {
			return filepath.Join(home, "Escritorio"), true
		} else if targetLower == "imágenes" || targetLower == "imagenes" {
			return filepath.Join(home, "Imágenes"), true
		} else if targetLower == "música" || targetLower == "musica" {
			return filepath.Join(home, "Música"), true
		} else if targetLower == "vídeos" || targetLower == "videos" {
			return filepath.Join(home, "Vídeos"), true
		}
	}

	return "", false
}

func (o *Orchestrator) findFilePath(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	var path string
	// 1. Intentar coincidencia exacta en la base de datos (con tipo file)
	query := "SELECT path FROM path_entries WHERE name = ? AND type = 'file' AND exists_now = 1 LIMIT 1"
	err := o.DB.QueryRow(query, name).Scan(&path)
	if err == nil {
		return path, true
	}

	// 2. Intentar coincidencia parcial por LIKE
	queryLike := "SELECT path FROM path_entries WHERE name LIKE ? AND type = 'file' AND exists_now = 1 ORDER BY open_count DESC LIMIT 1"
	err = o.DB.QueryRow(queryLike, "%"+name+"%").Scan(&path)
	if err == nil {
		return path, true
	}

	return "", false
}

func (o *Orchestrator) findBestAppMatch(query string) (string, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "", false
	}

	// Normalizaciones específicas para VS Code
	if strings.Contains(query, "code") || strings.Contains(query, "vscode") || strings.Contains(query, "visual studio") || strings.Contains(query, "vusaul") {
		var execName string
		err := o.DB.QueryRow("SELECT executable FROM app_launchers WHERE name LIKE '%code%' OR name LIKE '%vscode%' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "code", true
	}

	// Normalización para navegadores
	if strings.Contains(query, "navegador") || strings.Contains(query, "browser") || strings.Contains(query, "internet") || query == "web" {
		return "navegador", true
	}

	// Normalización para gestor de archivos
	if strings.Contains(query, "gestor de archivos") || strings.Contains(query, "explorador de archivos") || query == "archivos" || query == "carpetas" {
		var execName string
		err := o.DB.QueryRow("SELECT executable FROM app_launchers WHERE name = 'files' OR name = 'nautilus' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "nautilus", true
	}

	// Normalización para terminal/consola
	if query == "consola" || query == "terminal" || query == "kitty" || query == "alacritty" || query == "konsole" || strings.Contains(query, "linea de comandos") {
		var execName string
		err := o.DB.QueryRow("SELECT executable FROM app_launchers WHERE name = 'kitty' OR name = 'gnome-terminal' OR name = 'konsole' OR name = 'terminal' LIMIT 1").Scan(&execName)
		if err == nil {
			return execName, true
		}
		return "kitty", true
	}

	// Obtener todas las aplicaciones de la base de datos
	rows, err := o.DB.Query("SELECT name, display_name, executable FROM app_launchers WHERE is_available = 1")
	if err != nil {
		return "", false
	}
	defer rows.Close()

	type appScore struct {
		executable string
		score      int
	}
	var bestMatch string
	var maxScore int = 0

	queryTokens := strings.Fields(query)

	for rows.Next() {
		var name, displayName, executable string
		if err := rows.Scan(&name, &displayName, &executable); err != nil {
			continue
		}

		nameLower := strings.ToLower(name)
		displayLower := strings.ToLower(displayName)
		execLower := strings.ToLower(executable)

		if nameLower == query || execLower == query {
			return executable, true
		}

		score := 0
		for _, token := range queryTokens {
			if token == "" || len(token) < 2 {
				continue
			}
			if strings.Contains(nameLower, token) {
				score += 3
			}
			if strings.Contains(displayLower, token) {
				score += 2
			}
			if strings.Contains(execLower, token) {
				score += 3
			}
		}

		if strings.Contains(displayLower, query) || strings.Contains(query, displayLower) {
			score += 5
		}
		if strings.Contains(nameLower, query) || strings.Contains(query, nameLower) {
			score += 5
		}

		if score > maxScore {
			maxScore = score
			bestMatch = executable
		}
	}

	if maxScore >= 5 {
		return bestMatch, true
	}

	return "", false
}

func getFirstYouTubeVideo(query string) string {
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", strings.ReplaceAll(query, " ", "+"))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return searchURL
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return searchURL
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return searchURL
	}

	bodyStr := string(bodyBytes)

	re := regexp.MustCompile(`/watch\?v=[a-zA-Z0-9_-]{11}`)
	match := re.FindString(bodyStr)
	if match != "" {
		return "https://www.youtube.com" + match
	}

	return searchURL
}

func (o *Orchestrator) resolveAmbiguityInteractive(query string, matches []string) (string, error) {
	if len(matches) == 1 {
		return matches[0], nil
	}

	if o.IsVoiceMode {
		var friendlyMatches []string
		for _, m := range matches {
			friendlyMatches = append(friendlyMatches, filepath.Base(m))
		}
		return "", fmt.Errorf("se encontraron múltiples coincidencias para '%s': %s. Por favor, sé más específico sobre cuál carpeta abrir", query, strings.Join(friendlyMatches, ", "))
	}

	fmt.Printf("\n🔍 Se encontraron múltiples coincidencias para '%s':\n", query)
	for i, match := range matches {
		fmt.Printf(" [%d] %s\n", i+1, match)
	}
	fmt.Print("Por favor, selecciona el número de la carpeta/archivo que deseas usar (o 'c' para cancelar): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error al leer la entrada: %v", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "c" || input == "cancelar" {
		return "", fmt.Errorf("acción cancelada por el usuario")
	}

	var index int
	_, err = fmt.Sscanf(input, "%d", &index)
	if err != nil || index < 1 || index > len(matches) {
		return "", fmt.Errorf("selección inválida")
	}

	return matches[index-1], nil
}

func (o *Orchestrator) resolvePathSmart(path string, isCreation bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("ruta vacía")
	}

	if filepath.IsAbs(path) {
		if security.IsPathBlocked(path, o.BlockedPaths) {
			return "", fmt.Errorf("ruta bloqueada por seguridad")
		}
		return path, nil
	}

	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		if home != "" {
			path = filepath.Join(home, path[1:])
		}
		if security.IsPathBlocked(path, o.BlockedPaths) {
			return "", fmt.Errorf("ruta bloqueada por seguridad")
		}
		return path, nil
	}

	cleaned := filepath.Clean(path)
	parts := strings.Split(cleaned, string(filepath.Separator))

	if len(parts) == 1 {
		if isCreation {
			home, _ := os.UserHomeDir()
			if home != "" {
				return filepath.Join(home, "Descargas", cleaned), nil
			}
			return cleaned, nil
		}
		matches, err := files.FindMultipleFilesOrDirectories(o.DB, cleaned, o.AllowedRoots, o.BlockedPaths)
		if err != nil {
			// Intentar carpetas especiales por defecto si no están indexadas
			home, _ := os.UserHomeDir()
			if home != "" {
				targetLower := strings.ToLower(cleaned)
				var specialPath string
				switch targetLower {
				case "descargas":
					specialPath = filepath.Join(home, "Descargas")
				case "documentos":
					specialPath = filepath.Join(home, "Documentos")
				case "escritorio":
					specialPath = filepath.Join(home, "Escritorio")
				case "imágenes", "imagenes":
					specialPath = filepath.Join(home, "Imágenes")
				case "música", "musica":
					specialPath = filepath.Join(home, "Música")
				case "vídeos", "videos":
					specialPath = filepath.Join(home, "Vídeos")
				}
				if specialPath != "" {
					return specialPath, nil
				}
			}
			return "", err
		}
		return o.resolveAmbiguityInteractive(cleaned, matches)
	}

	if isCreation {
		for i := len(parts) - 1; i > 0; i-- {
			parentRel := filepath.Join(parts[:i]...)
			matches, err := files.FindMultipleFilesOrDirectories(o.DB, parentRel, o.AllowedRoots, o.BlockedPaths)
			if err == nil && len(matches) > 0 {
				resolvedParent, err := o.resolveAmbiguityInteractive(parentRel, matches)
				if err == nil {
					rest := filepath.Join(parts[i:]...)
					resolved := filepath.Join(resolvedParent, rest)
					if security.IsPathBlocked(resolved, o.BlockedPaths) {
						return "", fmt.Errorf("ruta bloqueada por seguridad")
					}
					return resolved, nil
				}
			}
		}
		firstPart := parts[0]
		matches, err := files.FindMultipleFilesOrDirectories(o.DB, firstPart, o.AllowedRoots, o.BlockedPaths)
		if err == nil && len(matches) > 0 {
			resolvedParent, err := o.resolveAmbiguityInteractive(firstPart, matches)
			if err == nil {
				rest := filepath.Join(parts[1:]...)
				resolved := filepath.Join(resolvedParent, rest)
				if security.IsPathBlocked(resolved, o.BlockedPaths) {
					return "", fmt.Errorf("ruta bloqueada por seguridad")
				}
				return resolved, nil
			}
		}
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, "Descargas", cleaned), nil
		}
		return cleaned, nil
	} else {
		matches, err := files.FindMultipleFilesOrDirectories(o.DB, cleaned, o.AllowedRoots, o.BlockedPaths)
		if err == nil && len(matches) > 0 {
			return o.resolveAmbiguityInteractive(cleaned, matches)
		}
		for i := len(parts) - 1; i > 0; i-- {
			parentRel := filepath.Join(parts[:i]...)
			parentMatches, err := files.FindMultipleFilesOrDirectories(o.DB, parentRel, o.AllowedRoots, o.BlockedPaths)
			if err == nil && len(parentMatches) > 0 {
				resolvedParent, err := o.resolveAmbiguityInteractive(parentRel, parentMatches)
				if err == nil {
					rest := filepath.Join(parts[i:]...)
					resolved := filepath.Join(resolvedParent, rest)
					if _, err := os.Stat(resolved); err == nil {
						if security.IsPathBlocked(resolved, o.BlockedPaths) {
							return "", fmt.Errorf("ruta bloqueada por seguridad")
						}
						return resolved, nil
					}
				}
			}
		}
		return "", fmt.Errorf("no se encontró ningún archivo o carpeta con el nombre '%s'", cleaned)
	}
}

func normalizeSpelling(s string) string {
	s = " " + s + " "

	replacements := map[string]string{
		"yutub":          "youtube",
		"yutú":           "youtube",
		"bisual":         "vscode",
		"gugol":          "google",
		"gugel":          "google",
		"convertsistem":  "convertsystems",
		"convertsystems": "convertsystems.site",
		"doker":          "docker",
		"karpeta":        "carpeta",
		"prueva":         "prueba",
		"arxivo":         "archivo",
		"interner":       "internet",
		"sirra":          "cierra",
		"sierra":         "cierra",
		"habre":          "abre",
		"tranqui":        "tranquila",
		"pa":             "para",
		"q":              "que",
		"xq":             "por que",
	}

	for k, v := range replacements {
		s = strings.ReplaceAll(s, " "+k+" ", " "+v+" ")
		s = strings.ReplaceAll(s, " "+k+",", " "+v+",")
		s = strings.ReplaceAll(s, " "+k+".", " "+v+".")
		s = strings.ReplaceAll(s, " "+k+"?", " "+v+"?")
		s = strings.ReplaceAll(s, " "+k+"!", " "+v+"!")
	}

	return strings.TrimSpace(s)
}

func (o *Orchestrator) hasUserConfirmedConversational(history []ollama.Message, userInput string) bool {
	if len(history) == 0 {
		return false
	}

	var lastAssistantMsg string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastAssistantMsg = strings.ToLower(history[i].Content)
			break
		}
	}

	if lastAssistantMsg == "" || (!strings.Contains(lastAssistantMsg, "confirm") && !strings.Contains(lastAssistantMsg, "seguro") && !strings.Contains(lastAssistantMsg, "desea") && !strings.Contains(lastAssistantMsg, "proceder")) {
		return false
	}

	cleanInput := strings.ToLower(strings.TrimSpace(userInput))
	affirmatives := []string{"sí", "si", "hazlo", "proceder", "confirmo", "confirmar", "correcto", "adelante", "ejecuta", "acepto", "dale"}
	for _, aff := range affirmatives {
		if cleanInput == aff || strings.Contains(cleanInput, " "+aff+" ") || strings.HasPrefix(cleanInput, aff+" ") || strings.HasSuffix(cleanInput, " "+aff) {
			return true
		}
	}

	return false
}
