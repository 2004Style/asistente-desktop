# Dependencias del Sistema y Entorno de Desarrollo

Este documento detalla todas las dependencias del sistema, librerías de desarrollo, ejecutables de C++ y modelos necesarios para compilar y ejecutar RBot de manera completa en Linux.

## 1. Librerías de Compilación y Gráficos (Go/Gio/GTK)

Para construir la interfaz gráfica de Ajustes (Gio) y el HUD (GTK 3), se requieren las siguientes librerías de desarrollo en la máquina anfitriona:

### Debian/Ubuntu y derivados:
```bash
# Dependencias gráficas para Gio (OpenGL, X11)
sudo apt install xorg-dev libgl1-mesa-dev libx11-dev

# Dependencia para el gestor de credenciales seguro (Keyring)
sudo apt install libsecret-1-dev

# Dependencias para el HUD nativo de GTK (Opt-in)
sudo apt install libgtk-3-dev libgdk-pixbuf2.0-dev glib-2.0
```

---

## 2. Dependencias de Audio y Voz (Motor de Voz)

El daemon del motor de voz (`scripts/rbot-voice-vosk.py`) se comunica con varios procesos nativos para capturar el audio, transcribir y sintetizar voz:

* **Grabación Inteligente (`sox` / `rec`)**: Utilizado para la captura inteligente de voz basada en umbrales de silencio y volumen.
* **Grabación de Fallback (`arecord`)**: Fallback estándar de grabación en Linux (ALSA).
* **Síntesis Neural (`piper`)**: Motor neural offline de síntesis de voz en español. Requiere su binario en el PATH o en la carpeta del proyecto.
* **Transcripción Offline (`whisper-cli` / `whisper.cpp`)**: Ejecutable optimizado de Whisper en C/C++ para transcripción de audio local ultrarrápida.

### Instalación de dependencias básicas de audio en Debian/Ubuntu:
```bash
sudo apt install sox libsox-fmt-all alsa-utils
```

---

## 3. Modelos de Machine Learning (Voz y Transcripción)

Los siguientes modelos deben ubicarse en la estructura del proyecto para el funcionamiento offline:

* **Modelo Piper ONNX**:
  * Archivo requerido: `voices/es_ES-davefx-medium.onnx` (y su archivo de configuración `.onnx.json` correspondiente).
  * Función: Voz de síntesis en español para el asistente.
* **Modelo Whisper GGML**:
  * Archivo requerido: `models/ggml-tiny.bin` (u otro tamaño soportado).
  * Función: Transcripción local por voz.

---

## 4. Dependencias del Llavero Seguro (Secret Service)

Para almacenar y recuperar tokens e inyecciones de claves API de forma segura:
* Se comunica con la especificación Freedesktop Secret Service a través de D-Bus.
* En entornos de escritorio (Gnome, KDE, XFCE), requiere de `gnome-keyring` o `ksecretservice` en ejecución.
* **Fallback a Texto Plano (`plain:`)**: Si no hay sesión gráfica activa o el demonio de D-Bus no está activado, RBot realiza de manera automática un fallback seguro a almacenamiento en texto plano en la configuración del usuario (`config/providers.yaml`).
