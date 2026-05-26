package policy

import (
	"strings"
	"time"

	"rbot/internal/planner"
)

type PendingConfirmation struct {
	PlanID    string
	Plan      *planner.Plan
	Reason    string
	ExpiresAt time.Time
}

// AddPending agrega un plan a la cola en memoria global.
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

// GetPending recupera el plan de la cola en memoria global.
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

// ClearPending vacía la cola en memoria global.
func (e *Engine) ClearPending() {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pending, "global")
}

// IsAffirmative determina si una frase representa una respuesta afirmativa.
func IsAffirmative(input string) bool {
	input = strings.ToLower(strings.Trim(input, " .,?!¡¿"))
	input = strings.ReplaceAll(input, ",", " ")
	words := strings.Fields(input)
	if len(words) > 0 {
		firstWord := words[0]
		affirmatives := []string{
			"sí", "si", "afirmativo", "confirmo", "aceptar", "acepto", "ejecuta", "dale", "procede", "proceder", "ok", "okay", "claro", "por supuesto", "está bien",
		}
		for _, val := range affirmatives {
			if firstWord == val {
				return true
			}
		}
	}
	return false
}

// IsNegative determina si una frase representa una respuesta negativa o cancelación.
func IsNegative(input string) bool {
	input = strings.ToLower(strings.Trim(input, " .,?!¡¿"))
	input = strings.ReplaceAll(input, ",", " ")
	words := strings.Fields(input)
	if len(words) > 0 {
		firstWord := words[0]
		negatives := []string{
			"no", "negativo", "cancela", "cancelar", "rechazo", "rechazar", "parar", "detener", "no ejecutes", "no hacer", "nopo", "nop",
		}
		for _, val := range negatives {
			if firstWord == val {
				return true
			}
		}
	}
	return false
}

