package hud

import (
	"context"
	"fmt"
	"time"
)

type VisualUpdate struct {
	State        HUDState
	Text         string
	AudioLevel   float64
	Notification *HUDNotification
	Visible      *bool
}

type EventMapper struct {
	isAwake         bool
	currentState    HUDState
	queue           *NotificationQueue
	audioSmoothing  float64
	prevAudioLevel  float64
	visible         bool
}

func NewEventMapper(audioSmoothing float64) *EventMapper {
	if audioSmoothing <= 0 || audioSmoothing >= 1 {
		audioSmoothing = 0.75
	}
	return &EventMapper{
		currentState:   HUDSleeping,
		queue:          NewNotificationQueue(),
		audioSmoothing: audioSmoothing,
		visible:        false,
	}
}

func (m *EventMapper) Process(ctx context.Context, input <-chan UIEvent, output chan<- VisualUpdate) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Periódicamente verificar si la notificación activa cambió
			activeNotif := m.queue.GetActive()
			// Si no hay actualizaciones de estado pero cambió la notificación activa, emitir actualización
			output <- VisualUpdate{
				State:        m.currentState,
				Notification: activeNotif,
			}

		case ev, ok := <-input:
			if !ok {
				return
			}

			update := m.mapEvent(ev)
			if update != nil {
				select {
				case output <- *update:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (m *EventMapper) mapEvent(ev UIEvent) *VisualUpdate {
	var state *HUDState
	var text string
	var audioLvl *float64
	var visible *bool
	var newNotif *HUDNotification

	// Valor para visibilidad
	visTrue := true
	visFalse := false

	switch ev.Type {
	case "hud.set_state": // Emitido por el cliente en caso de desconexión
		st := HUDState(getStringPayload(ev.Payload, "state", string(HUDDisconnected)))
		state = &st
		if st == HUDDisconnected {
			m.isAwake = false
		}

	case "daemon.started", "daemon.ready":
		st := HUDSleeping
		state = &st
		m.isAwake = false

	case "voice.engine.started", "voice.ready":
		st := HUDSleeping
		state = &st
		m.isAwake = false

	case "voice.listening":
		st := HUDListening
		state = &st
		m.isAwake = true
		visible = &visTrue

	case "voice.audio_level", "tts.audio_level":
		val, ok := ev.Payload["level"].(float64)
		if ok {
			// Aplicar suavizado: smoothed = previous * 0.75 + current * 0.25
			smoothed := m.prevAudioLevel*m.audioSmoothing + val*(1.0-m.audioSmoothing)
			m.prevAudioLevel = smoothed
			audioLvl = &smoothed
		}

	case "voice.transcribed":
		st := HUDTranscribing
		state = &st
		text = getStringPayload(ev.Payload, "text", "")

	case "voice.wake_detected":
		st := HUDWakeDetected
		state = &st
		m.isAwake = true
		visible = &visTrue

	case "voice.sleeping", "voice.timeout":
		st := HUDSleeping
		state = &st
		m.isAwake = false
		m.queue.ClearConfirmations() // Limpiar confirmaciones al dormir

	case "agent.thinking":
		st := HUDThinking
		state = &st
		text = getStringPayload(ev.Payload, "input", "")
		visible = &visTrue

	case "plan.created":
		st := HUDPlanning
		state = &st

	case "tool.started":
		st := HUDExecuting
		state = &st

	case "tts.speaking":
		st := HUDSpeaking
		state = &st
		text = getStringPayload(ev.Payload, "text", "")
		visible = &visTrue

	case "tts.finished":
		if m.isAwake {
			st := HUDListening
			state = &st
		} else {
			st := HUDSleeping
			state = &st
		}

	case "daemon.error":
		st := HUDError
		state = &st
		text = getStringPayload(ev.Payload, "error", "")

	case "hud.show":
		visible = &visTrue

	case "hud.hide":
		visible = &visFalse

	case "hud.force_state":
		st := HUDState(getStringPayload(ev.Payload, "state", string(HUDListening)))
		state = &st
		visible = &visTrue

	case "hud.notification":
		msg := getStringPayload(ev.Payload, "message", "")
		priority := getStringPayload(ev.Payload, "priority", "normal")
		if msg != "" {
			newNotif = &HUDNotification{
				ID:        fmt.Sprintf("notif_%d", time.Now().UnixNano()),
				Type:      "notification",
				Message:   msg,
				Priority:  priority,
				CreatedAt: time.Now(),
				Duration:  8 * time.Second,
			}
			m.queue.Add(newNotif)
			visible = &visTrue
		}

	case "policy.confirmation_required":
		// Crear una confirmación interactiva
		newNotif = &HUDNotification{
			ID:        fmt.Sprintf("confirm_%d", time.Now().UnixNano()),
			Type:      "confirmation",
			Message:   "Esta acción requiere confirmación. ¿Desea proceder?",
			Priority:  "high",
			CreatedAt: time.Now(),
		}
		m.queue.Add(newNotif)
		visible = &visTrue

	case "confirmation.accepted", "confirmation.cancelled", "confirmation.expired":
		// Al procesar confirmación, limpiar cualquier confirmación de la cola
		m.queue.ClearConfirmations()
	}

	// Si no hubo cambio de estado ni visibilidad ni audio ni notificación, retornar nil
	if state == nil && text == "" && audioLvl == nil && visible == nil && newNotif == nil {
		return nil
	}

	up := &VisualUpdate{}
	if state != nil {
		m.currentState = *state
		up.State = *state
	} else {
		up.State = m.currentState
	}

	up.Text = text

	if audioLvl != nil {
		up.AudioLevel = *audioLvl
	}

	if visible != nil {
		m.visible = *visible
		up.Visible = visible
	}

	// Obtener la notificación activa
	up.Notification = m.queue.GetActive()

	return up
}

func getStringPayload(payload map[string]interface{}, key string, def string) string {
	if payload == nil {
		return def
	}
	val, ok := payload[key]
	if !ok {
		return def
	}
	str, ok := val.(string)
	if !ok {
		return def
	}
	return str
}
