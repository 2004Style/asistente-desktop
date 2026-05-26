package hud

import (
	"sync"
	"time"
)

type HUDNotification struct {
	ID        string        `json:"id"`
	Type      string        `json:"type"` // "notification" | "confirmation"
	Message   string        `json:"message"`
	Priority  string        `json:"priority"` // "critical" | "high" | "normal" | "low"
	CreatedAt time.Time     `json:"created_at"`
	Duration  time.Duration `json:"duration"`
}

func (n *HUDNotification) IsExpired() bool {
	if n.Type == "confirmation" {
		return false // Las confirmaciones no expiran solas a menos que el daemon mande evento
	}
	return time.Since(n.CreatedAt) > n.Duration
}

type NotificationQueue struct {
	mu    sync.Mutex
	items []*HUDNotification
}

func NewNotificationQueue() *NotificationQueue {
	return &NotificationQueue{
		items: make([]*HUDNotification, 0),
	}
}

func (q *NotificationQueue) Add(n *HUDNotification) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Si ya existe el ID, no duplicarlo
	for _, item := range q.items {
		if item.ID == n.ID {
			item.Message = n.Message
			item.Priority = n.Priority
			item.CreatedAt = n.CreatedAt
			item.Duration = n.Duration
			return
		}
	}

	q.items = append(q.items, n)
}

func (q *NotificationQueue) Remove(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.items {
		if item.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return
		}
	}
}

func (q *NotificationQueue) ClearConfirmations() {
	q.mu.Lock()
	defer q.mu.Unlock()

	var active []*HUDNotification
	for _, item := range q.items {
		if item.Type != "confirmation" {
			active = append(active, item)
		}
	}
	q.items = active
}

func (q *NotificationQueue) GetActive() *HUDNotification {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Limpiar expirados
	var active []*HUDNotification
	for _, item := range q.items {
		if !item.IsExpired() {
			active = append(active, item)
		}
	}
	q.items = active

	if len(q.items) == 0 {
		return nil
	}

	// Buscar el de mayor prioridad
	// Prioridad: critical > high > normal > low
	priorityVal := func(p string) int {
		switch p {
		case "critical":
			return 4
		case "high":
			return 3
		case "normal":
			return 2
		case "low":
			return 1
		default:
			return 0
		}
	}

	bestIdx := 0
	bestVal := priorityVal(q.items[0].Priority)

	// Confirmaciones tienen prioridad por encima de notificaciones del mismo nivel
	if q.items[0].Type == "confirmation" {
		bestVal += 10
	}

	for i := 1; i < len(q.items); i++ {
		val := priorityVal(q.items[i].Priority)
		if q.items[i].Type == "confirmation" {
			val += 10
		}
		if val > bestVal {
			bestVal = val
			bestIdx = i
		}
	}

	return q.items[bestIdx]
}
