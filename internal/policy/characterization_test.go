package policy_test

import (
	"context"
	"testing"

	"rbot/internal/agent"
	"rbot/internal/db"
	"rbot/internal/llm"
	"rbot/internal/security"
)

// minimal test LLM provider
type testProvider struct {
	name  string
	model string
}

func (t *testProvider) Name() string       { return t.name }
func (t *testProvider) ModelID() string    { return t.model }
func (t *testProvider) SetModel(id string) { t.model = id }
func (t *testProvider) Chat(ctx context.Context, messages []llm.Message, tools []llm.Tool, opts llm.ChatOptions) (*llm.Message, error) {
	return &llm.Message{Role: "assistant", Content: "mock"}, nil
}
func (t *testProvider) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return []llm.ModelInfo{{ID: t.model, Name: t.model, Provider: t.name}}, nil
}
func (t *testProvider) Ping(ctx context.Context) error { return nil }

func TestDirectAndExecutorPathAgreement(t *testing.T) {
	// DB in memory
	sqliteDB, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("DB init failed: %v", err)
	}
	defer sqliteDB.Close()

	p := &testProvider{name: "test", model: "m"}
	o := agent.NewOrchestrator(sqliteDB, p, nil, []string{"~/.ssh"}, []string{"."}, "TestBot", nil)

	// Characterization: compare executor policy vs security helper for the same tool/args
	toolHandler, ok := o.Registry.Get("system.run_command_safe")
	if !ok {
		t.Fatalf("tool not registered: system.run_command_safe")
	}
	args := map[string]interface{}{"path": "~/.ssh/id_rsa"}
	decision := o.Executor.Policy.EvaluateTool(context.Background(), toolHandler, args)

	// Security helper - pass the path as targetPath
	allowed2, requiresConfirm2, _ := security.ValidateToolAction(o.DB, toolHandler.Name(), "~/.ssh/id_rsa", o.BlockedPaths)

	// They should agree on allowed/confirm for path-based checks
	if decision.Allowed != allowed2 {
		t.Fatalf("Policy vs security helper divergence: policy.Allowed=%v, helperAllowed=%v", decision.Allowed, allowed2)
	}
	if decision.RequiresConfirm != requiresConfirm2 {
		t.Fatalf("Policy vs security helper divergence on RequiresConfirm: %v vs %v", decision.RequiresConfirm, requiresConfirm2)
	}
}
