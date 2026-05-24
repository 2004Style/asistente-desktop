# Guía de Base de Datos, Habilidades y Configuración de MCP en RBot

Esta guía explica la arquitectura de almacenamiento, el funcionamiento del sistema de habilidades (skills) y cómo configurar el protocolo MCP (Model Context Protocol) en RBot.

---

## 1. Base de Datos SQLite (`rbot.db`)

RBot utiliza una base de datos local SQLite ubicada por defecto en:
`~/.local/share/rbot/rbot.db` (definido en `config/rbot.yaml`).

### Estructura de Tablas Principales:
*   `skills`: Almacena el metadato parseado de los archivos `SKILL.md` de cada habilidad (nombre, versión, descripción, triggers de voz, permisos, estado habilitado/deshabilitado).
*   `app_launchers`: Almacena las aplicaciones de escritorio `.desktop` indexadas del sistema (para poder abrirlas con comandos como *"abre cursor"*, *"abre vscode"*, etc.).
*   `path_entries`: Almacena el índice de archivos y carpetas del disco dentro de los directorios permitidos (`allowed_roots`).
*   `mcp_servers` y `mcp_tools`: Guarda información sobre servidores Model Context Protocol conectados y sus herramientas registradas.
*   `search_index`: Índice de búsqueda virtual usando FTS5 de SQLite para búsquedas rápidas semánticas y por palabras clave.

---

## 2. Inicialización Automática de Datos (¡Nuevo!)

Para evitar que tu base de datos comience vacía y tengas que rellenarla de forma manual, hemos modificado el inicio del ejecutable (`cmd/main.go`) para que realice las siguientes tareas de manera automática en el arranque:

1.  **Autodescubrimiento de Habilidades**: Si en tu `rbot.yaml` tienes `auto_discover: true`, RBot escaneará la carpeta `~/.local/share/rbot/skills` en busca de subcarpetas con un archivo `SKILL.md`. Las registrará en la base de datos y las **habilitará automáticamente** (`enabled = 1`) para que no tengas que hacer `enable-all` cada vez.
2.  **Autoindexación de Aplicaciones**: Al iniciar, si RBot detecta que la tabla de aplicaciones está vacía, iniciará un escaneo en segundo plano para indexar todos los lanzadores del sistema.
3.  **Autoindexación de Archivos**: Al iniciar, si detecta que la tabla de rutas de archivos está vacía, ejecutará un escaneo de archivos en segundo plano bajo las rutas configuradas en `allowed_roots` de `rbot.yaml`.

---

## 3. Comandos Manuales de Mantenimiento

Si en cualquier momento deseas refrescar, re-indexar o gestionar las habilidades de forma manual, puedes utilizar los siguientes comandos desde la terminal:

### Habilidades (Skills)
*   **Escanear habilidades manual**:
    ```bash
    ./bin/rbot skills scan
    ```
    *(Busca nuevos archivos SKILL.md y los registra en la base de datos).*
*   **Listar habilidades registradas**:
    ```bash
    ./bin/rbot skills list
    ```
*   **Habilitar una habilidad específica**:
    ```bash
    ./bin/rbot skills enable <nombre-de-la-skill>
    ```
*   **Habilitar todas las habilidades registradas**:
    ```bash
    ./bin/rbot skills enable-all
    ```
*   **Deshabilitar una habilidad específica**:
    ```bash
    ./bin/rbot skills disable <nombre-de-la-skill>
    ```

### Lanzadores de Aplicaciones y Archivos
*   **Actualizar/Indexar aplicaciones instaladas en el sistema**:
    ```bash
    ./bin/rbot index apps
    ```
    *(Escanea tus archivos `.desktop` en `/usr/share/applications` y `~/.local/share/applications` para que RBot pueda abrir tus apps).*
*   **Indexar archivos de tus carpetas configuradas**:
    ```bash
    ./bin/rbot index paths
    ```
    *(Indexa archivos y carpetas dentro de `allowed_roots` en `rbot.yaml` para lectura/resumen/búsqueda).*

---

## 4. Configuración de Servidores MCP (Model Context Protocol)

El protocolo Model Context Protocol (MCP) permite a RBot expandir sus capacidades conectándose a servidores externos que exponen herramientas adicionales para el LLM.

### Archivo de Configuración: `mcp/mcp_config.json`
RBot lee la configuración de servidores MCP de la ruta indicada en `rbot.yaml` (`mcp.config_path`). Hemos creado una configuración inicial en `mcp/mcp_config.json` con el siguiente contenido de ejemplo listo para usar:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "~/Documentos",
        "~/Descargas"
      ]
    }
  }
}
```

### ¿Cómo añadir nuevos servidores MCP?
Puedes agregar cualquier servidor MCP compatible con transporte `stdio` en el archivo `mcp/mcp_config.json`. Aquí tienes algunos ejemplos comunes:

#### 1. Servidor Filesystem (Sistema de Archivos)
Permite al asistente leer, escribir y explorar archivos en directorios autorizados:
```json
"filesystem": {
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-filesystem",
    "/ruta/a/tus/documentos",
    "/otra/ruta/permitida"
  ]
}
```

#### 2. Servidor Fetch (Descarga de contenido web)
Permite al asistente descargar contenido HTML de páginas web y convertirlo a texto limpio para procesarlo:
```json
"fetch": {
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-fetch"
  ]
}
```

#### 3. Servidor de Consultas a bases de datos SQLite
Si quieres que el bot consulte bases de datos relacionales locales:
```json
"sqlite": {
  "command": "npx",
  "args": [
    "-y",
    "@modelcontextprotocol/server-sqlite",
    "--db",
    "/ruta/a/tu/base_de_datos.db"
  ]
}
```

### Comprobar servidores y herramientas MCP activas:
Una vez configurados los servidores en `mcp/mcp_config.json`, inicia RBot y ejecuta el siguiente comando para ver qué herramientas se han cargado correctamente:
```bash
./bin/rbot mcp list
```
Este comando listará los servidores activos y cada una de las herramientas que exponen (por ejemplo, `read_file`, `write_file`, `fetch`, etc.) que estarán inmediatamente disponibles para que Ollama las utilice durante tus conversaciones.

---

## 5. Creación y Estructura de Habilidades (Skills)

RBot posee un motor de orquestación extensible mediante **habilidades** escritas en Markdown con cabecera (frontmatter) YAML. Esto permite modularizar el comportamiento del asistente e inyectar prompts específicos según el contexto del usuario.

### ¿Dónde viven las habilidades?
Cada habilidad debe estar en una subcarpeta propia dentro de la ruta especificada en `skills.path` de `config/rbot.yaml` (por defecto `~/.local/share/rbot/skills`). Cada carpeta debe contener al menos un archivo llamado `SKILL.md`.

Por ejemplo:
```txt
~/.local/share/rbot/skills/
├── youtube-music/
│   └── SKILL.md
└── mi-habilidad-personalizada/
    └── SKILL.md
```

### Formato de `SKILL.md`

Un archivo `SKILL.md` consta de dos partes principales: la cabecera frontmatter delimitada por `---`, y el cuerpo con las instrucciones de orquestación.

```markdown
---
name: nombre-de-la-habilidad
description: Explicación de qué hace esta habilidad (utilizada por la búsqueda semántica).
version: 1.0.0
author: Tu Nombre
risk_level: low | medium | high
voice_triggers:
  - "frase clave de activación 1"
  - "frase clave de activación 2"
permissions:
  - exec:nombre_binario  # Si requiere ejecutar algún programa del sistema
  - network             # Si requiere acceso a internet o URLs externas
---

# Título de la Habilidad

Aquí se escribe el prompt de sistema específico que se le inyectará a Ollama cuando esta habilidad esté activa.

## Instrucciones de Orquestación:
1. Si el usuario pide X, ejecuta la herramienta Y.
2. NUNCA menciones URLs o detalles internos de la herramienta al responder.
3. Responde siempre con tono educado y profesional.
```

### Parámetros del Frontmatter:
* **`name`**: Identificador único de la habilidad (ej. `youtube-music`).
* **`description`**: Descripción concisa. RBot la utiliza junto al motor de indexación FTS5 de SQLite para realizar búsquedas semánticas y activar la habilidad correspondiente si no coincide ningún trigger exacto.
* **`risk_level`**: Nivel de riesgo (`low`, `medium`, `high`). Si es `high`, RBot le pedirá confirmación explícita al usuario por voz o consola antes de realizar acciones mecánicas o comandos de sistema destructivos.
* **`voice_triggers`**: Lista de cadenas exactas o subfrases que activan de inmediato esta habilidad. Si el comando del usuario contiene alguna de estas frases, RBot activa la habilidad sin necesidad de recurrir a consultas pesadas en la base de datos.
* **`permissions`**: Lista de permisos requeridos (por ejemplo, permitir ejecutar ejecutables del sistema mediante `exec:<binario>`). El orquestador de RBot valida estos permisos frente a su módulo de seguridad antes de despachar tareas al LLM.

### Integración con el LLM:
Cuando el usuario le da una orden al asistente (ya sea por voz o chat), el orquestador:
1. Compara la orden con los `voice_triggers` de las habilidades habilitadas y busca coincidencias semánticas en SQLite.
2. Si encuentra habilidades coincidentes, lee el archivo `SKILL.md` e inyecta el cuerpo como parte del prompt del sistema (instrucciones contextuales) y sus herramientas registradas dentro de la llamada a Ollama.
3. Esto garantiza que Ollama responda y use las herramientas de forma especializada para esa tarea en particular, ahorrando tokens y reduciendo alucinaciones.
