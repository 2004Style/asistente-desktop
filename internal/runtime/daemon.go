package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"rbot/internal/agent"
	"rbot/internal/apps"
	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/executor"
	"rbot/internal/files"
	"rbot/internal/intent"
	"rbot/internal/llm"
	llmBootstrap "rbot/internal/llm/bootstrap"
	"rbot/internal/mcp"
	"rbot/internal/planner"
	"rbot/internal/policy"
	"rbot/internal/scheduler"
	"rbot/internal/secrets"
	"rbot/internal/skills"
	"rbot/internal/tools/notifications"
	workspaceTools "rbot/internal/tools/workspace"
	"rbot/internal/voice"
	"rbot/internal/workspace"
)

// Daemon representa el proceso principal en segundo plano
type Daemon struct {
	Config             *config.Config
	DB                 *sql.DB
	LLMManager         *llm.Manager
	MCPManager         *mcp.ServerManager
	Orchestrator       *agent.Orchestrator
	EventBus           *EventBus
	Scheduler          *scheduler.Scheduler
	WorkspaceLoader    *workspace.Loader
	WorkspaceWatcher   *workspace.Watcher
	WorkspaceContext   *workspace.WorkspaceContext
	WorkspaceContextMu sync.RWMutex

	mu           sync.Mutex
	voiceCtx     context.Context
	voiceCancel  context.CancelFunc
	isVoiceAwake bool
}

type DaemonEventPublisher struct {
	Bus *EventBus
}

func (dep *DaemonEventPublisher) Publish(eventType string, payload map[string]interface{}) {
	dep.Bus.Publish(Event{
		Type:    eventType,
		Payload: payload,
	})
}

// NewDaemon inicializa y configura una nueva instancia del daemon
func NewDaemon(conf *config.Config, database *sql.DB) *Daemon {
	eb := NewEventBus()
	mcpManager := mcp.NewServerManager()

	providersConf, err := config.LoadProvidersConfig(conf.Providers.ConfigFile)
	if err != nil {
		log.Printf("[Daemon] No se pudo cargar config de proveedores: %v. Usando defaults seguros.", err)
		providersConf = config.DefaultProvidersConfig()
	}
	bootstrapResult, err := llmBootstrap.BuildRegistry(conf, providersConf, secrets.NewManager())
	if err != nil {
		log.Printf("[Daemon] No se pudo construir registry LLM: %v. Usando registry vacío.", err)
		bootstrapResult = &llmBootstrap.Result{Registry: llm.NewRegistry(), Active: ""}
	}

	// Crear el LLM Manager
	llmManager := llm.NewManager(database, bootstrapResult.Registry)

	// Intentar cargar configuración del proveedor activo desde la BD
	if err := llmManager.LoadFromDB(); err != nil {
		log.Printf("[Daemon] No se pudo cargar proveedor desde la BD: %v", err)
	}

	// Si no hay proveedor activo en la BD, usar el proveedor activo de providers.yaml
	if llmManager.Active() == nil && bootstrapResult.Active != "" {
		if err := llmManager.SetActive(bootstrapResult.Active); err != nil {
			log.Printf("[Daemon] Error estableciendo proveedor por defecto '%s': %v", bootstrapResult.Active, err)
		}
	}

	// Configurar parámetros de voz de whisper
	voice.WhisperThreads = conf.Voice.WhisperThreads
	voice.WhisperFlags = conf.Voice.WhisperFlags

	orchestrator := agent.NewOrchestrator(
		database,
		llmManager.Active(),
		mcpManager,
		conf.Security.BlockedPaths,
		conf.Files.AllowedRoots,
		conf.Agent.Name,
		conf,
	)
	orchestrator.SetEventPublisher(&DaemonEventPublisher{Bus: eb})

	nm := notifications.NewNotificationManager(database, &DaemonEventPublisher{Bus: eb}, conf)
	polEngine := policy.NewEngine(conf.Security.BlockedPaths, true)
	sched := scheduler.NewScheduler(database, conf, nm, orchestrator.Executor, polEngine)

	d := &Daemon{
		Config:       conf,
		DB:           database,
		LLMManager:   llmManager,
		MCPManager:   mcpManager,
		Orchestrator: orchestrator,
		EventBus:     eb,
		Scheduler:    sched,
	}

	// Inicializar Loader del Workspace
	wsPath := db.ExpandPath(conf.Workspace.Path)
	wsLoader := workspace.NewLoader(wsPath, conf.Workspace.AutoCreate)
	if err := wsLoader.Init(); err != nil {
		log.Printf("[Daemon] Advertencia al inicializar workspace: %v", err)
	}

	wsCtx, err := wsLoader.Load()
	if err != nil {
		log.Printf("[Daemon] Advertencia al cargar el workspace: %v", err)
	}

	d.WorkspaceLoader = wsLoader
	d.WorkspaceContext = wsCtx

	if wsCtx != nil {
		if engine, ok := orchestrator.Executor.Policy.(*policy.Engine); ok {
			engine.SetWorkspacePolicies(wsCtx.Policies)
		}
	}

	// Configurar el getter del orquestador
	orchestrator.GetWorkspaceContext = func() *workspace.WorkspaceContext {
		d.WorkspaceContextMu.RLock()
		defer d.WorkspaceContextMu.RUnlock()
		return d.WorkspaceContext
	}

	skillsVal := skills.NewValidator(func(name string) bool {
		_, exists := orchestrator.Registry.Get(name)
		return exists
	})
	installer := skills.NewInstaller(db.ExpandPath(conf.Skills.Path), skillsVal)

	getWS := func() *workspace.WorkspaceContext {
		d.WorkspaceContextMu.RLock()
		defer d.WorkspaceContextMu.RUnlock()
		return d.WorkspaceContext
	}

	reloadWS := func() (*workspace.WorkspaceContext, error) {
		newWS, err := d.WorkspaceLoader.Load()
		if err != nil {
			return nil, err
		}
		d.WorkspaceContextMu.Lock()
		d.WorkspaceContext = newWS
		d.WorkspaceContextMu.Unlock()

		if d.Orchestrator != nil && d.Orchestrator.Executor != nil && d.Orchestrator.Executor.Policy != nil {
			if engine, ok := d.Orchestrator.Executor.Policy.(*policy.Engine); ok {
				engine.SetWorkspacePolicies(newWS.Policies)
			}
		}
		return newWS, nil
	}

	runPlan := func(ctx context.Context, plan interface{}) (*executor.ToolResult, error) {
		p, ok := plan.(planner.Plan)
		if !ok {
			return nil, fmt.Errorf("tipo de plan inválido")
		}
		res, err := d.Orchestrator.Executor.ExecutePlan(ctx, p)
		if err != nil {
			return nil, err
		}
		if !res.Success {
			return &executor.ToolResult{
				Success: false,
				Error:   res.Error,
			}, nil
		}
		return &executor.ToolResult{
			Success: true,
			Text:    "Plan del atajo ejecutado con éxito.",
		}, nil
	}

	wsCtrl := workspaceTools.NewWorkspaceController(database, conf, wsLoader, installer, getWS, reloadWS, runPlan)
	_ = workspaceTools.RegisterTools(orchestrator.Registry, wsCtrl)

	// Inicializar Watcher si está habilitado
	if conf.Workspace.WatchChanges {
		onReload := func(newWS *workspace.WorkspaceContext) {
			log.Println("[Daemon] Recarga automática del workspace detectada.")
			_, _ = reloadWS()
		}
		d.WorkspaceWatcher = workspace.NewWatcher(wsPath, conf.Workspace.IncludeFiles, wsLoader, onReload)
	}

	return d
}

// Start inicializa todos los recursos persistentes y arranca los loops en segundo plano
func (d *Daemon) Start(ctx context.Context) error {
	d.EventBus.Publish(Event{
		Type:    "daemon.started",
		Payload: map[string]interface{}{"time": time.Now().Format(time.RFC3339)},
	})

	// Autodescubrimiento de habilidades
	if d.Config.Skills.AutoDiscover {
		skillsPath := db.ExpandPath(d.Config.Skills.Path)
		if err := skills.ScanSkills(d.DB, skillsPath); err == nil {
			// Mantener apagadas por defecto las de alto riesgo
			_, _ = d.DB.Exec("UPDATE skills SET enabled = 0 WHERE risk_level IN ('high', 'critical')")
		} else {
			log.Printf("[Daemon] Advertencia en auto-discover de skills: %v", err)
		}
	}

	// Auto-indexar aplicaciones si está vacío
	var countApps int
	if err := d.DB.QueryRow("SELECT COUNT(*) FROM app_launchers").Scan(&countApps); err == nil && countApps == 0 {
		go func() {
			_ = apps.ScanApplications(d.DB)
		}()
	}

	// Auto-indexar carpetas del usuario
	var countPaths int
	if err := d.DB.QueryRow("SELECT COUNT(*) FROM path_entries").Scan(&countPaths); err == nil && countPaths == 0 {
		go func() {
			_ = files.IndexRoots(d.DB, d.Config.Files.AllowedRoots, d.Config.Security.BlockedPaths, d.Config.Files.Ignore, d.Config.Files.MaxDepth)
		}()
	}

	// Bootstrapping MCP
	if d.Config.Mcp.Enabled {
		mcpConfig := db.ExpandPath(d.Config.Mcp.ConfigPath)
		go func() {
			if err := d.MCPManager.Bootstrap(mcpConfig); err != nil {
				log.Printf("[Daemon] MCP Bootstrap Advertencia: %v", err)
			}
		}()
	}

	// Iniciar la escucha continua de voz si se requiere
	if d.Config.Agent.VoiceEnabled {
		d.StartVoiceLoop(ctx)
	}

	// Iniciar el planificador si está habilitado
	if d.Config.Scheduler.Enabled && d.Scheduler != nil {
		d.Scheduler.Start(ctx)
	}

	// Iniciar el workspace watcher
	if d.WorkspaceWatcher != nil {
		d.WorkspaceWatcher.Start(ctx, time.Duration(d.Config.Workspace.ReloadDebounceMs)*time.Millisecond)
	}

	d.EventBus.Publish(Event{
		Type:    "daemon.ready",
		Payload: map[string]interface{}{"name": d.Config.Agent.Name},
	})

	return nil
}

// Stop finaliza limpiamente el daemon, cerrando MCP y SQLite
func (d *Daemon) Stop() {
	d.EventBus.Publish(Event{
		Type:    "daemon.stopping",
		Payload: map[string]interface{}{"time": time.Now().Format(time.RFC3339)},
	})

	if d.Config.Scheduler.Enabled && d.Scheduler != nil {
		d.Scheduler.Stop()
	}
	d.StopVoiceLoop()
	d.MCPManager.CloseAll()
	_ = d.DB.Close()
}

// StartVoiceLoop arranca la goroutine del loop continuo de voz
func (d *Daemon) StartVoiceLoop(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.voiceCancel != nil {
		return // Ya está corriendo
	}

	d.voiceCtx, d.voiceCancel = context.WithCancel(ctx)
	vadThresh := d.Config.Voice.VadThreshold
	if vadThresh <= 0 {
		vadThresh = 550.0
	}

	if err := voice.StartVoiceEngine(".", d.Config.Voice.PiperModel, d.Config.Voice.WhisperModel, vadThresh); err != nil {
		log.Printf("[Daemon Voice] Error al inicializar motor de voz: %v", err)
		return
	}
	voice.OnAudioLevel = func(level float64) {
		d.EventBus.Publish(Event{
			Type:    "voice.audio_level",
			Payload: map[string]interface{}{"level": level, "source": "mic"},
		})
	}
	d.EventBus.Publish(Event{Type: "voice.engine.started", Payload: nil})

	d.Orchestrator.IsVoiceMode = true
	d.isVoiceAwake = false

	go d.runVoiceLoop(d.voiceCtx)
}

// StopVoiceLoop detiene la escucha de voz de manera controlada
func (d *Daemon) StopVoiceLoop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.voiceCancel != nil {
		d.voiceCancel()
		d.voiceCancel = nil
		d.voiceCtx = nil
		voice.StopVoiceEngine()
	}
}

func (d *Daemon) runVoiceLoop(ctx context.Context) {
	var history []llm.Message
	lastInteraction := time.Now()
	timeoutDuration := 3 * time.Minute

	d.EventBus.Publish(Event{
		Type:    "voice.ready",
		Payload: map[string]interface{}{"message": "Entorno preparado, señor."},
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Comprobar expiración por inactividad
		if d.isVoiceAwake && time.Since(lastInteraction) > timeoutDuration {
			d.isVoiceAwake = false
			d.EventBus.Publish(Event{Type: "voice.timeout", Payload: nil})
			d.speakWithEvents("Vuelvo al modo de espera por inactividad, señor.")
		}

		d.EventBus.Publish(Event{Type: "voice.listening", Payload: nil})
		texto, err := voice.Listen()
		if err != nil {
			log.Printf("[Voice Loop] Error al escuchar: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		texto = strings.TrimSpace(texto)
		if texto == "" {
			continue
		}

		textoLower := strings.ToLower(texto)

		// Omitir ruidos o alucinaciones típicas de Whisper
		if textoLower == "[blank_audio]" ||
			(strings.HasPrefix(textoLower, "[") && strings.HasSuffix(textoLower, "]")) ||
			(strings.HasPrefix(textoLower, "(") && strings.HasSuffix(textoLower, ")")) ||
			intent.IsWhisperHallucination(texto) {
			continue
		}

		// Detección de WakeWord
		triggerDetected := ""
		for _, ww := range d.Config.Agent.WakeWords {
			wwLower := strings.ToLower(ww)
			if strings.Contains(textoLower, wwLower) {
				triggerDetected = wwLower
				break
			}
		}

		if !d.isVoiceAwake && triggerDetected == "" {
			continue
		}

		d.EventBus.Publish(Event{
			Type:    "voice.transcribed",
			Payload: map[string]interface{}{"text": texto},
		})

		// Detección de palabras de desactivación
		isSleepWord := false
		sleepWords := []string{
			"eso es todo", "gracias", "vete a dormir", "duérmete",
			"silencio", "apágate", "desactívate", "nada más",
			"eso es todo por ahora", "desconéctate",
		}
		for _, sw := range sleepWords {
			if strings.Contains(textoLower, sw) {
				isSleepWord = true
				break
			}
		}

		if isSleepWord && d.isVoiceAwake {
			d.isVoiceAwake = false
			d.EventBus.Publish(Event{Type: "voice.sleeping", Payload: nil})
			d.speakWithEvents("Entendido señor, vuelvo al modo de espera.")
			continue
		}

		cmdLimpio := ""
		if triggerDetected != "" {
			d.isVoiceAwake = true
			d.EventBus.Publish(Event{Type: "voice.wake_detected", Payload: map[string]interface{}{"word": triggerDetected}})
			lastInteraction = time.Now()

			// Extraer comando que acompaña a la WakeWord
			partes := strings.SplitN(textoLower, triggerDetected, 2)
			if len(partes) > 1 {
				cmdLimpio = intent.CleanCommand(partes[1])
			}

			if cmdLimpio == "" {
				voice.PauseMedia()
				d.EventBus.Publish(Event{Type: "voice.greeted", Payload: nil})
				d.speakWithEvents("Hola señor, ¿en qué le puedo servir?")
				voice.ResumeMedia()
				continue
			}
		} else if d.isVoiceAwake {
			cmdLimpio = texto
			lastInteraction = time.Now()
		}

		if cmdLimpio != "" {
			voice.PauseMedia()
			d.EventBus.Publish(Event{
				Type:    "agent.thinking",
				Payload: map[string]interface{}{"input": cmdLimpio},
			})

			d.Orchestrator.OnTextChunk = func(chunk string) {
				d.EventBus.Publish(Event{
					Type:    "agent.chunk",
					Payload: map[string]interface{}{"chunk": chunk},
				})
			}

			runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			d.EventBus.Publish(Event{Type: "agent.responding", Payload: nil})

			var respuesta string
			var err error
			respuesta, history, err = d.ProcessInput(runCtx, cmdLimpio, "voice", "local_voice", history)
			cancel()

			if err != nil {
				log.Printf("[Voice Loop] Error en chat: %v", err)
				d.EventBus.Publish(Event{Type: "daemon.error", Payload: map[string]interface{}{"error": err.Error()}})
				d.speakWithEvents("Disculpa, tuve un problema interno al procesar esa orden.")
				voice.ResumeMedia()
				continue
			}

			d.EventBus.Publish(Event{
				Type:    "agent.response",
				Payload: map[string]interface{}{"response": respuesta},
			})

			d.speakWithEvents(respuesta)
			voice.ResumeMedia()
		}
	}
}

// ProcessInput procesa un comando de entrada (de voz o de consola) a través del pipeline de la Fase 3.
func (d *Daemon) ProcessInput(ctx context.Context, userInput, source, sessionID string, history []llm.Message) (string, []llm.Message, error) {
	// 1. Normalizar el input
	normalized := intent.Normalize(userInput, d.Config.Agent.WakeWords)
	d.EventBus.Publish(Event{
		Type:    "intent.normalized",
		Payload: map[string]interface{}{"input": userInput, "normalized": normalized},
	})

	// 2. Comprobar si hay confirmación pendiente
	pendingPlan, reason, err := policy.GetPendingPlan(d.DB, source, sessionID)
	if err != nil {
		log.Printf("[Daemon] Error al buscar plan pendiente: %v", err)
	}

	if pendingPlan != nil {
		if policy.IsAffirmative(normalized) {
			d.EventBus.Publish(Event{
				Type:    "confirmation.accepted",
				Payload: map[string]interface{}{"plan_id": pendingPlan.ID, "reason": reason},
			})
			_ = policy.DeletePendingPlan(d.DB, pendingPlan.ID, "accepted")

			// Ejecutar el plan guardado revalidando contra el PolicyEngine
			result, err := d.Orchestrator.Executor.ExecutePlan(ctx, *pendingPlan)
			if err != nil {
				return "", nil, err
			}
			if !result.Success {
				return fmt.Sprintf("Error en la ejecución del plan: %s", result.Error), history, nil
			}
			return "Plan ejecutado exitosamente tras su confirmación.", history, nil
		}

		if policy.IsNegative(normalized) {
			d.EventBus.Publish(Event{
				Type:    "confirmation.cancelled",
				Payload: map[string]interface{}{"plan_id": pendingPlan.ID, "reason": reason},
			})
			_ = policy.DeletePendingPlan(d.DB, pendingPlan.ID, "cancelled")
			return "Entendido, he cancelado la acción pendiente.", history, nil
		}

		// Si no es afirmativo ni negativo, cancelamos el plan previo por inactividad/reemplazo y procedemos con el nuevo input
		_ = policy.DeletePendingPlan(d.DB, pendingPlan.ID, "expired")
		d.EventBus.Publish(Event{
			Type:    "confirmation.expired",
			Payload: map[string]interface{}{"plan_id": pendingPlan.ID},
		})
	}

	// 3. NLP Intent Router Pipeline
	parts := intent.SplitMultiIntent(normalized)
	d.EventBus.Publish(Event{
		Type:    "intent.split",
		Payload: map[string]interface{}{"parts": parts},
	})

	router := intent.NewRouter(d.DB)
	router.SetWakeWords(d.Config.Agent.WakeWords)
	router.SetToolExists(func(name string) bool {
		_, exists := d.Orchestrator.Registry.Get(name)
		return exists
	})

	// Construir el plan
	plan := planner.BuildPlan(normalized, parts, router)

	if len(plan.Steps) > 0 {
		d.EventBus.Publish(Event{
			Type:    "intent.detected",
			Payload: map[string]interface{}{"intent": plan.Intent, "confidence": plan.Confidence},
		})
		d.EventBus.Publish(Event{
			Type:    "plan.created",
			Payload: map[string]interface{}{"plan_id": plan.ID, "steps": plan.Steps, "risk_level": plan.RiskLevel},
		})

		// Ordenar secuencialmente según dependencias
		ordered, err := planner.ResolveDependencies(plan.Steps)
		if err == nil {
			plan.Steps = ordered
		} else {
			log.Printf("[Daemon] Error resolviendo dependencias: %v", err)
		}

		// Verificar riesgo crítico preventivo evaluando dinámicamente con el PolicyEngine
		isBlocked := false
		blockReason := ""
		for _, step := range plan.Steps {
			tool, ok := d.Orchestrator.Registry.Get(step.ToolName)
			if ok {
				decision := d.Orchestrator.Executor.Policy.EvaluateTool(ctx, tool, step.Args)
				if !decision.Allowed || decision.RiskLevel == "critical" {
					isBlocked = true
					blockReason = decision.Reason
					if blockReason == "" {
						blockReason = "acción destructiva o de riesgo crítico bloqueada de forma preventiva"
					}
					break
				}
			}
		}

		if isBlocked {
			d.EventBus.Publish(Event{
				Type:    "policy.blocked",
				Payload: map[string]interface{}{"plan_id": plan.ID, "reason": blockReason},
			})

			argsJSON, _ := json.Marshal(map[string]interface{}{"input": userInput})
			_, _ = d.DB.Exec("INSERT INTO action_log (plan_id, user_input, tool_name, arguments_json, risk_level, status, error) VALUES (?, ?, ?, ?, ?, 'denied', ?)",
				plan.ID, normalized, "blocked", string(argsJSON), "critical", blockReason)

			return "Lo siento, esa acción contiene comandos o rutas de riesgo crítico y ha sido bloqueada preventivamente por seguridad.", history, nil
		}

		// Si requiere confirmación
		if plan.NeedsConfirm {
			err = policy.SavePendingPlan(d.DB, &plan, "Riesgo alto requiere confirmación", source, sessionID, 5*time.Minute)
			if err != nil {
				log.Printf("[Daemon] Error guardando confirmación en SQLite: %v", err)
			}
			d.EventBus.Publish(Event{
				Type:    "policy.confirmation_required",
				Payload: map[string]interface{}{"plan_id": plan.ID},
			})
			d.EventBus.Publish(Event{
				Type:    "confirmation.saved",
				Payload: map[string]interface{}{"plan_id": plan.ID},
			})

			return fmt.Sprintf("¿Estás seguro de que quieres realizar esta acción? (%s)", plan.UserInput), history, nil
		}

		// Ejecución directa si la confianza es alta
		if plan.Confidence >= 0.60 {
			result, err := d.Orchestrator.Executor.ExecutePlan(ctx, plan)
			if err != nil {
				return "", nil, err
			}
			if !result.Success {
				return fmt.Sprintf("Ejecución fallida: %s", result.Error), history, nil
			}
			return "Acción completada exitosamente.", history, nil
		}
	}

	// 4. Conversación ordinaria delegada a Ollama si no hay confianza en herramientas
	respText, err := d.Orchestrator.Chat(ctx, userInput, history)
	if err != nil {
		return "", nil, err
	}

	newHistory := append(history, llm.Message{Role: "user", Content: userInput})
	newHistory = append(newHistory, llm.Message{Role: "assistant", Content: respText})
	if len(newHistory) > 10 {
		newHistory = newHistory[2:]
	}

	return respText, newHistory, nil
}

// HandleCommand maneja peticiones que ingresan mediante el Unix socket de comandos.
// Está estructurada para mapear métodos JSON-RPC 2.0 y devolver interfaces/errores limpios.
func (d *Daemon) HandleCommand(method string, args map[string]interface{}) (interface{}, error) {
	switch method {
	case "agent.status":
		providerName := "unknown"
		modelID := d.Config.Model.Model
		if d.LLMManager != nil && d.LLMManager.Active() != nil {
			providerName = d.LLMManager.ActiveName()
			modelID = d.LLMManager.Active().ModelID()
		}
		return map[string]interface{}{
			"name":        d.Config.Agent.Name,
			"model":       modelID,
			"provider":    providerName,
			"voice_awake": d.isVoiceAwake,
			"voice_loop":  d.voiceCancel != nil,
			"mcp_servers": len(d.MCPManager.Clients),
			"time":        time.Now().Format(time.RFC3339),
		}, nil

	case "agent.say":
		text, _ := args["text"].(string)
		if text == "" {
			return nil, fmt.Errorf("el parámetro 'text' es requerido")
		}

		d.EventBus.Publish(Event{
			Type:    "agent.thinking",
			Payload: map[string]interface{}{"input": text},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		d.EventBus.Publish(Event{Type: "agent.responding", Payload: nil})
		respText, _, err := d.ProcessInput(ctx, text, "cli", "local_cli", nil)
		if err != nil {
			d.EventBus.Publish(Event{Type: "daemon.error", Payload: map[string]interface{}{"error": err.Error()}})
			return nil, err
		}

		d.EventBus.Publish(Event{
			Type:    "agent.response",
			Payload: map[string]interface{}{"response": respText},
		})

		if d.Config.Agent.VoiceEnabled {
			d.speakWithEvents(respText)
		}

		return AgentResponse{
			Text: respText,
		}, nil

	case "skills.list":
		rows, err := d.DB.Query("SELECT name, description, risk_level, status FROM skills")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var list []map[string]interface{}
		for rows.Next() {
			var name, desc, risk, status string
			if err := rows.Scan(&name, &desc, &risk, &status); err == nil {
				list = append(list, map[string]interface{}{
					"name":        name,
					"description": desc,
					"risk_level":  risk,
					"status":      status,
				})
			}
		}
		return list, nil

	case "skills.info":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		var desc, version, risk, status, validationErrors string
		err := d.DB.QueryRow("SELECT description, version, risk_level, status, validation_errors FROM skills WHERE name = ?", name).Scan(&desc, &version, &risk, &status, &validationErrors)
		if err != nil {
			return nil, fmt.Errorf("habilidad no encontrada: %v", err)
		}
		return map[string]interface{}{
			"name":              name,
			"description":       desc,
			"version":           version,
			"risk_level":        risk,
			"status":            status,
			"validation_errors": validationErrors,
		}, nil

	case "skills.install":
		zipPath, _ := args["path"].(string)
		if zipPath == "" {
			return nil, fmt.Errorf("el parámetro 'path' es requerido")
		}
		skillsVal := skills.NewValidator(func(name string) bool {
			_, exists := d.Orchestrator.Registry.Get(name)
			return exists
		})
		installer := skills.NewInstaller(db.ExpandPath(d.Config.Skills.Path), skillsVal)
		meta, err := installer.InstallZip(zipPath)
		if err != nil {
			return nil, fmt.Errorf("error de instalación: %w", err)
		}
		_ = skills.ScanSkills(d.DB, db.ExpandPath(d.Config.Skills.Path))
		return fmt.Sprintf("Habilidad '%s' instalada en estado 'disabled'. Habilítala manualmente.", meta.Name), nil

	case "skills.enable":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		if err := skills.EnableSkill(d.DB, name); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Habilidad '%s' habilitada correctamente.", name), nil

	case "skills.disable":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		if err := skills.DisableSkill(d.DB, name); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Habilidad '%s' deshabilitada correctamente.", name), nil

	case "skills.trust":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		if err := skills.TrustSkill(d.DB, name); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Habilidad '%s' marcada como confiable (trusted).", name), nil

	case "skills.quarantine":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		if err := skills.QuarantineSkill(d.DB, name); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Habilidad '%s' colocada en cuarentena.", name), nil

	case "workspace.status":
		d.WorkspaceContextMu.RLock()
		defer d.WorkspaceContextMu.RUnlock()
		if d.WorkspaceContext == nil {
			return nil, fmt.Errorf("workspace no inicializado")
		}
		return map[string]interface{}{
			"path":      d.Config.Workspace.Path,
			"loaded_at": d.WorkspaceContext.LoadedAt.Format(time.RFC3339),
			"shortcuts": len(d.WorkspaceContext.Shortcuts),
		}, nil

	case "workspace.reload":
		newWS, err := d.WorkspaceLoader.Load()
		if err != nil {
			return nil, err
		}
		d.WorkspaceContextMu.Lock()
		d.WorkspaceContext = newWS
		d.WorkspaceContextMu.Unlock()
		if d.Orchestrator != nil && d.Orchestrator.Executor != nil && d.Orchestrator.Executor.Policy != nil {
			if engine, ok := d.Orchestrator.Executor.Policy.(*policy.Engine); ok {
				engine.SetWorkspacePolicies(newWS.Policies)
			}
		}
		return fmt.Sprintf("Workspace recargado correctamente. Atajos detectados: %d.", len(newWS.Shortcuts)), nil

	case "workspace.validate":
		d.WorkspaceContextMu.RLock()
		defer d.WorkspaceContextMu.RUnlock()
		if d.WorkspaceContext == nil {
			return nil, fmt.Errorf("workspace no inicializado")
		}
		val := workspace.NewValidator()
		if err := val.ValidatePolicies(d.WorkspaceContext.Policies); err != nil {
			return nil, fmt.Errorf("políticas locales inválidas: %w", err)
		}
		if err := val.ValidateShortcuts(d.WorkspaceContext.Shortcuts); err != nil {
			return nil, fmt.Errorf("shortcuts locales inválidos: %w", err)
		}
		return "Políticas y shortcuts del workspace validados correctamente.", nil

	case "shortcuts.list":
		d.WorkspaceContextMu.RLock()
		defer d.WorkspaceContextMu.RUnlock()
		if d.WorkspaceContext == nil {
			return nil, fmt.Errorf("workspace no inicializado")
		}
		var list []map[string]interface{}
		for _, s := range d.WorkspaceContext.Shortcuts {
			list = append(list, map[string]interface{}{
				"name":        s.Name,
				"description": s.Description,
				"triggers":    s.Triggers,
				"steps":       len(s.Steps),
			})
		}
		return list, nil

	case "mcp.list":
		var servers []map[string]interface{}
		d.MCPManager.Mu.Lock()
		defer d.MCPManager.Mu.Unlock()
		for srvName, client := range d.MCPManager.Clients {
			servers = append(servers, map[string]interface{}{
				"name":   srvName,
				"active": client.IsActive,
			})
		}
		return servers, nil

	case "hud.show":
		d.EventBus.Publish(Event{Type: "hud.show", Payload: nil})
		return "HUD visual mostrado.", nil

	case "hud.hide":
		d.EventBus.Publish(Event{Type: "hud.hide", Payload: nil})
		return "HUD visual ocultado.", nil

	case "hud.force_state":
		stateStr, _ := args["state"].(string)
		if stateStr == "" {
			return nil, fmt.Errorf("el parámetro 'state' es requerido")
		}
		d.EventBus.Publish(Event{
			Type:    "hud.force_state",
			Payload: map[string]interface{}{"state": stateStr},
		})
		return fmt.Sprintf("HUD forzado al estado '%s'.", stateStr), nil

	case "hud.notification":
		msg, _ := args["message"].(string)
		priority, _ := args["priority"].(string)
		if msg == "" {
			return nil, fmt.Errorf("el parámetro 'message' es requerido")
		}
		d.EventBus.Publish(Event{
			Type: "hud.notification",
			Payload: map[string]interface{}{
				"message":  msg,
				"priority": priority,
			},
		})
		return "Notificación HUD encolada.", nil

	case "providers.list":
		if d.LLMManager == nil {
			return nil, fmt.Errorf("LLM Manager no inicializado")
		}
		var list []map[string]interface{}
		for _, p := range d.LLMManager.Registry().List() {
			isActive := d.LLMManager.ActiveName() == p.Name()
			list = append(list, map[string]interface{}{
				"name":   p.Name(),
				"model":  p.ModelID(),
				"active": isActive,
			})
		}
		// También incluir proveedores persistidos en BD que no estén registrados
		dbConfigs, err := d.LLMManager.GetProviderConfigs()
		if err == nil {
			for _, cfg := range dbConfigs {
				_, exists := d.LLMManager.Registry().Get(cfg.Name)
				if !exists {
					list = append(list, map[string]interface{}{
						"name":   cfg.Name,
						"model":  cfg.Model,
						"active": cfg.IsActive,
						"stored": true,
					})
				}
			}
		}
		return list, nil

	case "providers.status":
		if d.LLMManager == nil || d.LLMManager.Active() == nil {
			return nil, fmt.Errorf("no hay proveedor LLM activo")
		}
		p := d.LLMManager.Active()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pingErr := p.Ping(ctx)
		status := "disponible"
		if pingErr != nil {
			status = fmt.Sprintf("error: %v", pingErr)
		}
		return map[string]interface{}{
			"provider": p.Name(),
			"model":    p.ModelID(),
			"status":   status,
		}, nil

	case "providers.use":
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("el parámetro 'name' es requerido")
		}
		if d.LLMManager == nil {
			return nil, fmt.Errorf("LLM Manager no inicializado")
		}
		if err := d.LLMManager.SetActive(name); err != nil {
			return nil, err
		}
		// Actualizar el proveedor en el orquestador
		d.Orchestrator.LLM = d.LLMManager.Active()
		return fmt.Sprintf("Proveedor LLM cambiado a '%s' (modelo: %s).", name, d.LLMManager.Active().ModelID()), nil

	case "models.list":
		if d.LLMManager == nil {
			return nil, fmt.Errorf("LLM Manager no inicializado")
		}
		providerName, _ := args["provider"].(string)
		if providerName == "" && d.LLMManager.Active() == nil {
			return nil, fmt.Errorf("no hay proveedor LLM activo")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var models []llm.ModelInfo
		var err error
		if providerName != "" {
			models, err = d.LLMManager.ListModelsForProvider(ctx, providerName)
		} else {
			models, err = d.LLMManager.ListModels(ctx)
		}
		if err != nil {
			return nil, err
		}
		var list []map[string]interface{}
		for _, m := range models {
			list = append(list, map[string]interface{}{
				"id":       m.ID,
				"name":     m.Name,
				"provider": m.Provider,
				"size":     m.Size,
				"tools":    m.Capabilities.ToolCalling,
			})
		}
		return list, nil

	case "models.switch":
		modelID, _ := args["model"].(string)
		providerName, _ := args["provider"].(string)
		if modelID == "" {
			return nil, fmt.Errorf("el parámetro 'model' es requerido")
		}
		if d.LLMManager == nil {
			return nil, fmt.Errorf("LLM Manager no inicializado")
		}
		if providerName != "" {
			if err := d.LLMManager.SwitchProviderModel(providerName, modelID); err != nil {
				return nil, err
			}
			if d.Orchestrator != nil {
				d.Orchestrator.LLM = d.LLMManager.Active()
			}
			return fmt.Sprintf("Proveedor LLM cambiado a '%s' con modelo '%s'.", providerName, modelID), nil
		}
		if err := d.LLMManager.SwitchModel(modelID); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Modelo LLM cambiado a '%s'.", modelID), nil

	default:
		return nil, fmt.Errorf("método no soportado: %s", method)
	}
}

// speakWithEvents sintetiza voz de forma síncrona mientras emite eventos del nivel de audio en una goroutine
func (d *Daemon) speakWithEvents(text string) {
	if text == "" {
		return
	}
	d.EventBus.Publish(Event{Type: "tts.speaking", Payload: map[string]interface{}{"text": text}})

	stopAudioLevel := make(chan struct{})
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		levels := []float64{0.4, 0.6, 0.8, 0.5, 0.7, 0.9, 0.4, 0.3, 0.6, 0.8, 0.5}
		idx := 0
		for {
			select {
			case <-ticker.C:
				lvl := levels[idx%len(levels)]
				d.EventBus.Publish(Event{
					Type:    "tts.audio_level",
					Payload: map[string]interface{}{"level": lvl, "source": "piper"},
				})
				idx++
			case <-stopAudioLevel:
				return
			}
		}
	}()

	_ = voice.Speak(text)
	close(stopAudioLevel)
	d.EventBus.Publish(Event{Type: "tts.finished", Payload: nil})
}
