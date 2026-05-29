#!/usr/bin/env python3
import os
import sys
import json
import queue
import socket
import subprocess
import threading
import time

# Localización de este script
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(script_dir)
os.chdir(project_root)

# Configurar ruta aislada del modelo de Vosk en español
model_path = os.path.join(project_root, "config", "vosk_model")

# Si no existe, invocar al instalador automático
if not os.path.exists(model_path) or not os.listdir(model_path):
    print("⚠️ No se encontró el modelo de Vosk local. Lanzando instalador automático...")
    installer_path = os.path.join(project_root, "scripts", "install_vosk.py")
    try:
        subprocess.run([sys.executable, installer_path], check=True)
    except Exception as e:
        print(f"❌ Error al ejecutar el instalador de Vosk: {e}")
        sys.exit(1)

# Intentar importar dependencias
try:
    import sounddevice as sd
    from vosk import Model, KaldiRecognizer
except ImportError:
    print("❌ Error: Se requiere instalar 'vosk' y 'sounddevice' en tu entorno Python.")
    print("Podés instalarlas fácilmente ejecutando:")
    print(f"  {sys.executable} -m pip install vosk sounddevice")
    sys.exit(1)

print(f"📦 Cargando modelo Vosk desde la ruta del proyecto: {model_path}...")
model = Model(model_path)
rec = KaldiRecognizer(model, 16000)
q = queue.Queue()

# Variables de estado del bot compartidas
is_speaking = False
is_awake = False
cooldown_until = 0.0

def send_command_to_rbot_socket(text):
    socket_path = os.path.expanduser("~/.local/share/rbot/rbot.sock")
    if not os.path.exists(socket_path):
        socket_path = os.path.join(project_root, "config", "rbot.sock")
        if not os.path.exists(socket_path):
            socket_path = os.path.expanduser("~/.config/rbot/rbot.sock")

    if not os.path.exists(socket_path):
        return

    try:
        client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        client.connect(socket_path)
        payload = {
            "jsonrpc": "2.0",
            "method": "agent.say",
            "params": {"text": text},
            "id": 1
        }
        client.sendall((json.dumps(payload) + "\n").encode('utf-8'))
        client.recv(4096)
        client.close()
    except Exception as e:
        print(f"❌ Error al enviar transcripción al daemon: {e}")

def send_command_to_rbot(text):
    # Enviar de forma asíncrona para no bloquear el bucle principal de captura de micrófono
    threading.Thread(target=send_command_to_rbot_socket, args=(text,), daemon=True).start()

def send_wake_to_rbot_socket():
    socket_path = os.path.expanduser("~/.local/share/rbot/rbot.sock")
    if not os.path.exists(socket_path):
        socket_path = os.path.join(project_root, "config", "rbot.sock")
    if not os.path.exists(socket_path):
        return

    try:
        client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        client.connect(socket_path)
        payload = {
            "jsonrpc": "2.0",
            "method": "voice.wake",
            "params": {},
            "id": 1
        }
        client.sendall((json.dumps(payload) + "\n").encode('utf-8'))
        client.recv(4096)
        client.close()
    except Exception as e:
        print(f"❌ Error al despertar al daemon: {e}")

def send_wake_to_rbot():
    threading.Thread(target=send_wake_to_rbot_socket, daemon=True).start()

def send_sleep_to_rbot_socket():
    socket_path = os.path.expanduser("~/.local/share/rbot/rbot.sock")
    if not os.path.exists(socket_path):
        socket_path = os.path.join(project_root, "config", "rbot.sock")
    if not os.path.exists(socket_path):
        return

    try:
        client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        client.connect(socket_path)
        payload = {
            "jsonrpc": "2.0",
            "method": "voice.sleep",
            "params": {},
            "id": 1
        }
        client.sendall((json.dumps(payload) + "\n").encode('utf-8'))
        client.recv(4096)
        client.close()
    except Exception as e:
        print(f"❌ Error al dormir al daemon: {e}")

def send_sleep_to_rbot():
    threading.Thread(target=send_sleep_to_rbot_socket, daemon=True).start()

# Hilo para escuchar el bus de eventos y sincronizar el estado
def events_listener():
    global is_speaking, is_awake, cooldown_until
    socket_path = os.path.expanduser("~/.local/share/rbot/events.sock")
    if not os.path.exists(socket_path):
        socket_path = os.path.join(project_root, "config", "events.sock")
        if not os.path.exists(socket_path):
            socket_path = os.path.expanduser("~/.config/rbot/events.sock")

    print(f"🔗 Conectando al bus de eventos: {socket_path}...")

    while True:
        try:
            if not os.path.exists(socket_path):
                time.sleep(2)
                continue
            
            client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            client.connect(socket_path)
            
            buffer = ""
            while True:
                data = client.recv(4096)
                if not data:
                    break
                buffer += data.decode('utf-8', errors='ignore')
                while "\n" in buffer:
                    line, buffer = buffer.split("\n", 1)
                    line = line.strip()
                    if line:
                        try:
                            event = json.loads(line)
                            event_type = event.get("type")
                            if event_type == "tts.speaking":
                                is_speaking = True
                            elif event_type == "tts.finished":
                                is_speaking = False
                                # Cooldown de 1.8 segundos tras finalizar el TTS para disipar ecos físicos
                                cooldown_until = time.time() + 1.8
                                # Vaciar inmediatamente la cola de audio para eliminar cualquier eco del altavoz
                                try:
                                    while not q.empty():
                                        q.get_nowait()
                                except Exception:
                                    pass
                                print("🔇 Cola de captura purgada tras finalización de habla (cooldown activo).")
                            elif event_type in ["voice.wake_detected", "voice.listening", "voice.ready", "voice.transcribed"]:
                                is_awake = True
                            elif event_type in ["voice.sleeping", "voice.timeout"]:
                                is_awake = False
                        except Exception:
                            pass
            client.close()
        except Exception:
            time.sleep(2)

# Arrancar el hilo de escucha de eventos
t = threading.Thread(target=events_listener, daemon=True)
t.start()

def audio_callback(indata, frames, time, status):
    if status:
        print(status, file=sys.stderr)
    q.put(bytes(indata))

# Palabras clave de activación (Wake Words)
wake_words = ["oye ronald", "ey ronald", "go ronald", "hola ronald", "ronald", "rbot"]
# Palabras clave para dormir el bot
sleep_words = ["duérmete", "duermete", "cállate", "callate", "silencio", "dormir"]

try:
    with sd.RawInputStream(samplerate=16000, blocksize=8000, dtype="int16", channels=1, callback=audio_callback):
        print("\n🎙️ [RBot Vosk Bridge] Escuchando micrófono de forma aislada...")
        print("💡 Lógica de Wake Words activa. El bot no se activará con ruidos de fondo o música.")
        print("Ctrl+C para salir.\n")
        
        while True:
            data = q.get()
            
            # Descartar audio si RBot está hablando o en periodo de cooldown por eco
            current_time = time.time()
            if is_speaking or current_time < cooldown_until:
                rec.Result() # Limpiar buffer interno de Vosk
                # Purgar la cola de entrada para no acumular latencia
                try:
                    while not q.empty():
                        q.get_nowait()
                except Exception:
                    pass
                continue
                
            if rec.AcceptWaveform(data):
                if is_speaking or time.time() < cooldown_until:
                    rec.Result()
                    continue
                    
                res = json.loads(rec.Result())
                text = res.get("text", "").strip().lower()
                if not text:
                    continue
                
                # Comprobación de seguridad doble
                if is_speaking or time.time() < cooldown_until:
                    continue
                    
                print(f"🎙️ Detectado: \"{text}\" (Despierto: {is_awake})")
                
                if not is_awake:
                    # Permitir abrir ajustes o panel de control por voz incluso si está durmiendo
                    is_window_cmd = any(keyword in text for keyword in ["configurac", "ajuste", "panel", "esfera"])
                    if is_window_cmd:
                        print(f"✨ Comando de control de ventana detectado en reposo: \"{text}\"")
                        send_wake_to_rbot()
                        is_awake = True
                        cooldown_until = time.time() + 5.0
                        send_command_to_rbot(text)
                        continue

                    # Buscar si contiene alguna wake word
                    found_wake = False
                    for w in wake_words:
                        if w in text:
                            found_wake = True
                            # Extraer el comando que sigue a la palabra clave
                            parts = text.split(w, 1)
                            cmd = parts[1].strip()
                            
                            if cmd:
                                print(f"✨ Wake Word '{w}' detectada con comando: \"{cmd}\"")
                                cooldown_until = time.time() + 8.0
                                send_wake_to_rbot()
                                time.sleep(0.5) # Esperar a que RBot termine su saludo corto
                                send_command_to_rbot(cmd)
                            else:
                                print(f"✨ Wake Word '{w}' detectada. Despertando...")
                                cooldown_until = time.time() + 5.0
                                send_wake_to_rbot()
                            
                            is_awake = True
                            break
                    if not found_wake:
                        # Si no está despierto y no se dijo la wake word, descartar
                        continue
                else:
                    # Si ya está despierto, verificar si es una orden de dormir
                    is_sleep_cmd = False
                    for s in sleep_words:
                        if s in text:
                            is_sleep_cmd = True
                            print(f"💤 Orden de dormir detectada: '{s}'. Durmiendo...")
                            send_sleep_to_rbot()
                            is_awake = False
                            break
                            
                    if not is_sleep_cmd:
                        print(f"🗣️ Enviando orden: \"{text}\"")
                        cooldown_until = time.time() + 8.0
                        send_command_to_rbot(text)

except KeyboardInterrupt:
    print("\n👋 Puente de voz de Vosk finalizado.")
    sys.exit(0)
except Exception as e:
    print(f"\n❌ Ocurrió un error en el streaming de audio: {e}")
    sys.exit(1)
