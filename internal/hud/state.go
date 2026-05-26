package hud

type HUDState string

const (
	HUDSleeping     HUDState = "sleeping"
	HUDWakeDetected HUDState = "wake_detected"
	HUDListening    HUDState = "listening"
	HUDTranscribing HUDState = "transcribing"
	HUDThinking     HUDState = "thinking"
	HUDPlanning     HUDState = "planning"
	HUDExecuting    HUDState = "executing"
	HUDSpeaking     HUDState = "speaking"
	HUDError        HUDState = "error"
	HUDStateNotification HUDState = "notification"
	HUDDisconnected HUDState = "disconnected"
)
