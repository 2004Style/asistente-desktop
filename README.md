# RBot - Desktop Agent written in Go 🚀🤖

**RBot** es un asistente de voz y agente de automatización local para escritorio, desarrollado desde cero en **Go** y diseñado para interactuar mediante voz o texto. Utiliza **Ollama** local como cerebro de razonamiento, **SQLite + FTS5** como memoria operativa de largo plazo y soporta **Model Context Protocol (MCP)** junto con habilidades dinámicas basadas en especificaciones **SKILL.md**.

---

## 🌟 Características Principales

*   **Lenguaje de Programación:** Go (ejecución extremadamente rápida y bajo consumo de recursos).
*   **Base de Datos local (SQLite + FTS5):** Base de datos indexada con búsqueda de texto completo para recordar archivos, carpetas, alias de rutas, historial de ejecuciones y datos personales.
*   **Herramientas y Tool Calling nativo:** Conecta dinámicamente herramientas locales e inyecta herramientas de servidores MCP en Ollama mediante la API `/api/chat`.
*   **Cliente MCP stdio JSON-RPC 2.0:** Inicializa y enruta peticiones de forma **asíncrona y en segundo plano** a servidores MCP externos (por ejemplo, `server-filesystem`) para evitar retrasos de arranque.
*   **Motor de Habilidades (`SKILL.md`):** Parser nativo de Markdown y frontmatter YAML para indexar procedimientos y auditorías de seguridad antes de ejecutar comandos.
*   **Seguridad por Capas:** Control de rutas prohibidas (ej. carpetas `.ssh` o archivos `.env`) y confirmación interactiva para herramientas de alto riesgo antes de su ejecución.
*   **Confirmación Rápida de Acciones Directas:** Salta llamadas innecesarias al LLM para confirmar de inmediato (latencia < 1ms) la ejecución de acciones mecánicas como abrir/cerrar apps (`"ejecuta vscode"`) o cargar URLs, generando las respuestas conversacionales localmente en español.
*   **Auto-Carga e Indexación Asíncrona:** Descubre y habilita automáticamente habilidades en el inicio, y realiza el primer escaneo de archivos y aplicaciones de escritorio en segundo plano si la base de datos local está vacía.
*   **Ciclo de Voz de Alto Rendimiento:** Utiliza binarios de C++ altamente optimizados (`piper` para TTS y `whisper.cpp` para STT) controlados directamente por Go, eliminando la sobrecarga y latencia de Python.

---

## 📂 Estructura del Proyecto

```txt
asistente/ (Directorio Raíz)
├── cmd/
│   └── main.go           # CLI y loop de escucha principal de RBot
│
├── internal/
│   ├── agent/
│   │   └── orchestrator.go   # Ensamblador de contexto, prompt y control de ejecución
│   ├── db/
│   │   └── sqlite.go         # Conexión SQLite, migraciones FTS5 y expansor de rutas
│   ├── ollama/
│   │   └── client.go         # Cliente HTTP de Ollama con soporte de Tool Calling
│   ├── mcp/
│   │   └── client.go         # Cliente stdio JSON-RPC 2.0 y gestor de servidores
│   ├── files/
│   │   ├── indexer.go        # Indexación de disco respetando exclusiones y bloqueos
│   │   └── finder.go         # Flujo de resolución inteligente de rutas
│   ├── apps/
│   │   └── scanner.go        # Escáner y parser de ficheros .desktop del escritorio
│   ├── skills/
│   │   └── manager.go        # Lector e indexador de SKILL.md de habilidades
│   ├── security/
│   │   └── permissions.go    # Filtro de rutas prohibidas y validación de riesgos
│   ├── voice/
│   │   └── audio.go          # Interfaz Go para controlar Piper, Whisper.cpp y grabadores
│   └── config/
│       └── config.go         # Gestor de lectura y autogeneración de configuración
│
├── config/
│   └── rbot.yaml             # Configuración activa de RBot
│
├── mcp/
│   └── mcp_config.json       # Configuración de servidores MCP stdio
│
├── dist/
│   └── rbot         # Binario compilado listo para usar
└── docs/                     # Carpeta de documentación del proyecto
    ├── dependencies.md       # Guía de dependencias necesarias del sistema y modelos
    ├── compilation.md        # Guía detallada de compilación e instalación
    ├── interferencias.md     # Guía de resolución de interferencias de audio y ruido
    └── db_y_mcp_config.md    # Guía de base de datos SQLite, habilidades y MCP
```

---

## 🛠️ Requisitos e Instalación

Para ver la lista completa de requisitos y dependencias del sistema/modelos de IA, consulta:
*   [dependencies.md](docs/dependencies.md)

Para compilar `whisper.cpp`, descargar los modelos locales e instalar todo el entorno, consulta:
*   [compilation.md](docs/compilation.md)

Para solucionar problemas de interferencias de audio y ruido (música, películas, Netflix, etc.), consulta:
*   [interferencias.md](docs/interferencias.md)

Para entender cómo funciona la base de datos local de SQLite, la indexación inteligente y cómo configurar servidores MCP externos en `mcp/mcp_config.json`, consulta:
*   [db_y_mcp_config.md](docs/db_y_mcp_config.md)

### Compilación e Instalación Rápida
Puedes descargar dependencias y compilar todo automáticamente en la carpeta `dist/` usando el script provisto:
```bash
chmod +x setup_and_build.sh
./setup_and_build.sh
```

---

## 🚀 Guía de Uso Rápido

Al ejecutar `./dist/rbot` por primera vez sin argumentos, se autogenera la configuración recomendada por defecto en `config/rbot.yaml`.

### 1. Indexar el Sistema
Antes de comenzar, permite que RBot indexe tus archivos y programas instalados:
```bash
# Indexa los archivos de tus carpetas permitidas en SQLite
./dist/rbot index paths

# Indexa tus accesos directos de aplicaciones (.desktop)
./dist/rbot index apps

# Escanea tu carpeta local de habilidades instaladas
./dist/rbot skills scan
```

### 2. Ejecutar mediante comandos de chat (CLI)
Interactúa directamente mediante texto:
```bash
./dist/rbot chat "Abre mi proyecto Convertsystems en VS Code"
./dist/rbot chat "Recuerda que mi correo es usuario@example.com en la categoría personal"
./dist/rbot chat "Abre el navegador e ingresa a whatsapp web"
```

### 3. Activar el Modo de Voz Continuo (Espectacular 🎙️)
Haz que RBot te escuche constantemente. Puedes despertarlo utilizando cualquiera de las palabras clave configuradas (`oye ronald`, `ey ronald`, `go ronald`, `ronald`, `rbot`):
```bash
./dist/rbot voice
```
Una vez que despiertas a RBot, se mantendrá en **escucha continua y activa** (modo conversación). Ya no es necesario repetir la palabra clave para cada orden posterior.

#### 💤 Comandos de Desactivación (Poner a Dormir)
Para poner a RBot en modo de espera silencioso de nuevo, puedes pronunciar frases naturales como:
* *"Eso es todo"* o *"Eso es todo por ahora"*
* *"Gracias"* o *"Gracias Ronald"*
* *"Vete a dormir"* o *"Duérmete"*
* *"Apágate"*, *"Desactívate"* o *"Nada más"*

RBot responderá *"Entendido señor, vuelvo al modo de espera"* y volverá a estar dormido esperando únicamente por una palabra de activación.
*Nota: Si no dices nada durante 3 minutos, RBot se dormirá automáticamente por inactividad para no capturar conversaciones accidentales.*

---

## 🧪 Pruebas Unitarias

El proyecto cuenta con una amplia suite de pruebas unitarias locales para validar el correcto funcionamiento de cada componente de manera aislada (sin requerir acceso a dispositivos de audio reales o conexiones de red externas):

```bash
# Ejecutar todas las pruebas unitarias del proyecto
go test -v ./...

# Ejecutar sin caché
go test -count=1 -v ./...
```

La suite cubre:
*   `rbot/cmd` (main): Limpieza de comandos de voz y alucinaciones de Whisper.
*   `internal/agent`: Prompt de sistema dinámico y flujo completo de razonamiento LLM + tool calling con Ollama mockeado.
*   `internal/config`: Carga, validación y resolución de rutas dinámicas.
*   `internal/db`: Conexiones SQLite, esquemas e índice virtual FTS5.
*   `internal/security`: Validaciones de criticidad y paths denegados.
*   `internal/files`: Indexación incremental recursiva, búsqueda inteligente y alias.
*   `internal/voice`: Conversión de audio, VAD en Go y diálogo fallback.
*   `internal/mcp`: Conexión de herramientas MCP mediante JSON-RPC.
*   `internal/apps`: Escaneo y parseo de lanzadores `.desktop`.

## ⚙️ Configuración (`config/rbot.yaml`)

El archivo de configuración YAML te permite parametrizar el comportamiento:
*   `agent.name`: Nombre con el que te responde el agente (ej. `RBot`).
*   `agent.wake_words`: Lista de frases que lo despiertan (ej: `["oye ronald", "ey ronald", "rbot"]`).
*   `model.model`: Modelo cargado en Ollama.
*   `files.allowed_roots`: Carpetas seguras donde RBot tiene permitido buscar y resumir archivos.
*   `security.blocked_paths`: Directorios privados donde RBot tiene estrictamente prohibido ingresar (bloquea por defecto carpetas `.ssh`, claves privadas y ficheros `.env`).
