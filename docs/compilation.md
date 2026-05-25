# Guía de Compilación e Instalación de RBot 🛠️

Esta guía detalla paso a paso cómo preparar tu entorno, instalar dependencias, compilar binarios y descargar modelos locales para inicializar **RBot**.

---

## ⚡ Método Rápido y Automático (Recomendado)

Si ya tienes las herramientas básicas del sistema (o si deseas instalar todo de golpe), puedes ejecutar el script de aprovisionamiento automatizado en la raíz del proyecto. Este script realiza lo siguiente de forma autónoma:
1. Valida qué herramientas esenciales (`go`, `sox`, `arecord`, `playerctl`, etc.) están presentes en tu sistema.
2. Crea los directorios locales `voices/` y `models/` en la carpeta del proyecto.
3. Descarga la voz neuronal de Piper (`es_ES-davefx-medium.onnx` y su `.json`).
4. Descarga el modelo optimizado de transcripción rápida Whisper (`ggml-tiny.bin`).
5. Configura e inicializa el directorio local de habilidades de usuario en `~/.local/share/rbot/skills` copiando las habilidades por defecto del repositorio.
6. Compila el binario `rbot` optimizado en la carpeta `bin/rbot`.

```bash
# Dar permisos de ejecución
chmod +x setup_and_build.sh

# Ejecutar el aprovisionador y compilador automatizado
./setup_and_build.sh
```

Una vez finalizado, puedes iniciar el motor en modo de voz usando:
```bash
./bin/rbot voice
```

---

## 🔍 Método Manual y Avanzado Paso a Paso

Si deseas personalizar la compilación de `whisper.cpp` (por ejemplo, para habilitar aceleración por tarjeta gráfica NVIDIA GPU) o si estás configurando RBot en otra computadora, sigue este procedimiento paso a paso:

### Paso 1: Instalar Dependencias Básicas
Dependiendo de tu sistema operativo:

* **Arch Linux / AUR:**
  ```bash
  sudo pacman -S --needed base-devel cmake git go sox alsa-utils nodejs npm playerctl wget curl
  yay -S piper-tts-bin
  ```

* **Ubuntu / Debian / Linux Mint:**
  ```bash
  sudo apt update
  sudo apt install -y golang sox alsa-utils cmake git build-essential nodejs npm playerctl wget curl libasound2-dev
  # Sigue las instrucciones de 'dependencies.md' para descargar e instalar el binario 'piper'
  ```

---

### Paso 2: Clonar y Compilar Whisper.cpp con Aceleración GPU/CPU

RBot ejecuta el comando `whisper-cli` (o `whisper-cpp`) para transcribir el audio del micrófono. Compilar Whisper.cpp correctamente es el paso más importante para lograr una latencia baja.

#### Opción A: Compilación para Tarjeta Gráfica NVIDIA (GPU con CUDA) - RECOMENDADO
*Si tienes una tarjeta NVIDIA (por ejemplo, RTX 3050, RTX 4060, etc.) y tienes el driver CUDA instalado:*

1. Clonar el repositorio oficial de Whisper.cpp:
   ```bash
   git clone https://github.com/ggerganov/whisper.cpp.git
   cd whisper.cpp
   ```
2. Generar el entorno de CMake configurando la bandera de CUDA (`GGML_CUDA=ON`):
   ```bash
   cmake -B build -DGGML_CUDA=ON -DCMAKE_BUILD_TYPE=Release
   ```
3. Compilar el ejecutable `whisper-cli`:
   ```bash
   cmake --build build --config Release --target whisper-cli
   ```
4. Mover el binario generado a tu PATH para que RBot pueda invocarlo en cualquier lugar:
   ```bash
   sudo cp build/bin/whisper-cli /usr/local/bin/whisper-cli
   ```

#### Opción B: Compilación Estándar para Procesador (CPU)
*Si no dispones de tarjeta NVIDIA:*

1. Clonar e ingresar al repositorio:
   ```bash
   git clone https://github.com/ggerganov/whisper.cpp.git
   cd whisper.cpp
   ```
2. Generar entorno con CMake sin banderas de GPU (usará aceleración nativa de CPU AVX/OpenMP):
   ```bash
   cmake -B build -DCMAKE_BUILD_TYPE=Release
   ```
3. Compilar:
   ```bash
   cmake --build build --config Release --target whisper-cli
   ```
4. Instalar en el PATH:
   ```bash
   sudo cp build/bin/whisper-cli /usr/local/bin/whisper-cli
   ```

---

### Paso 3: Descargar los Modelos e Instalar Habilidades
Desde la carpeta raíz del proyecto de RBot:

1. **Crear las carpetas para modelos:**
   ```bash
   mkdir -p voices models
   ```
2. **Descargar modelo rápido de Whisper (Tiny):**
   ```bash
   wget -O models/ggml-tiny.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin
   ```
3. **Descargar modelo de voz en español de Piper:**
   ```bash
   wget -O voices/es_ES-davefx-medium.onnx https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx
   wget -O voices/es_ES-davefx-medium.onnx.json https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json
   ```
4. **Instalar Habilidades por Defecto:**
   Crea la carpeta en tu directorio local de usuario y copia las habilidades incluidas en el repositorio para que estén accesibles al asistente:
   ```bash
   mkdir -p ~/.local/share/rbot/skills
   cp -r skills/* ~/.local/share/rbot/skills/
   ```

---

### Paso 4: Compilar RBot
Compila el ejecutable en Go. Desde el directorio raíz del proyecto:
```bash
# Crear directorio de binarios y compilar
mkdir -p bin
go build -o bin/rbot cmd/main.go
```

---

### Paso 5: Lanzar y Validar el Asistente

1. **Asegúrate de que Ollama está activo con el modelo descargado:**
   ```bash
   ollama run qwen2.5:7b
   ```
2. **Ejecutar en modo de voz interactivo:**
   ```bash
   ./bin/rbot voice
   ```
   * RBot validará las dependencias y mostrará su estado.
   * Si compilaste Whisper con CUDA, verás un log que indica el uso de GPU.
   * RBot hablará indicando *"Entorno preparado, señor. Estoy atento a sus instrucciones."* y esperará a que digas la palabra clave (ej. *"oye ronald"* o *"ronald"*).

3. **Ejecutar en modo texto rápido (Chat CLI):**
   ```bash
   ./bin/rbot chat "ejecuta vscode"
   ```
   *(El modo chat no levanta servidores MCP externos ni bloquea en segundo plano para asegurar una respuesta inmediata).*

---

### Paso 6: Ejecutar las Pruebas Unitarias

Para garantizar la estabilidad antes de realizar aportes o empaquetar RBot para otra computadora, ejecuta la suite de pruebas unitarias:
```bash
# Ejecutar todas las pruebas del proyecto
go test -v ./...

# Forzar una ejecución limpia sin caché
go test -count=1 -v ./...
```

---

*(Para más detalles sobre la arquitectura de datos y la personalización de herramientas MCP, consulta la guía en [db_y_mcp_config.md](db_y_mcp_config.md)).*
