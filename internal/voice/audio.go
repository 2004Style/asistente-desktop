package voice

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	piperModelPath   string
	whisperModelPath string
	hasRec           bool
	hasArecord       bool
	hasPiper         bool
	piperBin         string
	hasWhisperCli    bool
	vadThreshold     float64 = 550.0
	WhisperThreads   int     = 4
	WhisperFlags     string  = ""

	OnAudioLevel     func(level float64) // callback para reportar volumen del micrófono
)

// StartVoiceEngine inicializa el motor de voz de Go validando los binarios disponibles.
func StartVoiceEngine(projectRoot string, piperModel string, whisperModel string, vadThresh float64) error {
	if vadThresh > 0 {
		vadThreshold = vadThresh
	}
	// Guardar rutas absolutas de modelos
	if filepath.IsAbs(piperModel) {
		piperModelPath = piperModel
	} else {
		piperModelPath = filepath.Join(projectRoot, piperModel)
	}

	if filepath.IsAbs(whisperModel) {
		whisperModelPath = whisperModel
	} else {
		whisperModelPath = filepath.Join(projectRoot, whisperModel)
	}

	// 1. Validar grabadores
	if _, err := exec.LookPath("rec"); err == nil {
		hasRec = true
	}
	if _, err := exec.LookPath("arecord"); err == nil {
		hasArecord = true
	}

	// 2. Validar sintetizador Piper
	if _, err := exec.LookPath("piper"); err == nil {
		hasPiper = true
		piperBin = "piper"
	} else if _, err := exec.LookPath("piper-tts"); err == nil {
		hasPiper = true
		piperBin = "piper-tts"
	}

	// 3. Validar transcriptor Whisper
	if _, err := exec.LookPath("whisper-cli"); err == nil {
		hasWhisperCli = true
	} else if _, err := exec.LookPath("whisper-cpp"); err == nil {
		hasWhisperCli = true
	}

	// Imprimir logs informativos de depuración
	log.Println("[Voice Engine] --- Estado de Dependencias C++ ---")
	log.Printf("[Voice Engine] sox/rec (Grabación inteligente): %t\n", hasRec)
	log.Printf("[Voice Engine] arecord (Grabación de fallback): %t\n", hasArecord)
	log.Printf("[Voice Engine] piper (Síntesis neural): %t\n", hasPiper)
	log.Printf("[Voice Engine] whisper-cli/whisper-cpp (Transcripción): %t\n", hasWhisperCli)
	log.Printf("[Voice Engine] Modelo Piper ONNX: %s (Existe: %t)\n", piperModelPath, fileExists(piperModelPath))
	log.Printf("[Voice Engine] Modelo Whisper GGML: %s (Existe: %t)\n", whisperModelPath, fileExists(whisperModelPath))
	log.Println("[Voice Engine] ----------------------------------")

	if !hasPiper {
		log.Println("[Voice Engine] ALERTA: No se encontró 'piper' en el PATH. RBot correrá en modo texto silencioso para Speak.")
	}
	if !hasWhisperCli {
		log.Println("[Voice Engine] ALERTA: No se encontró 'whisper-cli' o 'whisper-cpp' en el PATH. RBot correrá en modo texto silencioso para Listen.")
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Speak sintetiza y reproduce el texto en audio usando Piper
func Speak(text string) error {
	if text == "" {
		return nil
	}

	log.Printf("RBot (Voz): %s\n", text)

	if !hasPiper || !fileExists(piperModelPath) {
		// Modo texto fallback si piper no está configurado
		return nil
	}

	// Intentar streaming directo para baja latencia
	var playerCmd string
	if _, err := exec.LookPath("aplay"); err == nil {
		playerCmd = "aplay -r 22050 -f S16_LE -t raw -q"
	} else if _, err := exec.LookPath("pw-play"); err == nil {
		playerCmd = "pw-play --rate 22050 --format s16le -"
	} else if _, err := exec.LookPath("paplay"); err == nil {
		playerCmd = "paplay --raw --rate=22050 --format=s16le"
	}

	if playerCmd != "" {
		cmdStr := fmt.Sprintf("%s --model %s --output-raw | %s", piperBin, piperModelPath, playerCmd)
		cmd := exec.Command("bash", "-c", cmdStr)
		cmd.Stdin = strings.NewReader(text)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			return nil
		}
		log.Printf("[Voice Engine] Error en streaming TTS: %v (stderr: %s). Usando fallback...", err, stderr.String())
	}

	// Fallback: Escribir archivo WAV y reproducir
	wavFile := "voz.wav"
	_ = os.Remove(wavFile)
	
	cmd := exec.Command(piperBin, "--model", piperModelPath, "--output_file", wavFile)
	cmd.Stdin = strings.NewReader(text)
	
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error al ejecutar piper (fallback): %v (stderr: %s)", err, stderr.String())
	}

	// Reproducir el archivo generado
	playWav(wavFile)
	
	return nil
}

func playWav(wavFile string) {
	// Intentar reproducir usando comandos estándar de Linux
	for _, cmdName := range []string{"aplay", "paplay", "pw-play", "mpv", "play"} {
		if _, err := exec.LookPath(cmdName); err == nil {
			var cmd *exec.Cmd
			if cmdName == "aplay" {
				cmd = exec.Command("aplay", "-q", wavFile)
			} else {
				cmd = exec.Command(cmdName, wavFile)
			}
			_ = cmd.Run()
			return
		}
	}
}

// Listen graba audio y lo transcribe a texto usando whisper.cpp
func Listen() (string, error) {
	if !hasWhisperCli || !fileExists(whisperModelPath) {
		// Fallback interactivo por consola si no está disponible Whisper
		fmt.Print("\n[RBot MOCK - Escribe tu comando]: ")
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}

	wavFile := "audio.wav"
	// Eliminar WAV anterior si existe
	_ = os.Remove(wavFile)

	// 1. Grabar audio del micrófono
	if hasArecord {
		// Grabación inteligente en tiempo real usando Go Goroutine VAD y arecord
		if err := recordWithGoVAD(wavFile); err != nil {
			return "", err
		}
	} else if hasRec {
		// Fallback a SoX si arecord no está disponible
		log.Println("[Voice Engine] Grabando (SoX)... habla ahora...")
		cmd := exec.Command("rec", "-q", "-c", "1", "-r", "16000", wavFile, "silence", "1", "0.1", "1%", "1", "1.0", "1%")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("error al grabar con SoX: %v", err)
		}
	} else {
		return "", fmt.Errorf("no se encontró ningún grabador de audio ('arecord' o 'rec') en el sistema")
	}

	// Verificar que el archivo se haya creado y contenga datos
	info, err := os.Stat(wavFile)
	if err != nil || info.Size() < 100 {
		return "", nil // No se grabó nada
	}

	// 2. Transcribir con whisper-cli o whisper-cpp
	whisperBin := "whisper-cli"
	if _, err := exec.LookPath("whisper-cli"); err != nil {
		if _, err := exec.LookPath("whisper-cpp"); err == nil {
			whisperBin = "whisper-cpp"
		}
	}

	// Flags optimizados para velocidad máxima:
	// -nt: sin timestamps, -np: sin prints extra, -nf: sin fallback de temperatura
	// -bs 1 -bo 1: decodificación greedy (más rápida que beam search)
	// -fa: flash attention (aceleración GPU si disponible)
	args := []string{
		"-m", whisperModelPath,
		"-f", wavFile,
		"-nt",           // Sin timestamps
		"-l", "es",      // Idioma español
		"-nf",           // Sin fallback de temperatura
		"-bs", "1",      // Beam size 1 (greedy, más rápido)
		"-bo", "1",      // Best-of 1 (sin candidatos extra)
		"-np",           // Sin prints de progreso
		"-fa",           // Flash attention (GPU acelerado)
	}
	threads := WhisperThreads
	if threads <= 0 {
		threads = 8 // Default: 8 hilos para CPUs modernos
	}
	args = append(args, "-t", fmt.Sprintf("%d", threads))
	if WhisperFlags != "" {
		for _, flag := range strings.Fields(WhisperFlags) {
			args = append(args, flag)
		}
	}

	log.Println("[Voice Engine] Transcribiendo con Whisper C++...")
	cmd := exec.Command(whisperBin, args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error al transcribir con whisper: %v (stderr: %s)", err, stderr.String())
	}

	transcription := strings.TrimSpace(stdout.String())
	return transcription, nil
}

// recordWithGoVAD graba audio de arecord y utiliza una Goroutine para analizar los datos en tiempo real
// deteniendo la grabación inmediatamente tras 1.0 segundos de silencio.
func recordWithGoVAD(wavFile string) error {
	// Iniciar arecord para capturar audio crudo en PCM de 16-bit, 16000Hz, Mono
	cmd := exec.Command("arecord", "-q", "-t", "raw", "-f", "S16_LE", "-r", "16000", "-c", "1")
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error al obtener stdout de arecord: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error al iniciar arecord: %v (¿está instalado alsa-utils?)", err)
	}

	log.Println("[Voice Engine] Grabando... habla ahora...")

	var pcmBuffer []byte
	chunk := make([]byte, 3200) // 3200 bytes = 1600 muestras = 0.1 segundos

	hasSpoken := false
	silentChunks := 0
	maxChunks := 150   // 15 segundos máximo
	minChunks := 15    // Al menos 1.5 segundos
	chunksRecorded := 0
	threshold := vadThreshold // Sensibilidad de la detección de voz (VAD)

	for chunksRecorded < maxChunks {
		// Leer exactamente 0.1s de audio
		_, err := io.ReadFull(stdout, chunk)
		if err != nil {
			break
		}

		pcmBuffer = append(pcmBuffer, chunk...)
		chunksRecorded++

		// Calcular el valor medio absoluto de la amplitud del fragmento
		sum := 0.0
		for i := 0; i < len(chunk); i += 2 {
			val := int16(chunk[i]) | (int16(chunk[i+1]) << 8)
			absVal := val
			if val < 0 {
				absVal = -val
			}
			sum += float64(absVal)
		}
		avg := sum / (float64(len(chunk)) / 2.0)

		if OnAudioLevel != nil {
			normalized := avg / 3000.0
			if normalized > 1.0 {
				normalized = 1.0
			}
			OnAudioLevel(normalized)
		}

		// Máquina de estados para detección de voz
		if avg > threshold {
			if !hasSpoken {
				log.Println("[Voice Engine] Habla detectada...")
			}
			hasSpoken = true
			silentChunks = 0
		} else {
			if hasSpoken {
				silentChunks++
			}
		}

		// Si detectó habla y luego hubo 1.0s de silencio continuo, cortamos la grabación
		if hasSpoken && silentChunks >= 10 && chunksRecorded >= minChunks {
			log.Println("[Voice Engine] Silencio detectado, finalizando grabación.")
			break
		}

		// Si tras 4.0s no se detecta habla, cortamos para no grabar indefinidamente
		if !hasSpoken && chunksRecorded >= 40 {
			log.Println("[Voice Engine] No se detectó habla, cancelando.")
			break
		}
	}

	// Detener proceso arecord
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if len(pcmBuffer) == 0 {
		return fmt.Errorf("no se grabaron datos de audio")
	}

	// Escribir los datos PCM formateados en un contenedor WAV estándar
	return writeWavFile(wavFile, pcmBuffer, 16000)
}

// writeWavFile escribe datos PCM crudos envueltos en una cabecera WAV estándar de 44 bytes
func writeWavFile(filename string, pcmData []byte, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	numChannels := 1
	bitsPerSample := 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataLen := len(pcmData)

	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	fileSize := uint32(36 + dataLen)
	header[4] = byte(fileSize)
	header[5] = byte(fileSize >> 8)
	header[6] = byte(fileSize >> 16)
	header[7] = byte(fileSize >> 24)

	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	
	header[16] = 16
	header[17] = 0
	header[18] = 0
	header[19] = 0

	header[20] = 1
	header[21] = 0

	header[22] = byte(numChannels)
	header[23] = byte(numChannels >> 8)

	header[24] = byte(sampleRate)
	header[25] = byte(sampleRate >> 8)
	header[26] = byte(sampleRate >> 16)
	header[27] = byte(sampleRate >> 24)

	header[28] = byte(byteRate)
	header[29] = byte(byteRate >> 8)
	header[30] = byte(byteRate >> 16)
	header[31] = byte(byteRate >> 24)

	header[32] = byte(blockAlign)
	header[33] = byte(blockAlign >> 8)

	header[34] = byte(bitsPerSample)
	header[35] = byte(bitsPerSample >> 8)

	copy(header[36:40], []byte("data"))

	header[40] = byte(dataLen)
	header[41] = byte(dataLen >> 8)
	header[42] = byte(dataLen >> 16)
	header[43] = byte(dataLen >> 24)

	_, err = file.Write(header)
	if err != nil {
		return err
	}

	_, err = file.Write(pcmData)
	return err
}

// StopVoiceEngine no necesita hacer nada en Go puro
func StopVoiceEngine() {}

// PauseMedia pausa los reproductores de música/vídeo del sistema (MPRIS) usando playerctl
func PauseMedia() {
	_ = exec.Command("playerctl", "pause").Run()
}

// ResumeMedia reanuda los reproductores de música/vídeo del sistema (MPRIS) usando playerctl
func ResumeMedia() {
	_ = exec.Command("playerctl", "play").Run()
}

