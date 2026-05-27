package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"rbot/internal/config"
	"rbot/internal/db"
	"rbot/internal/ipc"
)

func main() {
	// Definir ruta del config rbot.yaml (Modo Producción / XDG Base Directory)
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")

	// Modo Desarrollo: prioritario si está en la carpeta raíz
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		configPath = "config/rbot.yaml"
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error al cargar la configuración: %v", err)
	}

	socketPath := db.ExpandPath(conf.Runtime.SocketPath)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	if len(os.Args) > 1 && strings.ToLower(os.Args[1]) == "settings" {
		if len(os.Args) < 3 {
			printUsage()
			return
		}
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "status":
		resp, err := ipc.SendCommandRPC(socketPath, "agent.status", nil, "status-req")
		if err != nil {
			log.Fatalf("Error de conexión: %v", err)
		}
		if resp.Error != nil {
			log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
		}

		data, ok := resp.Result.(map[string]interface{})
		if !ok {
			log.Fatalf("Respuesta malformada en Result: %v", resp.Result)
		}

		fmt.Println("--- ESTADO DEL DAEMON RBot ---")
		fmt.Printf("Nombre del Agente: %v\n", data["name"])
		fmt.Printf("Proveedor LLM:    %v\n", data["provider"])
		fmt.Printf("Modelo LLM:       %v\n", data["model"])
		fmt.Printf("Loop de Voz:      %v\n", data["voice_loop"])
		fmt.Printf("Voz Despierta:    %v\n", data["voice_awake"])
		fmt.Printf("Servidores MCP:   %v\n", data["mcp_servers"])
		fmt.Printf("Hora del Daemon:  %v\n", data["time"])

	case "say":
		if len(os.Args) < 3 {
			log.Fatal("Especifica el texto o instrucción a enviar. Ej: rbotctl say \"abre el navegador\"")
		}
		text := os.Args[2]

		resp, err := ipc.SendCommandRPC(socketPath, "agent.say", map[string]interface{}{"text": text}, "say-req")
		if err != nil {
			log.Fatalf("Error de conexión: %v", err)
		}
		if resp.Error != nil {
			log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
		}

		data, ok := resp.Result.(map[string]interface{})
		if !ok {
			log.Fatalf("Respuesta malformada en Result: %v", resp.Result)
		}

		fmt.Printf("%v: %s\n", conf.Agent.Name, data["text"])

	case "skills":
		if len(os.Args) < 3 {
			log.Fatal("Especifica el subcomando de skills: 'list', 'info', 'install', 'enable', 'disable', 'trust' o 'quarantine'.")
		}
		subCmd := strings.ToLower(os.Args[2])

		switch subCmd {
		case "list":
			resp, err := ipc.SendCommandRPC(socketPath, "skills.list", nil, "skills-list-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
			}

			list, ok := resp.Result.([]interface{})
			if !ok {
				log.Fatalf("Respuesta malformada en Result: %v", resp.Result)
			}

			fmt.Println("\n--- HABILIDADES EN EL DAEMON ---")
			for _, item := range list {
				m, _ := item.(map[string]interface{})
				fmt.Printf("- %s (Riesgo: %s) [Estado: %s]: %s\n", m["name"], m["risk_level"], m["status"], m["description"])
			}

		case "info":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.info", map[string]interface{}{"name": name}, "skills-info-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			data, _ := resp.Result.(map[string]interface{})
			fmt.Printf("--- HABILIDAD: %s ---\n", data["name"])
			fmt.Printf("Descripción: %s\n", data["description"])
			fmt.Printf("Versión:     %s\n", data["version"])
			fmt.Printf("Riesgo:      %s\n", data["risk_level"])
			fmt.Printf("Estado:      %s\n", data["status"])
			if data["validation_errors"] != "" {
				fmt.Printf("Errores de validación: %s\n", data["validation_errors"])
			}

		case "install":
			if len(os.Args) < 4 {
				log.Fatal("Especifica la ruta al archivo ZIP de la habilidad.")
			}
			zipPath := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.install", map[string]interface{}{"path": zipPath}, "skills-install-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "enable":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a habilitar.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.enable", map[string]interface{}{"name": name}, "skills-enable-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "disable":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a deshabilitar.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.disable", map[string]interface{}{"name": name}, "skills-disable-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "trust":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a marcar como confiable.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.trust", map[string]interface{}{"name": name}, "skills-trust-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "quarantine":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre de la habilidad a colocar en cuarentena.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "skills.quarantine", map[string]interface{}{"name": name}, "skills-quarantine-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		default:
			log.Fatalf("Subcomando de skills desconocido en rbotctl: '%s'", subCmd)
		}

	case "workspace":
		if len(os.Args) < 3 {
			log.Fatal("Especifica el subcomando de workspace: 'status', 'reload' o 'validate'.")
		}
		subCmd := strings.ToLower(os.Args[2])

		switch subCmd {
		case "status":
			resp, err := ipc.SendCommandRPC(socketPath, "workspace.status", nil, "workspace-status-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			data, _ := resp.Result.(map[string]interface{})
			fmt.Println("--- ESTADO DEL WORKSPACE ---")
			fmt.Printf("Ruta:       %v\n", data["path"])
			fmt.Printf("Cargado:    %v\n", data["loaded_at"])
			fmt.Printf("Atajos:     %v\n", data["shortcuts"])

		case "reload":
			resp, err := ipc.SendCommandRPC(socketPath, "workspace.reload", nil, "workspace-reload-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "validate":
			resp, err := ipc.SendCommandRPC(socketPath, "workspace.validate", nil, "workspace-validate-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		default:
			log.Fatalf("Subcomando de workspace desconocido en rbotctl: '%s'", subCmd)
		}

	case "shortcuts":
		if len(os.Args) < 3 || strings.ToLower(os.Args[2]) != "list" {
			log.Fatal("Uso: rbotctl shortcuts list")
		}
		resp, err := ipc.SendCommandRPC(socketPath, "shortcuts.list", nil, "shortcuts-list-req")
		if err != nil {
			log.Fatalf("Error de conexión: %v", err)
		}
		if resp.Error != nil {
			log.Fatalf("Error del daemon: %s", resp.Error.Message)
		}
		list, ok := resp.Result.([]interface{})
		if !ok {
			log.Fatalf("Respuesta malformada en Result: %v", resp.Result)
		}
		fmt.Println("\n--- ATAJOS (SHORTCUTS) EN EL WORKSPACE ---")
		for _, item := range list {
			m, _ := item.(map[string]interface{})
			fmt.Printf("- %s: %s (Pasos: %v)\n", m["name"], m["description"], m["steps"])
			if trigSlice, ok := m["triggers"].([]interface{}); ok && len(trigSlice) > 0 {
				var trigs []string
				for _, t := range trigSlice {
					trigs = append(trigs, fmt.Sprintf("'%v'", t))
				}
				fmt.Printf("  Triggers: %s\n", strings.Join(trigs, ", "))
			}
		}

	case "mcp":
		if len(os.Args) < 3 || strings.ToLower(os.Args[2]) != "list" {
			log.Fatal("Uso: rbotctl mcp list")
		}

		resp, err := ipc.SendCommandRPC(socketPath, "mcp.list", nil, "mcp-list-req")
		if err != nil {
			log.Fatalf("Error de conexión: %v", err)
		}
		if resp.Error != nil {
			log.Fatalf("Error del daemon (código %d): %s", resp.Error.Code, resp.Error.Message)
		}

		list, ok := resp.Result.([]interface{})
		if !ok {
			log.Fatalf("Respuesta malformada en Result: %v", resp.Result)
		}

		fmt.Println("\n--- SERVIDORES MCP ACTIVOS EN EL DAEMON ---")
		for _, item := range list {
			m, _ := item.(map[string]interface{})
			status := "inactivo"
			if m["active"].(bool) {
				status = "activo"
			}
			fmt.Printf("- %s [%s]\n", m["name"], status)
		}

	case "hud":
		if len(os.Args) < 3 {
			log.Fatal("Especifica el subcomando de hud: 'show', 'hide', 'state' o 'notify'.")
		}
		subCmd := strings.ToLower(os.Args[2])

		switch subCmd {
		case "show":
			resp, err := ipc.SendCommandRPC(socketPath, "hud.show", nil, "hud-show-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "hide":
			resp, err := ipc.SendCommandRPC(socketPath, "hud.hide", nil, "hud-hide-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "state":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el estado a forzar (ej: 'thinking', 'listening', 'speaking').")
			}
			state := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "hud.force_state", map[string]interface{}{"state": state}, "hud-force-state-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		case "notify":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el mensaje a notificar en el HUD.")
			}
			msg := os.Args[3]
			priority := "normal"
			if len(os.Args) >= 5 {
				priority = os.Args[4]
			}
			resp, err := ipc.SendCommandRPC(socketPath, "hud.notification", map[string]interface{}{"message": msg, "priority": priority}, "hud-notify-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		default:
			log.Fatalf("Subcomando de hud desconocido en rbotctl: '%s'", subCmd)
		}

	case "providers":
		if len(os.Args) < 3 {
			log.Fatal("Uso: rbotctl providers [list|status|use <nombre>]")
		}
		subCmd := strings.ToLower(os.Args[2])

		switch subCmd {
		case "list":
			resp, err := ipc.SendCommandRPC(socketPath, "providers.list", nil, "providers-list-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			providers, ok := resp.Result.([]interface{})
			if !ok {
				fmt.Println(resp.Result)
			} else {
				fmt.Println("\n--- PROVEEDORES LLM REGISTRADOS ---")
				for _, p := range providers {
					if pm, ok := p.(map[string]interface{}); ok {
						marker := " "
						if active, _ := pm["active"].(bool); active {
							marker = "*"
						}
						fmt.Printf("  %s %v (modelo: %v)\n", marker, pm["name"], pm["model"])
					}
				}
			}

		case "status":
			resp, err := ipc.SendCommandRPC(socketPath, "providers.status", nil, "providers-status-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			data, ok := resp.Result.(map[string]interface{})
			if !ok {
				fmt.Println(resp.Result)
			} else {
				fmt.Println("\n--- PROVEEDOR LLM ACTIVO ---")
				fmt.Printf("Proveedor:  %v\n", data["provider"])
				fmt.Printf("Modelo:     %v\n", data["model"])
				fmt.Printf("Estado:     %v\n", data["status"])
			}

		case "use", "set-default":
			if len(os.Args) < 4 {
				log.Fatal("Especifica el nombre del proveedor.")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "providers.use", map[string]interface{}{"name": name}, "providers-use-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		default:
			log.Fatalf("Subcomando de providers desconocido: '%s'", subCmd)
		}

	case "models":
		if len(os.Args) < 3 {
			log.Fatal("Uso: rbotctl models [list [--provider <nombre>]|current|switch [<provider>] <modelo>]")
		}
		subCmd := strings.ToLower(os.Args[2])

		switch subCmd {
		case "list":
			params := map[string]interface{}{}
			for i := 3; i < len(os.Args); i++ {
				if os.Args[i] == "--provider" && i+1 < len(os.Args) {
					params["provider"] = os.Args[i+1]
					i++
				}
			}
			resp, err := ipc.SendCommandRPC(socketPath, "models.list", params, "models-list-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			models, ok := resp.Result.([]interface{})
			if !ok {
				fmt.Println(resp.Result)
			} else {
				fmt.Println("\n--- MODELOS DISPONIBLES ---")
				for _, m := range models {
					if mm, ok := m.(map[string]interface{}); ok {
						tools := ""
						if t, ok := mm["tools"].(bool); ok && t {
							tools = " [tools]"
						}
						sz := ""
						if s, ok := mm["size"].(string); ok && s != "" {
							sz = fmt.Sprintf(" (%s)", s)
						}
						fmt.Printf("  - %v%s%s\n", mm["id"], sz, tools)
					}
				}
			}

		case "current":
			resp, err := ipc.SendCommandRPC(socketPath, "providers.status", nil, "models-current-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			data, ok := resp.Result.(map[string]interface{})
			if !ok {
				fmt.Println(resp.Result)
			} else {
				fmt.Printf("Proveedor: %v\nModelo: %v\nEstado: %v\n", data["provider"], data["model"], data["status"])
			}

		case "switch":
			if len(os.Args) < 4 {
				log.Fatal("Uso: rbotctl models switch [<provider>] <modelo>")
			}
			params := map[string]interface{}{}
			if len(os.Args) >= 5 {
				params["provider"] = os.Args[3]
				params["model"] = os.Args[4]
			} else {
				params["model"] = os.Args[3]
			}
			resp, err := ipc.SendCommandRPC(socketPath, "models.switch", params, "models-switch-req")
			if err != nil {
				log.Fatalf("Error de conexión: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Error del daemon: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)

		default:
			log.Fatalf("Subcomando de models desconocido: '%s'", subCmd)
		}

	default:
		fmt.Printf("Comando desconocido: '%s'\n", cmd)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Uso: rbotctl <comando> [argumentos]")
	fmt.Println("\nEstado y control del daemon:")
	fmt.Println("  status                     Muestra el estado del daemon rbotd en ejecución.")
	fmt.Println("  say \"<mensaje>\"            Envía una orden al daemon para que la procese e informe.")
	fmt.Println("\nConfiguración LLM:")
	fmt.Println("  settings <providers|models|status> Alias único para la configuración.")
	fmt.Println("  providers list             Lista proveedores LLM registrados.")
	fmt.Println("  providers status           Muestra el proveedor LLM activo.")
	fmt.Println("  providers use <nombre>     Cambia el proveedor LLM activo.")
	fmt.Println("  providers set-default <n>  Alias de providers use.")
	fmt.Println("  models list [--provider p] Lista modelos del proveedor activo o del indicado.")
	fmt.Println("  models current             Muestra proveedor/modelo activo.")
	fmt.Println("  models switch <modelo>     Cambia el modelo activo del proveedor actual.")
	fmt.Println("  models switch <prov> <mod> Cambia proveedor y modelo activos.")
	fmt.Println("\nAutomatización y workspace:")
	fmt.Println("  skills list                Muestra las habilidades registradas en el daemon.")
	fmt.Println("  skills info <nombre>       Muestra información detallada de una habilidad.")
	fmt.Println("  skills install <zipPath>   Instala una habilidad localmente desde un ZIP.")
	fmt.Println("  skills enable <nombre>     Habilita una habilidad en el daemon.")
	fmt.Println("  skills disable <nombre>    Deshabilita una habilidad en el daemon.")
	fmt.Println("  skills trust <nombre>      Marca una habilidad como confiable.")
	fmt.Println("  skills quarantine <nombre> Envía una habilidad a cuarentena.")
	fmt.Println("  workspace status           Muestra el estado del workspace.")
	fmt.Println("  workspace reload           Fuerza la recarga de los archivos del workspace.")
	fmt.Println("  workspace validate         Valida las políticas y macros del workspace.")
	fmt.Println("  shortcuts list             Lista los atajos de teclado y macros configurados.")
	fmt.Println("\nHUD:")
	fmt.Println("  hud show                   Muestra la interfaz HUD.")
	fmt.Println("  hud hide                   Oculta la interfaz HUD.")
	fmt.Println("  hud state <estado>         Fuerza un estado en el orbe del HUD (ej: thinking).")
	fmt.Println("  hud notify \"<msg>\" [prior] Envía una notificación directa al HUD.")
	fmt.Println("\nMCP:")
	fmt.Println("  mcp list                   Muestra los servidores MCP activos en el daemon.")
}
