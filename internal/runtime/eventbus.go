package runtime

import (
	"sync"
	"time"
)

// Event representa un suceso o cambio de estado en el sistema
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// EventBus provee un mecanismo seguro para concurrencia de Publicar/Suscribir
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan Event]bool
}

// NewEventBus inicializa un bus de eventos vacío
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan Event]bool),
	}
}

// Subscribe crea y registra un canal para recibir eventos
func (eb *EventBus) Subscribe() chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, 200) // Buffer holgado para evitar bloquear publicadores
	eb.subscribers[ch] = true
	return ch
}

// Unsubscribe elimina el canal de los suscriptores y lo cierra
func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if _, exists := eb.subscribers[ch]; exists {
		delete(eb.subscribers, ch)
		close(ch)
	}
}

// Publish envía un evento a todos los suscriptores activos sin bloquear
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	for ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Si el lector va muy lento, no bloqueamos el hilo principal del daemon
		}
	}
}
