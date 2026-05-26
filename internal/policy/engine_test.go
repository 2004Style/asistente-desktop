package policy_test

import (
	"context"
	"testing"

	"rbot/internal/config"
	"rbot/internal/executor"
	"rbot/internal/policy"
)

type dummyTool struct {
	name      string
	riskLevel string
}

func (t *dummyTool) Name() string                   { return t.name }
func (t *dummyTool) Description() string            { return "Dummy Tool" }
func (t *dummyTool) Category() string               { return "dummy" }
func (t *dummyTool) RiskLevel() string              { return t.riskLevel }
func (t *dummyTool) Schema() map[string]interface{} { return nil }
func (t *dummyTool) Execute(ctx context.Context, args map[string]interface{}) (*executor.ToolResult, error) {
	return &executor.ToolResult{Success: true}, nil
}

func TestPolicyEngineEvaluateTool(t *testing.T) {
	engine := policy.NewEngine([]string{"~/.ssh", "/etc/shadow"}, true)

	// Test 1: Low risk tool on a safe path
	lowTool := &dummyTool{name: "files.read_file", riskLevel: "low"}
	decision := engine.EvaluateTool(context.Background(), lowTool, map[string]interface{}{"path": "~/Documentos/notas.txt"})
	if !decision.Allowed {
		t.Error("Expected low risk tool on safe path to be allowed")
	}
	if decision.RequiresConfirm {
		t.Error("Expected low risk tool on safe path not to require confirmation")
	}

	// Test 2: Blocked path
	decision = engine.EvaluateTool(context.Background(), lowTool, map[string]interface{}{"path": "~/.ssh/id_rsa"})
	if decision.Allowed {
		t.Error("Expected blocked path tool to be denied")
	}
	if decision.RiskLevel != "critical" {
		t.Errorf("Expected critical risk for blocked path, got %s", decision.RiskLevel)
	}

	// Test 3: Critical command validation in system.run_command_safe
	execTool := &dummyTool{name: "system.run_command_safe", riskLevel: "high"}
	decision = engine.EvaluateTool(context.Background(), execTool, map[string]interface{}{"command": "sudo rm -rf /"})
	if decision.Allowed {
		t.Error("Expected critical command tool to be blocked preventively (Allowed: false)")
	}
	if decision.RequiresConfirm {
		t.Error("Expected critical command tool not to require confirmation as it is blocked")
	}
	if decision.RiskLevel != "critical" {
		t.Errorf("Expected escalated critical risk level, got %s", decision.RiskLevel)
	}
}

func TestDefaultBlockedPathsDenySensitiveLocations(t *testing.T) {
	conf := config.DefaultConfig()
	config.NormalizeConfig(conf)
	engine := policy.NewEngine(conf.Security.BlockedPaths, true)
	lowTool := &dummyTool{name: "files.read_file", riskLevel: "low"}

	sensitivePaths := []string{
		"~/.ssh/id_rsa",
		"~/.gnupg/private-keys-v1.d/key.key",
		"~/.aws/credentials",
		"~/.config/gh/hosts.yml",
		"~/.config/rclone/rclone.conf",
		"~/.docker/config.json",
		"project/.env.local",
		"project/secrets.pem",
		"project/token.key",
	}

	for _, path := range sensitivePaths {
		decision := engine.EvaluateTool(context.Background(), lowTool, map[string]interface{}{"path": path})
		if decision.Allowed {
			t.Fatalf("expected sensitive path %q to be blocked", path)
		}
		if decision.RiskLevel != "critical" {
			t.Fatalf("expected critical risk for %q, got %s", path, decision.RiskLevel)
		}
	}
}

func TestWorkspacePolicyAndImmutableRules(t *testing.T) {
	engine := policy.NewEngine([]string{}, true)

	// Test case: Immutable rules block "rm -rf /" command
	cmdTool := &dummyTool{name: "system.run_command", riskLevel: "high"}
	decision := engine.EvaluateTool(context.Background(), cmdTool, map[string]interface{}{"CommandLine": "rm -rf /"})
	if decision.Allowed {
		t.Error("Expected rm -rf / to be blocked by immutable rules")
	}
	if decision.RiskLevel != "critical" {
		t.Errorf("Expected critical risk for immutable rule breach, got %s", decision.RiskLevel)
	}

	// Test case: Workspace policies parsed and enforced
	policyContent := `- No ejecutar comandos shell sin confirmación del usuario.
- Confirmar siempre antes de cerrar ventanas del sistema.
- Bloquear clics de mouse automáticos.`
	engine.SetWorkspacePolicies(policyContent)

	// 1. Tool matching "comando shell" (run_command) should now require confirmation due to local policy
	decision = engine.EvaluateTool(context.Background(), cmdTool, map[string]interface{}{"CommandLine": "ls -la"})
	if !decision.Allowed {
		t.Error("Expected command to be allowed (but require confirmation)")
	}
	if !decision.RequiresConfirm {
		t.Error("Expected command to require confirmation by local policy")
	}

	// 2. Tool matching "cerrar ventana" (close_window)
	closeTool := &dummyTool{name: "desktop.close_window", riskLevel: "low"}
	decision = engine.EvaluateTool(context.Background(), closeTool, nil)
	if !decision.Allowed {
		t.Error("Expected close_window to be allowed (but require confirmation)")
	}
	if !decision.RequiresConfirm {
		t.Error("Expected close_window to require confirmation by local policy")
	}

	// 3. Tool matching "clic" / "mouse" (mouse.click) should be blocked by "Bloquear clics de mouse automáticos."
	clickTool := &dummyTool{name: "input.mouse.click", riskLevel: "low"}
	decision = engine.EvaluateTool(context.Background(), clickTool, nil)
	if decision.Allowed {
		t.Error("Expected mouse.click to be blocked by local policy")
	}
}
