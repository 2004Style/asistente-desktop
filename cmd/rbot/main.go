package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"flag"
	"rbot/internal/agent"
	"rbot/internal/apps"
	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/files"
	"rbot/internal/llm"
	llmBootstrap "rbot/internal/llm/bootstrap"
	"rbot/internal/mcp"
	"rbot/internal/onboarding"
	"rbot/internal/secrets"
	"rbot/internal/skills"
	"rbot/internal/voice"
)

func main() {
	// Definir ruta del config rbot.yaml (Modo Producción / XDG Base Directory)
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error al cargar la configuración: %v", err)
	}

	if len(os.Args) > 1 {
		cmd := strings.ToLower(os.Args[1])
		if cmd == "setup" || cmd == "onboard" {
			// parse flags for non-interactive setup
			fs := flag.NewFlagSet(cmd, flag.ExitOnError)
			provider := fs.String("provider", "", "Provider name: ollama|openai|compatible or custom key")
			model := fs.String("model", "", "Model id to set")
			baseURL := fs.String("base-url", "", "Base URL for provider (for compatible/openai)")
			secretRef := fs.String("secret-ref", "", "Secret reference, e.g. env:OPENAI_API_KEY")
			nonInteractive := fs.Bool("yes", false, "Non-interactive: accept defaults or provided flags")
			// parse only args after the command
			_ = fs.Parse(os.Args[2:])

			opts := onboarding.Options{
				ConfigPath:     configPath,
				In:             os.Stdin,
				Out:            os.Stdout,
				NonInteractive: *nonInteractive || *provider != "" || *model != "" || *baseURL != "" || *secretRef != "",
				Provider:       *provider,
				Model:          *model,
				BaseURL:        *baseURL,
				SecretRef:      *secretRef,
			}

			if err := onboarding.Run(context.Background(), opts); err != nil {
				log.Fatalf("Error durante onboarding: %v", err)
			}
			return
		}
	}

	// Inicializar Base de Datos
	sqlitePath := db.ExpandPath(conf.Database.Path)
	database, err := db.InitDB(sqlitePath)
	if err != nil {
		log.Fatalf("Error al inicializar la base de datos: %v", err)
	}
	defer database.Close()

	// Autodescubrimiento de habilidades
	if conf.Skills.AutoDiscover {
		skillsPath := db.ExpandPath(conf.Skills.Path)
		if err := skills.ScanSkills(database, skillsPath); err == nil {
			// Las habilidades inician deshabilitadas por defecto.
			// Nos aseguramos de mantener apagadas las de alto riesgo.
			_, _ = database.Exec("UPDATE skills SET enabled = 0 WHERE risk_level IN ('high', 'critical')")
		} else {
			log.Printf("[Warning] Error en auto-discovery de skills: %v", err)
		}
	}

	// Auto-indexar aplicaciones de escritorio si la tabla está vacía
	var countApps int
	if err := database.QueryRow("SELECT COUNT(*) FROM app_launchers").Scan(&countApps); err == nil && countApps == 0 {
		log.Println("[Auto-Index] La base de datos de aplicaciones está vacía. Escaneando en segundo plano...")
		go func() {
			if err := apps.ScanApplications(database); err != nil {
				log.Printf("[Auto-Index] Error al indexar aplicaciones: %v", err)
			} else {
				log.Println("[Auto-Index] Aplicaciones de escritorio indexadas con éxito.")
			}
		}()
	}

	// Auto-indexar rutas de archivos permitidas si la tabla está vacía
	var countPaths int
	if err := database.QueryRow("SELECT COUNT(*) FROM path_entries").Scan(&countPaths); err == nil && countPaths == 0 {
		log.Println("[Auto-Index] La base de datos de archivos está vacía. Indexando en segundo plano...")
		go func() {
			err := files.IndexRoots(database, conf.Files.AllowedRoots, conf.Security.BlockedPaths, conf.Files.Ignore, conf.Files.MaxDepth)
			if err != nil {
				log.Printf("[Auto-Index] Error al indexar rutas de archivos: %v", err)
			} else {
				log.Println("[Auto-Index] Rutas de archivos indexadas con éxito.")
			}
		}()
	}

	// Inicializar Manager de MCP (solo se levanta para modos que lo necesitan)
	mcpManager := mcp.NewServerManager()
	defer mcpManager.CloseAll()

	// Configurar parámetros globales de voz
	voice.WhisperThreads = conf.Voice.WhisperThreads
	voice.WhisperFlags = conf.Voice.WhisperFlags

	providersConf, err := config.LoadProvidersConfig(conf.Providers.ConfigFile)
	if err != nil {
		log.Printf("[LLM] No se pudo cargar config de proveedores: %v. Usando defaults seguros.", err)
		providersConf = config.DefaultProvidersConfig()
	}
	bootstrapResult, err := llmBootstrap.BuildRegistry(conf, providersConf, secrets.NewManager())
	if err != nil {
		log.Printf("[LLM] No se pudo construir registry de proveedores: %v", err)
		bootstrapResult = &llmBootstrap.Result{Registry: llm.NewRegistry(), Active: ""}
	}
	llmManager := llm.NewManager(database, bootstrapResult.Registry)
	if err := llmManager.LoadFromDB(); err != nil {
		log.Printf("[LLM] No se pudo cargar proveedor desde BD: %v", err)
	}
	if llmManager.Active() == nil && bootstrapResult.Active != "" {
		if err := llmManager.SetActive(bootstrapResult.Active); err != nil {
			log.Printf("[LLM] No se pudo activar proveedor '%s': %v", bootstrapResult.Active, err)
		}
	}
	if bootstrapResult.ActiveProfile != "" || bootstrapResult.ActiveModel != "" || bootstrapResult.ActiveAuthMode != "" {
		log.Printf("[LLM] Selección activa: profile=%s provider=%s model=%s auth=%s", bootstrapResult.ActiveProfile, bootstrapResult.Active, bootstrapResult.ActiveModel, bootstrapResult.ActiveAuthMode)
	}

	// Configurar callback de cambio de selección para mantener providersConf y YAML al día
	llmManager.OnActiveSelectionChanged = func(providerName, modelID, profileName string) {
		providersConf.ActiveProvider = providerName
		providersConf.ActiveModel = modelID
		providersConf.ActiveProfile = profileName
		if profileName != "" {
			if profile, ok := providersConf.Profiles[profileName]; ok {
				providersConf.ActiveAuthMode = profile.AuthMode
			}
		} else {
			if provider, ok := providersConf.Providers[providerName]; ok {
				providersConf.ActiveAuthMode = provider.EffectiveAuthMode()
			}
		}
		if conf.Providers.ConfigFile != "" {
			_ = config.SaveProvidersConfig(conf.Providers.ConfigFile, providersConf)
		}
	}

	// Inicializar Orquestador del Agente
	orchestrator := agent.NewOrchestrator(
		database,
		llmManager,
		providersConf,
		mcpManager,
		conf.Security.BlockedPaths,
		conf.Files.AllowedRoots,
		conf.Agent.Name,
		conf,
	)

	// Verificar argumentos CLI
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := strings.ToLower(os.Args[1])

	// Iniciar MCP solo para modos que realmente lo necesitan (voice, mcp)
	// Para chat, la mayoría de acciones directas no usan MCP y solo lo bloquearía al salir
	if conf.Mcp.Enabled && (cmd == "voice" || cmd == "mcp") {
		mcpConfig := db.ExpandPath(conf.Mcp.ConfigPath)
		go func() {
			if err := mcpManager.Bootstrap(mcpConfig); err != nil {
				log.Printf("[MCP] Advertencia: %v", err)
			}
		}()
	}

	switch cmd {
	case "chat":
		if len(os.Args) < 3 {
			log.Fatal("Por favor ingresa tu mensaje para el agente.")
		}
		msg := os.Args[2]

		// Iniciar motor de voz si está habilitado por voz
		if conf.Agent.VoiceEnabled {
			vadThresh := conf.Voice.VadThreshold
			if vadThresh <= 0 {
				vadThresh = 550.0
			}
			_ = voice.StartVoiceEngine(".", conf.Voice.PiperModel, conf.Voice.WhisperModel, vadThresh)
			defer voice.StopVoiceEngine()
		}

		var printedPrefix bool = false
		orchestrator.OnTextChunk = func(chunk string) {
			if !printedPrefix {
				fmt.Printf("\n%s: ", conf.Agent.Name)
				printedPrefix = true
			}
			fmt.Print(chunk)
		}

		ctx := context.Background()
		respuesta, err := orchestrator.Chat(ctx, msg, nil)
		if err != nil {
			log.Fatalf("Error en chat: %v", err)
		}

		if !printedPrefix {
			fmt.Printf("\n%s: %s\n", conf.Agent.Name, respuesta)
		} else {
			fmt.Println()
		}
		if conf.Agent.VoiceEnabled {
			_ = voice.Speak(respuesta)
		}

	case "index":
		if len(os.Args) < 3 {
			log.Fatal("Especifica qué indexar: 'paths' o 'apps'.")
		}
		subCmd := strings.ToLower(os.Args[2])
		if subCmd == "paths" {
			log.Println("Indexando archivos y carpetas del disco...")
			err := files.IndexRoots(database, conf.Files.AllowedRoots, conf.Security.BlockedPaths, conf.Files.Ignore, conf.Files.MaxDepth)
			if err != nil {
				log.Fatalf("Error indexando rutas: %v", err)
			}
			log.Println("Indexación de archivos finalizada con éxito.")
		} else if subCmd == "apps" {
			log.Println("Escaneando aplicaciones de escritorio (.desktop)...")
			err := apps.ScanApplications(database)
			if err != nil {
				log.Fatalf("Error indexando aplicaciones: %v", err)
			}
			log.Println("Indexación de aplicaciones finalizada.")
		} else {
			log.Fatalf("Comando de indexación desconocido: '%s'", subCmd)
		}

	case "skills":
		if len(os.Args) < 3 {
			log.Fatal("Uso: rbot skills [scan|list|enable|disable]")
		}
		subCmd := strings.ToLower(os.Args[2])
		skillsPath := db.ExpandPath(conf.Skills.Path)

		switch subCmd {
		case "scan":
			log.Printf("Escaneando carpeta de habilidades en: %s", skillsPath)
			if err := skills.ScanSkills(database, skillsPath); err != nil {
				log.Fatalf("Error al escanear skills: %v", err)
			}
			log.Println("Escaneo finalizado.")
		case "list":
			listSkills(database)
		case "enable":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a habilitar.")
			}
			name := os.Args[3]
			if err := skills.EnableSkill(database, name); err != nil {
				log.Fatalf("Error: %v", err)
			}
			log.Printf("Habilidad '%s' habilitada correctamente.", name)
		case "enable-all":
			_, err := database.Exec("UPDATE skills SET enabled = 1")
			if err != nil {
				log.Fatalf("Error al habilitar todas las habilidades: %v", err)
			}
			log.Println("Todas las habilidades registradas han sido habilitadas con éxito.")
		case "disable":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a deshabilitar.")
			}
			name := os.Args[3]
			if err := skills.DisableSkill(database, name); err != nil {
				log.Fatalf("Error: %v", err)
			}
			log.Printf("Habilidad '%s' deshabilitada.", name)
		case "add":
			if len(os.Args) < 4 {
				log.Fatal("Especifica la URL del repositorio Git de la habilidad a descargar.")
			}
			repoURL := os.Args[3]
			parts := strings.Split(strings.TrimSuffix(repoURL, "/"), "/")
			folderName := strings.TrimSuffix(parts[len(parts)-1], ".git")
			targetPath := filepath.Join(skillsPath, folderName)

			if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
				log.Fatalf("Error: La carpeta '%s' ya existe.", targetPath)
			}

			log.Printf("Descargando habilidad desde %s en %s...", repoURL, targetPath)
			cmd := exec.Command("git", "clone", repoURL, targetPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Fatalf("Error al descargar skill: %v", err)
			}

			if err := skills.ScanSkills(database, skillsPath); err != nil {
				log.Fatalf("Error al escanear skills: %v", err)
			}
			log.Printf("Habilidad descargada y registrada con éxito.")

		case "create":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a crear.")
			}
			name := os.Args[3]
			targetPath := filepath.Join(skillsPath, name)

			if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
				log.Fatalf("Error: La habilidad '%s' ya existe.", name)
			}

			if err := os.MkdirAll(targetPath, 0755); err != nil {
				log.Fatalf("Error al crear directorio: %v", err)
			}

			skillMdContent := fmt.Sprintf(`---
name: %s
description: "Habilidad para automatizar tareas relacionadas con %s"
version: 1.0.0
author: rbot
risk_level: low
voice_triggers:
  - "activar %s"
permissions:
  - "exec:echo"
---

# Instrucciones

Cuando el usuario pida "%s":
1. Usa la herramienta del sistema correspondiente
2. Responde confirmando que se ha ejecutado la acción de forma educada
`, name, name, name, name)

			skillMdPath := filepath.Join(targetPath, "SKILL.md")
			if err := os.WriteFile(skillMdPath, []byte(skillMdContent), 0644); err != nil {
				log.Fatalf("Error al escribir SKILL.md: %v", err)
			}

			if err := skills.ScanSkills(database, skillsPath); err != nil {
				log.Fatalf("Error al registrar skill: %v", err)
			}
			log.Printf("Habilidad '%s' creada, escrita en '%s' y registrada correctamente.", name, skillMdPath)

		default:
			log.Fatalf("Subcomando de skills desconocido: '%s'. Modos válidos: scan, list, enable, disable, add, create", subCmd)
		}

	case "mcp":
		if len(os.Args) < 3 || strings.ToLower(os.Args[2]) != "list" {
			log.Fatal("Uso: rbot mcp list")
		}
		listMcpTools(context.Background(), mcpManager)

	case "voice":
		// Obtener la ruta del socket
		socketPath := db.ExpandPath(conf.Runtime.SocketPath)

		// Intentar conectar para ver si el daemon está activo
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			log.Println("[Voice CLI] El daemon RBotd ya está activo y ejecutando el modo voz en segundo plano.")
			log.Println("[Voice CLI] Puedes interactuar por voz directamente, o usar 'rbotctl say' para enviarle instrucciones.")
			return
		}

		log.Println("[Voice CLI] El daemon RBotd no está activo.")
		log.Println("[Voice CLI] Inicia el daemon en segundo plano y luego vuelve a usar el CLI de voz.")
		log.Println("  rbotd &")
		return

	default:
		fmt.Printf("Comando desconocido: '%s'\n", cmd)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Uso: rbot <comando> [argumentos]")
	fmt.Println("\nComandos disponibles:")
	fmt.Println("  setup / onboard             Asistente interactivo para elegir proveedor y modelo.")
	fmt.Println("  chat \"<mensaje>\"            Envía un mensaje directo al agente (conversación).")
	fmt.Println("  index paths                 Indexa archivos y carpetas del disco.")
	fmt.Println("  index apps                  Indexa lanzadores de aplicaciones del escritorio.")
	fmt.Println("  skills scan                 Busca e indexa habilidades SKILL.md.")
	fmt.Println("  skills list                 Muestra la lista de habilidades instaladas.")
	fmt.Println("  skills enable <nombre>      Habilita una habilidad.")
	fmt.Println("  skills disable <nombre>     Deshabilita una habilidad.")
	fmt.Println("  mcp list                    Muestra las herramientas expuestas por servidores MCP.")
	fmt.Println("  voice                       Usa el modo de voz del daemon; inicia rbotd primero.")
}

func listSkills(db *sql.DB) {
	rows, err := db.Query("SELECT name, description, risk_level, enabled FROM skills")
	if err != nil {
		log.Fatalf("Error al leer skills: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n--- HABILIDADES REGISTRADAS ---")
	for rows.Next() {
		var name, desc, risk string
		var enabled int
		if err := rows.Scan(&name, &desc, &risk, &enabled); err == nil {
			status := "deshabilitada"
			if enabled == 1 {
				status = "habilitada"
			}
			fmt.Printf("- %s (Riesgo: %s) [%s]: %s\n", name, risk, status, desc)
		}
	}
}

func listMcpTools(ctx context.Context, mcpManager *mcp.ServerManager) {
	mcpManager.Mu.Lock()
	defer mcpManager.Mu.Unlock()

	fmt.Println("\n--- SERVIDORES Y HERRAMIENTAS MCP ---")
	for srvName, client := range mcpManager.Clients {
		status := "inactivo"
		if client.IsActive {
			status = "activo"
		}
		fmt.Printf("\nServidor: %s [%s]\n", srvName, status)

		if client.IsActive {
			tools, err := client.ListTools(ctx)
			if err != nil {
				fmt.Printf("  Error listando herramientas: %v\n", err)
				continue
			}
			for _, t := range tools {
				fmt.Printf("  * %s: %s\n", t.Name, t.Description)
			}
		}
	}
}
