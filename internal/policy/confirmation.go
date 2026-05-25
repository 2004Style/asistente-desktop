package policy

import (
	"sync"
	"time"

	"rbot/internal/planner"
)

type PendingConfirmation struct {
	PlanID    string
	Plan      *planner.Plan
	Reason    string
	ExpiresAt time.Time
}

type Engine struct {
	mu      sync.Mutex
	pending map[string]*PendingConfirmation // Key by UserID or "global" para este agente local
}

func NewEngine() *Engine {
	return &Engine{
		pending: make(map[string]*PendingConfirmation),
	}
}

func (e *Engine) AddPending(plan *planner.Plan, reason string, ttl time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pending["global"] = &PendingConfirmation{
		PlanID:    plan.ID,
		Plan:      plan,
		Reason:    reason,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (e *Engine) GetPending() *PendingConfirmation {
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pending["global"]
	if !ok {
		return nil
	}
	if time.Now().After(p.ExpiresAt) {
		delete(e.pending, "global")
		return nil
	}
	return p
}

func (e *Engine) ClearPending() {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pending, "global")
}
