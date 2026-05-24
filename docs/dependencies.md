# Requisitos y Dependencias de RBot 📋

RBot es un asistente de voz escrito en Go diseñado para ejecutarse localmente con una latencia extremadamente baja. A diferencia de otros asistentes, no depende de Python para sus tareas principales. En su lugar, interactúa de forma nativa con binarios optimizados en C/C++ (`piper` para síntesis de voz, `whisper.cpp` para reconocimiento de voz) y utiliza herramientas del sistema Linux.

Esta guía detalla los requisitos del sistema, cómo instalar los paquetes necesarios en diferentes distribuciones de Linux y cómo descargar los modelos locales de Inteligencia Artificial.

---

## 💻 1. Requisitos de Hardware Recomendados
* **Procesador (CPU):** Mínimo 4 núcleos (con soporte para instrucciones AVX2 para optimizar Whisper y Piper).
* **Tarjeta Gráfica (GPU):** Opcional pero altamente recomendada. Una GPU NVIDIA (como la RTX 3050 o superior) con CUDA instalado reduce el tiempo de transcripción de voz de **7 segundos a menos de 2 segundos** utilizando Whisper C++.
* **Memoria RAM:** Mínimo 8 GB (16 GB recomendados para poder ejecutar Ollama con modelos de 7B de manera fluida en segundo plano).
* **Sistema de Sonido:** Servidor de sonido Linux operativo (PipeWire con PipeWire-Pulse o PulseAudio), un micrófono físico y altavoces configurados.

---

## 🛠️ 2. Dependencias del Sistema por Distribución

RBot requiere utilidades de grabación de audio, utilidades de control multimedia y Node.js para ejecutar servidores de Model Context Protocol (MCP).

### Opción A: Arch Linux
Instala los paquetes principales desde los repositorios oficiales usando `pacman`:
```bash
sudo pacman -S --needed go sox alsa-utils cmake git base-devel nodejs npm playerctl wget curl
```
Para el motor de síntesis de voz **Piper**, instala el paquete de AUR usando tu gestor preferido (por ejemplo, `yay`):
```bash
yay -S piper-tts-bin
```
*Nota: Este paquete instala el binario `piper-tts` en `/usr/bin/piper-tts`, que RBot detecta automáticamente.*

### Opción B: Debian / Ubuntu / Linux Mint
Instala los paquetes necesarios desde los repositorios oficiales usando `apt`:
```bash
sudo apt update
sudo apt install -y golang sox alsa-utils cmake git build-essential nodejs npm playerctl wget curl libasound2-dev
```

#### Instalar Piper TTS manualmente en Debian/Ubuntu:
Debido a que Piper no siempre está en los repositorios de APT, puedes descargar e instalar la versión binaria standalone de forma muy sencilla:
```bash
# Descargar el archivo comprimido del repositorio de lanzamientos oficiales
wget https://github.com/rhasspy/piper/releases/latest/download/piper_amd64.tar.gz

# Extraer el contenido
tar -xf piper_amd64.tar.gz

# Copiar el binario y las librerías compartidas a una ruta del sistema
sudo cp piper/piper /usr/local/bin/
sudo cp piper/libpiper_bindings.so* /usr/local/lib/
sudo cp piper/libonnxruntime.so* /usr/local/lib/

# Actualizar el enlace dinámico de librerías en el sistema
sudo ldconfig
```
Para validar que está correctamente instalado, escribe:
```bash
piper --help
```

---

## 🚀 3. Servidor de Inferencia Local (Ollama)

Ollama actúa como el "cerebro" conversacional de RBot.

1. **Instalar Ollama en Linux:**
   ```bash
   curl -fsSL https://ollama.com/install.sh | sh
   ```
2. **Iniciar el servicio y descargar el modelo conversacional:**
   RBot utiliza la API local de Ollama (por defecto en `http://localhost:11434`). Se requiere un modelo optimizado para instrucciones en español y con soporte nativo de **Tool Calling** (Llamada a Funciones):
   ```bash
   ollama pull qwen2.5:7b
   ```
   *(Alternativa ligera para equipos de menores recursos: `qwen2.5:3b` o `qwen2.5:1.5b`. RBot es compatible con cualquier LLM que soporte Tool Calling en Ollama).*

---

## 🧠 4. Modelos Locales de Audio

Para funcionar sin conexión de internet, RBot requiere modelos locales de Síntesis (ONNX) y Transcripción (GGML) dentro de la carpeta raíz del proyecto. El script `setup_and_build.sh` los descarga automáticamente, pero aquí tienes los detalles si deseas descargarlos manualmente:

### A. Modelo de Voz de Piper (Síntesis de voz - TTS)
Necesitas descargar un modelo `.onnx` para español y su archivo de configuración `.json` correspondiente. Colócalos en la carpeta `voices/` del proyecto:
* **Voz Recomendada:** DaveFX en calidad media (`es_ES-davefx-medium.onnx`)
* **Modelo ONNX:** [HuggingFace - es_ES-davefx-medium.onnx](https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx)
* **Archivo de Configuración:** [HuggingFace - es_ES-davefx-medium.onnx.json](https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json)

```bash
mkdir -p voices
wget -O voices/es_ES-davefx-medium.onnx "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx"
wget -O voices/es_ES-davefx-medium.onnx.json "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json"
```

### B. Modelos de Whisper.cpp (Transcripción - STT)
Whisper.cpp lee modelos en formato GGML. Colócalos en la carpeta `models/`:
* **Modelo Tiny (75 MB - Recomendado por velocidad):** Transcripción ultra rápida en CPU y GPU.
  * **Descarga:** [ggml-tiny.bin](https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin)
* **Modelo Base (142 MB - Excelente balance):** Ligeramente más preciso que tiny pero con mayor coste.
  * **Descarga:** [ggml-base.bin](https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin)
* **Modelo Small (460 MB - Máxima precisión):** Diseñado para ignorar ruidos de fondo persistentes y alucinaciones en entornos ruidosos. Requiere aceleración GPU para mantener una baja latencia.
  * **Descarga:** [ggml-small.bin](https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin)

```bash
mkdir -p models
# Descargar el modelo por defecto (tiny)
wget -O models/ggml-tiny.bin "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
```

---

## 🌐 5. Dependencias para Servidores MCP (Model Context Protocol)

RBot levanta servidores MCP externos configurados en `mcp/mcp_config.json`. La mayoría de los servidores oficiales de Anthropic y de la comunidad de MCP están desarrollados en Node.js y se inician mediante el comando ejecutor `npx`.
* Requiere **Node.js** (v18 o superior) y **npm** instalados y accesibles en el PATH del sistema para permitir la autoejecución dinámica de herramientas MCP (como el servidor `@modelcontextprotocol/server-filesystem` o `@modelcontextprotocol/server-fetch`).

