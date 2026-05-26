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
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "rbot", "rbot.yaml")
	if _, err := os.Stat("config/rbot.yaml"); err == nil {
		configPath = "config/rbot.yaml"
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	socketPath := db.ExpandPath(conf.Runtime.SocketPath)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "providers":
		if len(os.Args) < 3 {
			printUsage()
			return
		}
		sub := strings.ToLower(os.Args[2])
		switch sub {
		case "list":
			resp, err := ipc.SendCommandRPC(socketPath, "providers.list", nil, "providers-list-req")
			if err != nil {
				log.Fatalf("Error connecting to daemon: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Daemon error: %s", resp.Error.Message)
			}
			providers, ok := resp.Result.([]interface{})
			if !ok {
				fmt.Println(resp.Result)
				return
			}
			fmt.Println("Registered providers:")
			for _, p := range providers {
				pm := p.(map[string]interface{})
				marker := " "
				if active, _ := pm["active"].(bool); active {
					marker = "*"
				}
				fmt.Printf(" %s %s (model: %v)\n", marker, pm["name"], pm["model"])
			}
		case "status":
			resp, err := ipc.SendCommandRPC(socketPath, "providers.status", nil, "providers-status-req")
			if err != nil {
				log.Fatalf("Error connecting to daemon: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Daemon error: %s", resp.Error.Message)
			}
			m := resp.Result.(map[string]interface{})
			fmt.Printf("Provider: %v\nModel: %v\nStatus: %v\n", m["provider"], m["model"], m["status"])
		case "use":
			if len(os.Args) < 4 {
				log.Fatalf("Usage: rbot-settings providers use <name>")
			}
			name := os.Args[3]
			resp, err := ipc.SendCommandRPC(socketPath, "providers.use", map[string]interface{}{"name": name}, "providers-use-req")
			if err != nil {
				log.Fatalf("Error connecting to daemon: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Daemon error: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)
		default:
			printUsage()
		}
	case "models":
		if len(os.Args) < 3 {
			printUsage()
			return
		}
		sub := strings.ToLower(os.Args[2])
		switch sub {
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
				log.Fatalf("Error connecting to daemon: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Daemon error: %s", resp.Error.Message)
			}
			list, ok := resp.Result.([]interface{})
			if !ok {
				fmt.Println(resp.Result)
				return
			}
			for _, m := range list {
				mm := m.(map[string]interface{})
				tools := ""
				if t, ok := mm["tools"].(bool); ok && t {
					tools = " [tools]"
				}
				sz := ""
				if s, ok := mm["size"].(string); ok && s != "" {
					sz = fmt.Sprintf(" (%s)", s)
				}
				fmt.Printf("- %v%s%s\n", mm["id"], sz, tools)
			}
		case "switch":
			if len(os.Args) < 4 {
				log.Fatalf("Usage: rbot-settings models switch [<provider>] <model>")
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
				log.Fatalf("Error connecting to daemon: %v", err)
			}
			if resp.Error != nil {
				log.Fatalf("Daemon error: %s", resp.Error.Message)
			}
			fmt.Println(resp.Result)
		default:
			printUsage()
		}
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: rbot-settings <command> [args]")
	fmt.Println("Commands:")
	fmt.Println("  providers list")
	fmt.Println("  providers status")
	fmt.Println("  providers use <name>")
	fmt.Println("  models list [--provider <name>]")
	fmt.Println("  models switch [<provider>] <model>")
}
