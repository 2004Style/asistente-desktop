package llm

import (
	"context"
	"testing"
)

// mockProvider implements Provider for testing
type mockProvider struct {
	name    string
	model   string
	pingErr error
}

func (m *mockProvider) Name() string       { return m.name }
func (m *mockProvider) ModelID() string    { return m.model }
func (m *mockProvider) SetModel(id string) { m.model = id }
func (m *mockProvider) Chat(ctx context.Context, messages []Message, tools []Tool, opts ChatOptions) (*Message, error) {
	return &Message{Role: "assistant", Content: "mock response from " + m.name}, nil
}
func (m *mockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: m.model, Name: m.model, Provider: m.name},
	}, nil
}
func (m *mockProvider) Ping(ctx context.Context) error { return m.pingErr }

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	p := &mockProvider{name: "test-provider", model: "test-model"}
	err := reg.Register(p)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Should fail on duplicate
	err = reg.Register(p)
	if err == nil {
		t.Error("Expected error on duplicate registration")
	}

	// Get should find it
	got, ok := reg.Get("test-provider")
	if !ok {
		t.Fatal("Expected to find provider")
	}
	if got.Name() != "test-provider" {
		t.Errorf("Expected name 'test-provider', got %q", got.Name())
	}

	// Get unknown should not find it
	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent provider")
	}
}

func TestRegistryRegisterOrReplace(t *testing.T) {
	reg := NewRegistry()

	p1 := &mockProvider{name: "test", model: "model-1"}
	p2 := &mockProvider{name: "test", model: "model-2"}

	_ = reg.Register(p1)
	reg.RegisterOrReplace(p2)

	got, ok := reg.Get("test")
	if !ok {
		t.Fatal("Expected to find provider")
	}
	if got.ModelID() != "model-2" {
		t.Errorf("Expected model 'model-2', got %q", got.ModelID())
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "a", model: "m1"})
	_ = reg.Register(&mockProvider{name: "b", model: "m2"})

	list := reg.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(list))
	}

	names := reg.Names()
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(names))
	}
}

func TestRegistryRemove(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "remove-me", model: "m"})

	reg.Remove("remove-me")
	_, ok := reg.Get("remove-me")
	if ok {
		t.Error("Expected provider to be removed")
	}
}

func TestManagerSetActiveAndChat(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "provider-a", model: "model-a"})
	_ = reg.Register(&mockProvider{name: "provider-b", model: "model-b"})

	mgr := NewManager(nil, reg)

	// No active provider yet
	if mgr.Active() != nil {
		t.Error("Expected no active provider initially")
	}

	// Set active
	err := mgr.SetActive("provider-a")
	if err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	if mgr.ActiveName() != "provider-a" {
		t.Errorf("Expected active 'provider-a', got %q", mgr.ActiveName())
	}

	// Chat should work
	resp, err := mgr.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, ChatOptions{})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if resp.Content != "mock response from provider-a" {
		t.Errorf("Expected mock response from provider-a, got %q", resp.Content)
	}

	// Switch model
	err = mgr.SwitchModel("new-model")
	if err != nil {
		t.Fatalf("SwitchModel failed: %v", err)
	}
	if mgr.Active().ModelID() != "new-model" {
		t.Errorf("Expected model 'new-model', got %q", mgr.Active().ModelID())
	}
}

func TestManagerSetActiveInvalid(t *testing.T) {
	reg := NewRegistry()
	mgr := NewManager(nil, reg)

	err := mgr.SetActive("nonexistent")
	if err == nil {
		t.Error("Expected error setting nonexistent provider as active")
	}
}

func TestManagerChatWithNoActiveProvider(t *testing.T) {
	reg := NewRegistry()
	mgr := NewManager(nil, reg)

	_, err := mgr.Chat(context.Background(), nil, nil, ChatOptions{})
	if err == nil {
		t.Error("Expected error when no active provider")
	}
}

func TestManagerListModels(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "test", model: "test-model"})

	mgr := NewManager(nil, reg)
	_ = mgr.SetActive("test")

	models, err := mgr.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
	if models[0].ID != "test-model" {
		t.Errorf("Expected model 'test-model', got %q", models[0].ID)
	}
}

func TestManagerListModelsForProviderDoesNotChangeActive(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "a", model: "ma"})
	_ = reg.Register(&mockProvider{name: "b", model: "mb"})

	mgr := NewManager(nil, reg)
	if err := mgr.SetActive("a"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}

	models, err := mgr.ListModelsForProvider(context.Background(), "b")
	if err != nil {
		t.Fatalf("ListModelsForProvider failed: %v", err)
	}
	if len(models) != 1 || models[0].ID != "mb" {
		t.Fatalf("expected provider b models, got %#v", models)
	}
	if mgr.ActiveName() != "a" {
		t.Fatalf("provider-scoped list must not change active provider, got %q", mgr.ActiveName())
	}
}

func TestManagerSwitchProviderModelSetsActiveProviderAndModel(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "a", model: "ma"})
	_ = reg.Register(&mockProvider{name: "b", model: "mb"})

	mgr := NewManager(nil, reg)
	if err := mgr.SetActive("a"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}
	if err := mgr.SwitchProviderModel("b", "mb-new"); err != nil {
		t.Fatalf("SwitchProviderModel failed: %v", err)
	}
	if mgr.ActiveName() != "b" {
		t.Fatalf("expected active provider b, got %q", mgr.ActiveName())
	}
	if mgr.Active().ModelID() != "mb-new" {
		t.Fatalf("expected model mb-new, got %q", mgr.Active().ModelID())
	}
}

func TestManagerListAllModels(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(&mockProvider{name: "a", model: "ma"})
	_ = reg.Register(&mockProvider{name: "b", model: "mb"})

	mgr := NewManager(nil, reg)
	_ = mgr.SetActive("a")

	all, err := mgr.ListAllModels(context.Background())
	if err != nil {
		t.Fatalf("ListAllModels failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("Expected models from 2 providers, got %d", len(all))
	}
}
