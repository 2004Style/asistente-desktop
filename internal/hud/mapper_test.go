package hud

import (
	"context"
	"testing"
	"time"
)

func TestEventMapper_SleepingToWake(t *testing.T) {
	mapper := NewEventMapper(0.75)

	// Simular evento wake word
	ev := UIEvent{
		Type:      "voice.wake_detected",
		Timestamp: time.Now(),
		Payload:   map[string]interface{}{"word": "ronald"},
	}

	up := mapper.mapEvent(ev)
	if up == nil {
		t.Fatal("Esperaba VisualUpdate y retornó nil")
	}

	if up.State != HUDWakeDetected {
		t.Errorf("Estado incorrecto: esperado %v, obtenido %v", HUDWakeDetected, up.State)
	}

	if up.Visible == nil || !*up.Visible {
		t.Errorf("Visibilidad incorrecta: esperado true")
	}
}

func TestEventMapper_AudioLevelSmoothing(t *testing.T) {
	mapper := NewEventMapper(0.75) // 0.75 smoothing

	ev1 := UIEvent{
		Type:    "voice.audio_level",
		Payload: map[string]interface{}{"level": 0.8},
	}
	up1 := mapper.mapEvent(ev1)
	expected1 := 0.0*0.75 + 0.8*0.25 // 0.20
	if up1 == nil || up1.AudioLevel != expected1 {
		t.Errorf("AudioLevel 1 incorrecto: esperado %v, obtenido %v", expected1, up1.AudioLevel)
	}

	ev2 := UIEvent{
		Type:    "voice.audio_level",
		Payload: map[string]interface{}{"level": 0.4},
	}
	up2 := mapper.mapEvent(ev2)
	expected2 := expected1*0.75 + 0.4*0.25 // 0.20*0.75 + 0.10 = 0.25
	if up2 == nil || up2.AudioLevel != expected2 {
		t.Errorf("AudioLevel 2 incorrecto: esperado %v, obtenido %v", expected2, up2.AudioLevel)
	}
}

func TestEventMapper_TTSFlow(t *testing.T) {
	mapper := NewEventMapper(0.75)

	// Iniciar TTS
	up1 := mapper.mapEvent(UIEvent{
		Type:    "tts.speaking",
		Payload: map[string]interface{}{"text": "Hola señor"},
	})
	if up1 == nil || up1.State != HUDSpeaking || up1.Text != "Hola señor" {
		t.Errorf("TTS start incorrecto: %+v", up1)
	}

	// Terminar TTS (no despierto por voz previamente)
	up2 := mapper.mapEvent(UIEvent{
		Type: "tts.finished",
	})
	if up2 == nil || up2.State != HUDSleeping || up2.Visible == nil || *up2.Visible {
		t.Errorf("TTS finish (not awake) incorrecto: %+v", up2)
	}

	// Marcar despierto primero
	mapper.mapEvent(UIEvent{Type: "voice.listening"})

	// Hablar
	mapper.mapEvent(UIEvent{
		Type:    "tts.speaking",
		Payload: map[string]interface{}{"text": "Hola"},
	})

	// Terminar TTS (despierto)
	up3 := mapper.mapEvent(UIEvent{
		Type: "tts.finished",
	})
	if up3 == nil || up3.State != HUDListening {
		t.Errorf("TTS finish (awake) incorrecto: %+v", up3)
	}
}

func TestEventMapper_ProcessChannel(t *testing.T) {
	mapper := NewEventMapper(0.75)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan UIEvent, 5)
	output := make(chan VisualUpdate, 5)

	go mapper.Process(ctx, input, output)

	input <- UIEvent{
		Type:    "hud.force_state",
		Payload: map[string]interface{}{"state": "thinking"},
	}

	select {
	case up := <-output:
		if up.State != HUDThinking {
			t.Errorf("Estado en canal incorrecto: esperado %v, obtenido %v", HUDThinking, up.State)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout esperando VisualUpdate del canal")
	}
}
