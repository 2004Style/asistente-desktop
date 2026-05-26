package runtime

import (
	"testing"
	"time"
)

func TestEventBusPublishSubscribe(t *testing.T) {
	eb := NewEventBus()

	ch1 := eb.Subscribe()
	ch2 := eb.Subscribe()

	ev := Event{
		Type:    "test.event",
		Payload: map[string]interface{}{"foo": "bar"},
	}

	eb.Publish(ev)

	// Verificar recepción ch1
	select {
	case received := <-ch1:
		if received.Type != "test.event" || received.Payload["foo"] != "bar" {
			t.Errorf("Evento incorrecto recibido en ch1: %v", received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout esperando evento en ch1")
	}

	// Verificar recepción ch2
	select {
	case received := <-ch2:
		if received.Type != "test.event" || received.Payload["foo"] != "bar" {
			t.Errorf("Evento incorrecto recibido en ch2: %v", received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout esperando evento en ch2")
	}

	// Desuscribir ch1
	eb.Unsubscribe(ch1)

	// Volver a publicar
	eb.Publish(Event{Type: "another.event"})

	// ch1 debe estar cerrado y retornar inmediato
	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("ch1 no debería recibir más eventos")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout esperando cierre de ch1")
	}

	// ch2 debe recibir el nuevo evento
	select {
	case received := <-ch2:
		if received.Type != "another.event" {
			t.Errorf("Evento incorrecto en ch2: %v", received)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout esperando nuevo evento en ch2")
	}
}
