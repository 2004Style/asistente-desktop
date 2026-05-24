package voice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartVoiceEngine(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-voice-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	piperModel := filepath.Join(tempDir, "piper.onnx")
	whisperModel := filepath.Join(tempDir, "whisper.bin")
	_ = os.WriteFile(piperModel, []byte("piper dummy"), 0644)
	_ = os.WriteFile(whisperModel, []byte("whisper dummy"), 0644)

	err = StartVoiceEngine(tempDir, "piper.onnx", "whisper.bin", 500.0)
	if err != nil {
		t.Fatalf("StartVoiceEngine failed: %v", err)
	}

	if vadThreshold != 500.0 {
		t.Errorf("Expected vadThreshold to be 500.0, got %f", vadThreshold)
	}

	if piperModelPath != piperModel {
		t.Errorf("Expected piperModelPath %q, got %q", piperModel, piperModelPath)
	}

	if whisperModelPath != whisperModel {
		t.Errorf("Expected whisperModelPath %q, got %q", whisperModel, whisperModelPath)
	}
}

func TestSpeakFallback(t *testing.T) {
	// Test empty text
	err := Speak("")
	if err != nil {
		t.Errorf("Speak with empty string returned error: %v", err)
	}

	// Test fallback path when hasPiper is false
	oldHasPiper := hasPiper
	hasPiper = false
	defer func() { hasPiper = oldHasPiper }()

	err = Speak("hello")
	if err != nil {
		t.Errorf("Speak fallback returned error: %v", err)
	}
}

func TestListenInteractiveFallback(t *testing.T) {
	// Force dependencies to be false
	oldHasWhisperCli := hasWhisperCli
	hasWhisperCli = false
	defer func() { hasWhisperCli = oldHasWhisperCli }()

	// Redirect stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Write input to the write end of the pipe
	inputLine := "hola asistente\n"
	_, _ = w.WriteString(inputLine)
	w.Close()

	res, err := Listen()
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	if res != "hola asistente" {
		t.Errorf("Expected 'hola asistente', got %q", res)
	}
}

func TestWriteWavFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rbot-voice-wav")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	wavPath := filepath.Join(tempDir, "test.wav")
	pcmData := make([]byte, 1600) // 0.05s of silent samples
	
	err = writeWavFile(wavPath, pcmData, 16000)
	if err != nil {
		t.Fatalf("writeWavFile failed: %v", err)
	}

	info, err := os.Stat(wavPath)
	if err != nil {
		t.Fatalf("Wav file was not created: %v", err)
	}

	// 44 bytes header + 1600 bytes PCM = 1644 bytes
	if info.Size() != 1644 {
		t.Errorf("Expected wav file size to be 1644, got %d", info.Size())
	}
}

func TestPauseResumeMedia(t *testing.T) {
	// Should not panic or block
	PauseMedia()
	ResumeMedia()
}

func TestStopVoiceEngine(t *testing.T) {
	StopVoiceEngine()
}
