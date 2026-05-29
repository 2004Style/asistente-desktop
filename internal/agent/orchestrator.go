package agent

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/files"
	"rbot/internal/intent"
	"rbot/internal/llm"
	"rbot/internal/mcp"
	"rbot/internal/personality"
	"rbot/internal/planner"
	"rbot/internal/policy"
	"rbot/internal/security"
	"rbot/internal/skills"
	browserTools "rbot/internal/tools/browser"
	desktopTools "rbot/internal/tools/desktop"
	filesTools "rbot/internal/tools/files"
	inputTools "rbot/internal/tools/input"
	mediaTools "rbot/internal/tools/media"
	meetingsTools "rbot/internal/tools/meetings"
	memoryTools "rbot/internal/tools/memory"
	notificationsTools "rbot/internal/tools/notifications"
	remindersTools "rbot/internal/tools/reminders"
	systemTools "rbot/internal/tools/system"
	tasksTools "rbot/internal/tools/tasks"
	llmTools "rbot/internal/tools/llm"
	"rbot/internal/workspace"
)

type DirectAction struct {
	ToolName string
	Args     map[string]interface{}
}

type Orchestrator struct {
	DB                  *sql.DB
	LLM                 llm.Provider
	MCP                 *mcp.ServerManager
	BlockedPaths        []string
	AllowedRoots        []string
	AgentName           string
	IsVoiceMode         bool
	Registry            *executor.Registry
	Executor            *executor.Executor
	GetWorkspaceContext func() *workspace.WorkspaceContext
	OnTextChunk         func(string) // Callback para streaming de texto
	EventPublisher      executor.EventPublisher
}

func NewOrchestrator(db *sql.DB, llmManager *llm.Manager, providersConf *config.ProvidersConfig, mcpManager *mcp.ServerManager, blockedPaths []string, allowedRoots []string, agentName string, cfg *config.Config) *Orchestrator {
	if agentName == "" {
		agentName = "RBot"
	}

	reg := executor.NewRegistry()
	pol := policy.NewEngine(blockedPaths, true)
	pol.SetDB(db)
	execObj := executor.NewExecutor(reg, pol, nil, db)

	// Registrar herramientas internas
	_ = desktopTools.RegisterTools(reg, db, allowedRoots, blockedPaths)
	_ = browserTools.RegisterTools(reg)
	_ = filesTools.RegisterTools(reg, db, allowedRoots, blockedPaths)
	_ = systemTools.RegisterTools(reg)
	_ = memoryTools.RegisterTools(reg, db)
	_ = inputTools.RegisterTools(reg)
	_ = mediaTools.RegisterTools(reg)

	// Registrar herramientas de gestión de LLM
	if llmManager != nil && providersConf != nil && cfg != nil {
		_ = llmTools.RegisterTools(reg, llmManager, providersConf, cfg.Providers.ConfigFile)
	}

	// Registrar herramientas de productividad si la configuración está disponible
	if cfg != nil {
		nm := notificationsTools.NewNotificationManager(db, nil, cfg)
		_ = notificationsTools.RegisterTools(reg, nm)
		_ = tasksTools.RegisterTools(reg, db, cfg)
		_ = remindersTools.RegisterTools(reg, db, cfg)
		_ = meetingsTools.RegisterTools(reg, db, cfg)
	}

	var activeProvider llm.Provider
	if llmManager != nil {
		activeProvider = llmManager.Active()
	}

	return &Orchestrator{
		DB:           db,
		LLM:          activeProvider,
		MCP:          mcpManager,
		BlockedPaths: blockedPaths,
		AllowedRoots: allowedRoots,
		AgentName:    agentName,
		Registry:     reg,
		Executor:     execObj,
	}
}

func (o *Orchestrator) SetEventPublisher(ep executor.EventPublisher) {
	if o.Executor != nil {
		o.Executor.Events = ep
	}
	o.EventPublisher = ep
}

// BuildSystemPrompt genera el prompt con memoria de usuario, habilidades/skills activas y metadatos del sistema en tiempo real
func (o *Orchestrator) BuildSystemPrompt(skillContexts []string, userInput string) string {
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

	workspaceSnippet := ""
	if o.GetWorkspaceContext != nil {
		wCtx := o.GetWorkspaceContext()
		if wCtx != nil {
			cb := workspace.NewContextBuilder()
			workspaceSnippet = cb.Build(userInput, wCtx)
		}
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

[CONTEXTO DE WORKSPACE]
%s
`, "2004Style", skillsSection, currentTime, appsStr, memoryStr, workspaceSnippet)
}

// GetAvailableTools consolida herramientas internas y MCP en formato compatible con LLM Tool Calling
func (o *Orchestrator) GetAvailableTools(ctx context.Context) []llm.Tool {
	// Asegurar que las herramientas MCP activas estén registradas
	o.refreshMCPTools(ctx)

	// Obtener la definición compatible con LLM de todas las herramientas registradas
	return o.Registry.GetLLMTools()
}

func (o *Orchestrator) refreshMCPTools(ctx context.Context) {
	if o.MCP == nil {
		return
	}
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
			fullName := fmt.Sprintf("mcp__%s__%s", serverName, t.Name)
			adapter := mcp.NewMCPToolAdapter(client, t, o.DB, fullName)
			o.Registry.RegisterOrReplace(adapter)
		}
	}
}

func (o *Orchestrator) matchShortcut(userInput string) *workspace.Shortcut {
	if o.GetWorkspaceContext == nil {
		return nil
	}
	wCtx := o.GetWorkspaceContext()
	if wCtx == nil {
		return nil
	}
	inputLower := strings.ToLower(strings.TrimSpace(userInput))
	for _, s := range wCtx.Shortcuts {
		for _, trigger := range s.Triggers {
			if strings.ToLower(strings.TrimSpace(trigger)) == inputLower {
				return &s
			}
		}
	}
	return nil
}

func (o *Orchestrator) processWindowCommands(userInput string) (string, bool) {
	clean := strings.ToLower(strings.TrimSpace(userInput))
	clean = strings.TrimFunc(clean, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == '?' || r == '¿' || r == '¡'
	})

	isOpen := false
	isClose := false
	isHUD := false
	isSettings := false

	if strings.Contains(clean, "ajustes") || strings.Contains(clean, "configuracion") || strings.Contains(clean, "configuración") || strings.Contains(clean, "configuraciones") || strings.Contains(clean, "control") {
		isSettings = true
	} else if strings.Contains(clean, "hud") || strings.Contains(clean, "pantalla principal") || strings.Contains(clean, "panel") || strings.Contains(clean, "esfera") {
		isHUD = true
	}

	if strings.Contains(clean, "abre") || strings.Contains(clean, "abrir") || strings.Contains(clean, "muestra") || strings.Contains(clean, "mostrar") || strings.Contains(clean, "enseña") || strings.Contains(clean, "enseñar") {
		isOpen = true
	} else if strings.Contains(clean, "cierra") || strings.Contains(clean, "cerrar") || strings.Contains(clean, "oculta") || strings.Contains(clean, "ocultar") || strings.Contains(clean, "quita") || strings.Contains(clean, "quitar") || strings.Contains(clean, "apaga") || strings.Contains(clean, "apagar") {
		isClose = true
	}

	if clean == "configuración" || clean == "configuracion" || clean == "ajustes" || clean == "control" || clean == "panel de control" {
		isOpen = true
		isSettings = true
	}

	if clean == "panel" || clean == "esfera" {
		isOpen = true
		isHUD = true
	}

	if isSettings {
		if isOpen {
			log.Println("[Orchestrator] Comando de voz detectado: abriendo configuración...")
			go func() {
				cmdPath := "rbot-settings-gio"
				if _, err := os.Stat("bin/rbot-settings-gio"); err == nil {
					cmdPath = "./bin/rbot-settings-gio"
				}
				_ = exec.Command(cmdPath).Start()
			}()
			return "Entendido señor, abriendo el panel de configuración.", true
		}
		if isClose {
			log.Println("[Orchestrator] Comando de voz detectado: cerrando configuración...")
			go func() {
				_ = exec.Command("killall", "rbot-settings-gio").Run()
			}()
			return "Entendido señor, cerrando el panel de configuración.", true
		}
	}

	if isHUD {
		if isOpen {
			log.Println("[Orchestrator] Comando de voz detectado: mostrando HUD...")
			if o.EventPublisher != nil {
				o.EventPublisher.Publish("hud.show", nil)
			}
			go func() {
				if err := exec.Command("pgrep", "rbot-hud").Run(); err != nil {
					cmdPath := "rbot-hud"
					if _, err := os.Stat("bin/rbot-hud"); err == nil {
						cmdPath = "./bin/rbot-hud"
					}
					_ = exec.Command(cmdPath).Start()
				}
			}()
			return "Entendido señor, mostrando el panel principal.", true
		}
		if isClose {
			log.Println("[Orchestrator] Comando de voz detectado: ocultando HUD...")
			if o.EventPublisher != nil {
				o.EventPublisher.Publish("hud.hide", nil)
			}
			return "Entendido señor, ocultando el panel principal.", true
		}
	}

	return "", false
}

// Chat realiza un paso conversacional resolviendo llamadas a herramientas si Ollama las requiere.
func (o *Orchestrator) Chat(ctx context.Context, userInput string, history []llm.Message) (string, error) {
	// Interceptar primero comandos de ventana antes de cualquier validación
	if resp, ok := o.processWindowCommands(userInput); ok {
		return resp, nil
	}

	if o.LLM == nil {
		return "No ha configurado aún ningún proveedor de inteligencia artificial. Por favor, abra los ajustes para configurar su proveedor local o en la nube.", nil
	}
	// Detectar si la entrada coincide con algún shortcut del workspace
	if shortcut := o.matchShortcut(userInput); shortcut != nil {
		log.Printf("[Orchestrator] Coincidencia con shortcut del workspace: '%s'. Ejecutando pasos...", shortcut.Name)
		var steps []planner.PlanStep
		for i, sStep := range shortcut.Steps {
			steps = append(steps, planner.PlanStep{
				ID:        fmt.Sprintf("step-%d", i+1),
				ToolName:  sStep.Intent,
				Args:      sStep.Args,
				TimeoutMs: 20000,
			})
		}
		plan := planner.Plan{
			ID:           "plan-shortcut-" + time.Now().Format("20060102150405"),
			UserInput:    userInput,
			Intent:       "shortcut",
			Confidence:   1.0,
			RiskLevel:    "medium",
			NeedsConfirm: false,
			Steps:        steps,
		}

		res, err := o.Executor.ExecutePlan(ctx, plan)
		if err != nil {
			return fmt.Sprintf("Señor, ocurrió un error al iniciar la ejecución del atajo '%s': %v", shortcut.Name, err), nil
		}
		if !res.Success {
			return fmt.Sprintf("Señor, falló la ejecución del atajo '%s': %s", shortcut.Name, res.Error), nil
		}
		return fmt.Sprintf("Entendido, señor. He completado con éxito todas las acciones del atajo '%s'.", shortcut.Name), nil
	}

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

			// Validate security using unified Policy path
			// Resolve tool handler
			toolHandler, ok := o.Registry.Get(toolName)
			if !ok {
				resultStr := fmt.Sprintf("Error: herramienta no registrada: %s", toolName)
				execErr := fmt.Errorf("herramienta no registrada: %s", toolName)
				// Log as denied for safety
				_, _ = o.DB.Exec(`INSERT INTO action_log (user_input, tool_name, tool_source, arguments_json, result_json, success, error, required_confirmation, confirmed_by_user, duration_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, userInput, toolName, "internal", "{}", resultStr, 0, execErr.Error(), 0, 0, 0)
				return fmt.Sprintf("%s", resultStr), nil
			}

			decision := o.Executor.Policy.EvaluateTool(ctx, toolHandler, args)

			// Extra check for shell critical commands: escalate to require confirmation
			if toolName == "system.run_command_safe" {
				if cmdStr, ok := args["command"].(string); ok {
					if security.IsCommandCritical(cmdStr) {
						decision.RequiresConfirm = true
						if decision.Reason == "" {
							decision.Reason = fmt.Sprintf("el comando '%s' es crítico y requiere confirmación explícita", cmdStr)
						}
						decision.RiskLevel = "high"
					}
				}
			}

			var resultStr string
			var execErr error
			var success int = 0
			var confirmedByUser int = 0

			if !decision.Allowed {
				resultStr = fmt.Sprintf("Error: Acción denegada por seguridad. %s", decision.Reason)
				execErr = fmt.Errorf("denegado: %s", decision.Reason)
			} else {
				if decision.RequiresConfirm {
					confirmedByUser = 1
					if o.IsVoiceMode {
						if o.hasUserConfirmedConversational(history, userInput) {
							resultStr, execErr = o.executeTool(ctx, toolName, args)
						} else {
							resultStr = fmt.Sprintf("Error: Confirmación conversacional requerida para '%s'.", toolName)
							execErr = fmt.Errorf("confirmación conversacional requerida: %s", decision.Reason)
							success = 0
						}
					} else {
						if o.askConfirmationInteractive(toolName, fmt.Sprintf("%v", args), decision.Reason) {
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
			`, userInput, toolName, source, string(argsBytes), resultStr, success, fmt.Sprintf("%v", execErr), boolToInt(decision.RequiresConfirm), confirmedByUser, 0)

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
				State:     state,
				Error:     lastErr,
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
				State:     personality.StateDone,
				Risk:      personality.RiskLow,
				ToolName:  act.ToolName,
				Target:    target,
				AgentName: o.AgentName,
			})
		} else {
			confirmationText = personality.ComposeResponse(personality.ResponseContext{
				State:     personality.StateDone,
				Risk:      personality.RiskLow,
				AgentName: o.AgentName,
			})
		}

		return confirmationText, nil
	}

	// Construir historial con el mensaje de sistema y el nuevo prompt
	var messages []llm.Message
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: o.BuildSystemPrompt(skillContexts, userInput),
	})

	// Cargar historial previo
	messages = append(messages, history...)

	// Agregar entrada del usuario
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userInput,
	})

	// Obtener herramientas
	tools := o.GetAvailableTools(ctx)

	log.Printf("[Orchestrator] Iniciando ciclo de llamadas a LLM (%s)...", o.LLM.Name())
	start := time.Now()

	chatOpts := llm.ChatOptions{
		OnTextChunk: o.OnTextChunk,
	}

	maxSteps := 5
	for step := 0; step < maxSteps; step++ {
		log.Printf("[Orchestrator] Enviando chat a %s (Paso %d/%d)...", o.LLM.Name(), step+1, maxSteps)
		respMessage, err := o.LLM.Chat(ctx, messages, tools, chatOpts)
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
			if toolName == "system.run_command_safe" {
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
		messages = append(messages, llm.Message{
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
		o.refreshMCPTools(ctx)
	}

	step := planner.PlanStep{
		ID:        "step-1",
		ToolName:  toolName,
		Args:      args,
		TimeoutMs: 20000,
	}
	plan := planner.Plan{
		ID:           "plan-" + time.Now().Format("20060102150405"),
		UserInput:    "Llamada directa de herramienta",
		Intent:       "direct_action",
		Confidence:   1.0,
		RiskLevel:    "low",
		NeedsConfirm: false,
		Steps:        []planner.PlanStep{step},
	}

	res, err := o.Executor.ExecutePlan(ctx, plan)
	if err != nil {
		return "", err
	}

	if !res.Success {
		if res.Error != "" {
			return "", fmt.Errorf("%s", res.Error)
		}
		if len(res.Results) > 0 {
			lastRes := res.Results[len(res.Results)-1]
			if lastRes.Error != "" {
				return "", fmt.Errorf("%s", lastRes.Error)
			}
		}
		return "", fmt.Errorf("ejecución fallida")
	}

	if len(res.Results) > 0 {
		return res.Results[0].Text, nil
	}

	return "", nil
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

	// --- 0. Control de multimedia directo ---
	cleanedLower := strings.ToLower(cleaned)

	// Next
	for _, kw := range []string{"siguiente", "siguiente cancion", "siguiente canción", "siguiente pista", "siguiente video", "siguiente vídeo", "cambia la musica", "cambia la música", "cambia de cancion", "cambia de canción", "cambia cancion", "cambia canción", "cambiar musica", "cambiar música"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.next", Args: map[string]interface{}{}}}
		}
	}

	// Previous
	for _, kw := range []string{"anterior", "cancion anterior", "canción anterior", "pista anterior", "anterior cancion", "anterior canción", "anterior pista", "anterior video", "anterior vídeo", "vuelve a la anterior"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.previous", Args: map[string]interface{}{}}}
		}
	}

	// Pause
	for _, kw := range []string{"pausa", "pausar", "pausa la musica", "pausa la música", "pausar musica", "pausar música", "para la musica", "para la música", "parar musica", "parar música", "detén la música", "deten la musica"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.pause", Args: map[string]interface{}{}}}
		}
	}

	// Resume
	for _, kw := range []string{"continua", "continúa", "reanuda", "reanudar", "sigue con la musica", "sigue con la música", "reanuda la musica", "reanuda la música", "continua la musica", "continúa la música", "reanudar la musica", "reanudar la música", "play"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.resume", Args: map[string]interface{}{}}}
		}
	}

	// Volume Up
	for _, kw := range []string{"sube el volumen", "subir volumen", "mas volumen", "más volumen", "sube audio", "sube el audio", "subir audio"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.volume_up", Args: map[string]interface{}{}}}
		}
	}

	// Volume Down
	for _, kw := range []string{"baja el volumen", "bajar volumen", "menos volumen", "baja audio", "baja el audio", "bajar audio"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.volume_down", Args: map[string]interface{}{}}}
		}
	}

	// Mute
	for _, kw := range []string{"silencia", "silenciar", "mutea", "mutear", "quitar volumen", "quita el volumen", "sin sonido"} {
		if cleanedLower == kw {
			return []DirectAction{{ToolName: "media.mute", Args: map[string]interface{}{}}}
		}
	}

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
	// 3.1. YouTube Play / Reproducción de Música
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

	// Forzar reproducción directa si el comando menciona youtube y no es solo abrir la web
	hasYouTube := strings.Contains(inputLower, "youtube") || strings.Contains(inputLower, "yutub") || strings.Contains(inputLower, "yutú")
	if hasYouTube && !isYouTubeOpenOnly {
		isPlayMusicIntent = true
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

		// Extraer consulta limpia quitando sufijos de youtube y prefijos de acción
		query := cleaned
		for _, suf := range []string{" en youtube", " en el youtube", " en youtube.com", " youtube", " yutub", " yutú"} {
			if strings.HasSuffix(query, suf) {
				query = strings.TrimSuffix(query, suf)
				break
			}
		}
		
		// Remover múltiples prefijos redundantes
		for {
			prevQuery := query
			for _, pref := range []string{
				"en youtube ", "en el youtube ", "en youtube.com ", "en yutub ", "en yutú ",
				"buscar ", "busca ", "encuentra ", "reproduce ", "reproducir ", "pon ", "poner ", "toca ", "tocar ", "escucha ", "escuchar ",
			} {
				if strings.HasPrefix(query, pref) {
					query = strings.TrimPrefix(query, pref)
				}
			}
			if query == prevQuery {
				break
			}
		}

		query = strings.TrimSpace(query)
		if query != "" {
			// Siempre usar browser.youtube_play en lugar de youtube_search
			return []DirectAction{{ToolName: "browser.youtube_play", Args: map[string]interface{}{"query": query}}}
		}
	}

	// --- 3.1.5. Control de proveedores y modelos LLM en lenguaje natural ---
	if inputLower == "lista mis providers" || inputLower == "lista providers" || inputLower == "qué providers hay" || inputLower == "qué providers tengo" {
		return []DirectAction{{ToolName: "llm.list_providers", Args: map[string]interface{}{}}}
	}

	if inputLower == "lista mis modelos locales" || inputLower == "lista mis modelos" || inputLower == "lista modelos locales" || strings.Contains(inputLower, "lista mis modelos de ollama") || strings.Contains(inputLower, "lista los modelos de ollama") || strings.Contains(inputLower, "modelos de ollama") {
		return []DirectAction{{ToolName: "llm.list_models", Args: map[string]interface{}{"provider": "ollama"}}}
	}

	if inputLower == "qué modelo estoy usando" || inputLower == "qué modelo usas" || inputLower == "qué modelo está activo" || inputLower == "modelo activo" ||
		inputLower == "qué provider está activo" || inputLower == "qué provider usas" || inputLower == "provider activo" ||
		inputLower == "qué modo de autenticación está activo" || inputLower == "qué auth está activa" || inputLower == "auth activa" {
		return []DirectAction{{ToolName: "llm.get_status", Args: map[string]interface{}{}}}
	}

	if inputLower == "verifica si ollama está disponible" || inputLower == "verifica ollama" {
		return []DirectAction{{ToolName: "llm.verify_provider", Args: map[string]interface{}{"provider": "ollama"}}}
	}
	if inputLower == "verifica si openai está configurado" || inputLower == "verifica openai" {
		return []DirectAction{{ToolName: "llm.verify_provider", Args: map[string]interface{}{"provider": "openai"}}}
	}

	if inputLower == "usa openai con api key" || inputLower == "usa openai api key" || inputLower == "openai con api key" {
		return []DirectAction{{ToolName: "llm.use_profile", Args: map[string]interface{}{"name": "openai_api"}}}
	}
	if inputLower == "usa mi cuenta de openai" || inputLower == "usa openai con cuenta" || inputLower == "openai con cuenta" {
		return []DirectAction{{ToolName: "llm.use_profile", Args: map[string]interface{}{"name": "openai_account"}}}
	}
	if inputLower == "usa el perfil de código" || inputLower == "usa el perfil de codigo" || inputLower == "usa el perfil coder" {
		return []DirectAction{{ToolName: "llm.use_profile", Args: map[string]interface{}{"name": "local_code"}}}
	}
	if strings.HasPrefix(inputLower, "usa el perfil ") {
		profileName := strings.TrimPrefix(inputLower, "usa el perfil ")
		profileName = strings.TrimSpace(profileName)
		return []DirectAction{{ToolName: "llm.use_profile", Args: map[string]interface{}{"name": profileName}}}
	}

	if inputLower == "cambia a mistral" || inputLower == "cambia el modelo a mistral" || inputLower == "usa mistral" || inputLower == "cambia a mistral local" {
		return []DirectAction{{ToolName: "llm.switch_model", Args: map[string]interface{}{"model": "mistral:latest"}}}
	}
	if inputLower == "usa qwen para tareas normales" || inputLower == "cambia a qwen" || inputLower == "usa qwen" {
		return []DirectAction{{ToolName: "llm.switch_model", Args: map[string]interface{}{"model": "qwen2.5:7b"}}}
	}
	if inputLower == "usa el modelo coder" || inputLower == "usa qwen coder" || inputLower == "cambia a coder" {
		return []DirectAction{{ToolName: "llm.switch_model", Args: map[string]interface{}{"model": "qwen2.5-coder:7b"}}}
	}

	if inputLower == "cambia a openai" || inputLower == "usa openai" {
		return []DirectAction{{ToolName: "llm.use_provider", Args: map[string]interface{}{"provider": "openai"}}}
	}
	if inputLower == "vuelve a ollama" || inputLower == "usa ollama" || inputLower == "vuelve al modelo local" {
		return []DirectAction{{ToolName: "llm.use_provider", Args: map[string]interface{}{"provider": "ollama"}}}
	}

	if inputLower == "crea un perfil para programar" || inputLower == "crea un perfil de código" || inputLower == "crear perfil de codigo" {
		return []DirectAction{{ToolName: "llm.create_profile", Args: map[string]interface{}{
			"name":        "local_code",
			"provider":    "ollama",
			"model":       "qwen2.5-coder:7b",
			"description": "Perfil local para programar.",
		}}}
	}
	if inputLower == "crea un perfil local rápido" || inputLower == "crear perfil local rapido" {
		return []DirectAction{{ToolName: "llm.create_profile", Args: map[string]interface{}{
			"name":        "local_fast",
			"provider":    "ollama",
			"model":       "qwen2.5:7b",
			"description": "Perfil local rápido para tareas generales.",
		}}}
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

	stopwords := map[string]bool{
		"el": true, "la": true, "los": true, "las": true,
		"un": true, "una": true, "unos": true, "unas": true,
		"de": true, "del": true, "al": true,
		"y": true, "o": true, "e": true, "u": true,
		"en": true, "es": true, "con": true, "por": true, "para": true,
		"se": true, "me": true, "te": true, "le": true, "lo": true,
		"nos": true, "les": true, "os": true,
		"que": true, "qué": true, "como": true, "cómo": true,
		"si": true, "sí": true, "no": true,
		"mi": true, "mis": true, "su": true, "sus": true, "tu": true, "tus": true,
		"este": true, "esta": true, "esto": true, "estos": true, "estas": true,
		"ser": true, "estar": true, "hacer": true, "ir": true,
		"a": true, "an": true, "the": true, "and": true, "or": true, "is": true, "are": true, "it": true,
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
			if stopwords[token] {
				continue
			}

			// Para tokens cortos (longitud <= 3), evitar coincidencia de subcadenas en medio de palabras
			if len(token) <= 3 {
				baseExec := execLower
				if idx := strings.LastIndex(execLower, "/"); idx != -1 {
					baseExec = execLower[idx+1:]
				}
				if nameLower == token || strings.HasPrefix(nameLower, token) {
					score += 3
				}
				if displayLower == token || strings.HasPrefix(displayLower, token) {
					score += 2
				}
				if baseExec == token || strings.HasPrefix(baseExec, token) {
					score += 3
				}
			} else {
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
		}

		// Solo aplicar coincidencia completa de consulta si la consulta entera no es un stopword y no es extremadamente corta
		if !stopwords[query] && len(query) > 3 {
			if strings.Contains(displayLower, query) || strings.Contains(query, displayLower) {
				score += 5
			}
			if strings.Contains(nameLower, query) || strings.Contains(query, nameLower) {
				score += 5
			}
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

func (o *Orchestrator) hasUserConfirmedConversational(history []llm.Message, userInput string) bool {
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
