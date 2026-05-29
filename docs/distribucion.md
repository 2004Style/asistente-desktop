# Distribución, Empaquetado e Instalación

Este documento describe la estructura de directorios del asistente en el sistema operativo Linux, las unidades de Systemd para su ejecución persistente y cómo realizar despliegues locales.

---

## 1. Estructura de Directorios Canónica

Cuando RBot se instala en producción o entorno de usuario en Linux, se despliega en las siguientes rutas según el estándar XDG:

* **Binarios del Sistema (`~/.local/bin/` o `/usr/local/bin/`)**:
  * `rbot`: Comando CLI interactivo para inicializar o configurar.
  * `rbotd`: Daemon principal del servicio en segundo plano.
  * `rbotctl`: Controlador CLI para enviar comandos al daemon.
  * `rbot-settings-gio`: Ventana gráfica Gio para Ajustes.
  * `rbot-hud`: Interfaz flotante de las esferas del asistente (GTK).
* **Configuración del Usuario (`~/.config/rbot/`)**:
  * `rbot.yaml`: Configuración general del orquestador, rutas, políticas y audio.
  * `providers.yaml`: Definición de capacidades declarativas de proveedores, claves API cifradas, perfiles y selección activa.
* **Datos del Sistema (`~/.local/share/rbot/`)**:
  * `rbot.db`: Base de datos SQLite.
  * `rbot.sock` y `events.sock`: Sockets unix para IPC y comunicación RPC.
  * `logs/`: Directorio de archivos de registro del daemon y la interfaz gráfica.

---

## 2. Servicios de Systemd (Ejecución en segundo plano)

El asistente proporciona archivos de definición de servicios de Systemd de usuario en la carpeta `systemd/` para que el daemon (`rbotd`) y el capturador de voz se ejecuten de fondo en cada inicio de sesión:

### Archivo: `systemd/rbot.service` (Servicio de Usuario)
```ini
[Unit]
Description=RBot Daemon - Automatización Local-First
After=network.target sound.target pulseaudio.service pipewire.service

[Service]
ExecStart=%h/.local/bin/rbotd
Restart=always
RestartSec=3
Environment=PATH=%h/.local/bin:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=default.target
```

### Comandos de gestión del Servicio de Systemd:
```bash
# Copiar servicio a directorio de systemd de usuario
mkdir -p ~/.config/systemd/user/
cp systemd/rbot.service ~/.config/systemd/user/

# Recargar daemon de Systemd de usuario
systemctl --user daemon-reload

# Habilitar e iniciar el servicio
systemctl --user enable rbot.service
systemctl --user start rbot.service

# Comprobar el estado del servicio
systemctl --user status rbot.service
```

---

## 3. Script de Instalación (`install.sh`)

El archivo de automatización `install.sh` realiza los siguientes pasos de distribución de forma transparente:
1. Valida que las herramientas del compilador de Go y GCC estén disponibles.
2. Compila el daemon, CLI y Ajustes en un directorio temporal de compilación.
3. Si el HUD es requerido y las dependencias de GTK están presentes, compila `rbot-hud` con las etiquetas de construcción correctas (`-tags "gtk_3_18 hud"`).
4. Copia los ejecutables compilados a `~/.local/bin/` y crea el directorio `~/.config/rbot/` copiando la configuración por defecto de `config/`.
