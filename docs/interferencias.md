# Guía de Resolución de Interferencias de Audio 🎧🔇

Esta guía explica cómo configurar el sistema para evitar que la música de fondo, películas (Netflix, YouTube, VLC, etc.) o videos impidan que RBot escuche tus comandos de voz o reconozca la palabra de activación (wake word).

El sistema utiliza una solución de **dos capas** para resolver este problema:

---

## ⚡ Capa 1: Pausar/Reanudar Audio Automáticamente (Aplicación RBot)

RBot se integra directamente con el protocolo multimedia estándar de Linux (MPRIS) a través de `playerctl`. 

### Cómo funciona:
1. Al pronunciar la palabra de activación ("oye ronald", "rbot", etc.), RBot envía una orden global de **Pausa** a todo reproductor compatible del sistema.
2. Durante tu interacción (mientras el micrófono te graba y mientras el asistente habla de vuelta), el audio de fondo permanece pausado para garantizar una claridad total.
3. Una vez finalizada la respuesta de RBot, la reproducción se **reanuda** automáticamente.

### Requisito:
Solo necesitas tener instalado el paquete del sistema `playerctl`:

```bash
# Instalar en Arch Linux
sudo pacman -S playerctl
```

---

## 🎙️ Capa 2: Cancelación de Eco Acústico (A nivel de Sistema - PipeWire)

Cuando estás reproduciendo sonido a través de tus altavoces, este es captado por el micrófono físico, ahogando tu voz. 

La solución definitiva en Linux es configurar **Acoustic Echo Cancellation (AEC)**. Este módulo matemático resta el sonido saliente (altavoces) de la captura de entrada (micrófono) en tiempo real, permitiendo que el micrófono escuche *únicamente* tu voz, incluso con música sonando a gran volumen.

### Configuración paso a paso en Arch Linux:

1. **Crea el directorio de configuración de PipeWire para tu usuario (si no existe):**
   ```bash
   mkdir -p ~/.config/pipewire/pipewire.conf.d/
   ```

2. **Crea y edita el archivo de configuración `60-echo-cancel.conf`:**
   ```bash
   nano ~/.config/pipewire/pipewire.conf.d/60-echo-cancel.conf
   ```

3. **Copia y pega la siguiente configuración dentro del archivo:**
   ```text
   context.modules = [
       { name = libpipewire-module-echo-cancel
           args = {
               monitor.mode = true
               source.props = {
                   node.name = "source_ec"
                   node.description = "Micrófono con Cancelación de Eco"
               }
               aec.args = {
                   webrtc.gain_control = true
                   webrtc.extended_filter = false
               }
           }
       }
   ]
   ```
   *(Guarda y cierra el editor presionando `Ctrl+O`, `Enter` y `Ctrl+X`).*

4. **Reinicia los servicios de sonido para aplicar el cambio:**
   ```bash
   systemctl --user restart pipewire pipewire-pulse
   ```

5. **Activa el nuevo micrófono virtual:**
   * Abre tu panel de control de volumen de tu entorno gráfico (o ejecuta `pavucontrol` en la terminal).
   * Ve a la sección de **Dispositivos de Entrada** o **Grabación**.
   * Selecciona el nuevo dispositivo virtual llamado **"Micrófono con Cancelación de Eco"** como tu entrada predeterminada.

---

## 🌪️ Capa 3: Supresión de Ruido de Ventiladores (RNNoise)

Si los ventiladores de tu máquina hacen mucho ruido y Whisper los interpreta como música o habla falsa, puedes activar un filtro de red neuronal llamado **RNNoise** en PipeWire para que elimine el zumbido de fondo por completo manteniendo solo tu voz.

### Configuración en Arch Linux:
1. **Instala el plugin de supresión de ruido para PipeWire:**
   ```bash
   sudo pacman -S noise-suppression-for-voice
   ```
2. **Carga el plugin en tu configuración de PipeWire:**
   Crea el archivo `65-noise-suppression.conf`:
   ```bash
   nano ~/.config/pipewire/pipewire.conf.d/65-noise-suppression.conf
   ```
3. **Pega la siguiente configuración:**
   ```text
   context.modules = [
       { name = libpipewire-module-filter-chain
           args = {
               node.description = "Micrófono con Supresión de Ruido (RNNoise)"
               media.name       = "Noise Suppressed Source"
               filter.graph = {
                   nodes = [
                       {
                           type   = ladspa
                           name   = rnnoise
                           plugin = /usr/lib/ladspa/librnnoise_ladspa.so
                           label  = noise_suppressor_mono
                           control = {
                               "VAD Threshold (%)" = 50.0
                           }
                       }
                   ]
               }
               capture.props = {
                   node.passive = true
               }
               playback.props = {
                   media.class = Audio/Source
               }
           }
       }
   ]
   ```
4. **Reinicia PipeWire:**
   ```bash
   systemctl --user restart pipewire pipewire-pulse
   ```
5. **Selecciona "Micrófono con Supresión de Ruido (RNNoise)"** como tu micrófono predeterminado en la configuración de audio del sistema o `pavucontrol`.

---

## 🧠 Capa 4: Modelo de Whisper y Aceleración por GPU (CUDA)

Para lograr un rendimiento óptimo de latencia, RBot viene preconfigurado con el modelo **`ggml-tiny.bin`** (~75MB) y **8 hilos** de ejecución. Si se compila `whisper.cpp` con soporte **CUDA (GPU)**, la transcripción tarda apenas **2 segundos** y es sumamente eficiente.

Sin embargo, en ambientes muy ruidosos o si el micrófono capta mucho ruido de fondo, el modelo `tiny` puede alucinar o generar texto basura. Para solucionar esto, puedes cambiar al modelo **`ggml-small.bin`** (~460MB), que es significativamente más preciso y robusto ante ruidos extraños, aunque requiere mayor capacidad de procesamiento.

### Instrucciones para cambiar al modelo `small`:
1. **Descarga el modelo `small`:**
   Desde el directorio raíz de RBot:
   ```bash
   wget -O models/ggml-small.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
   ```
2. **Actualiza la configuración de RBot (`config/rbot.yaml`):**
   Abre el archivo y actualiza los parámetros del transcriptor:
   ```yaml
   voice:
       whisper_model: models/ggml-small.bin
   ```
3. **Vuelve a iniciar el asistente:**
   ```bash
   ./bin/rbot voice
   ```
   *(Nota: Se recomienda encarecidamente utilizar aceleración por GPU/CUDA al usar el modelo `small` para mantener los tiempos de respuesta por debajo de los 3 segundos).*

---

## 🛠️ Solución de Problemas

* **¿La música no se pausa?** Asegúrate de que el reproductor que usas soporte el protocolo MPRIS (Spotify, VLC, Audacious, MPV y navegadores modernos basados en Chromium/Firefox lo soportan de forma nativa).
* **¿Sigue sin reaccionar o se confunde?** Comprueba en tu panel de volumen que el micrófono predeterminado del sistema sea el virtual de cancelación de eco o el de supresión de ruido (RNNoise) y no el micrófono físico directo, ya que el físico carece del filtrado por software.
