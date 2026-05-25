# RBot Max Skill Pack

Paquete de habilidades para RBot Desktop Agent.

Este pack está diseñado para un entorno Linux/Arch/Hyprland, con RBot escrito en Go, SQLite + FTS5, MCP, tool calling y skills en formato `SKILL.md`.

## Qué contiene

Incluye 26 skills separadas por responsabilidad:

| Carpeta | Skill | Propósito |
|---|---|---|
| `router-core` | `router-core` | Núcleo de enrutamiento para decidir qué habilidad debe activarse, evitar conflictos y resolver órdenes ambiguas. |
| `security-guard` | `security-guard` | Guardia de seguridad para bloquear rutas privadas, comandos destructivos, exposición de secretos y operaciones peligrosas. |
| `memory-manager` | `memory-manager` | Manejo de memoria local SQLite/FTS5 para recordar rutas, alias, preferencias técnicas, proyectos y ejecuciones útiles. |
| `file-reader-search` | `file-reader-search` | Búsqueda, lectura, listado y resumen seguro de archivos y carpetas locales sin abrir aplicaciones externas. |
| `file-writer-safe` | `file-writer-safe` | Creación, edición, movimiento, renombrado y borrado seguro de archivos con confirmaciones por riesgo. |
| `app-launcher` | `app-launcher` | Abrir, enfocar y resolver aplicaciones instaladas desde .desktop, alias y base de datos local. |
| `window-workspace-manager` | `window-workspace-manager` | Control avanzado de ventanas y workspaces en Linux/Hyprland, incluyendo inspección de ventanas del navegador. |
| `browser-session-manager` | `browser-session-manager` | Manejo de navegador evitando duplicar pestañas, abriendo URLs, enfocando ventanas y separando búsquedas de reproducción. |
| `web-research` | `web-research` | Investigación web, lectura de URLs, resumen de páginas y búsquedas informativas sin crear pestañas innecesarias. |
| `youtube-media-control` | `youtube-media-control` | Reproducción de música y videos en YouTube evitando abrir pestañas duplicadas o activar búsquedas web equivocadas. |
| `system-control` | `system-control` | Control seguro de volumen, brillo, suspensión, apagado, reinicio, bloqueo de pantalla y acciones mecánicas del sistema Linux. |
| `linux-diagnostics` | `linux-diagnostics` | Diagnóstico de Linux/Arch/Hyprland: GPU NVIDIA, audio, red, disco, memoria, servicios, logs y rendimiento. |
| `arch-package-manager` | `arch-package-manager` | Gestión segura de paquetes en Arch Linux con pacman/yay, búsquedas, instalación, actualización y limpieza con confirmación. |
| `developer-workflow` | `developer-workflow` | Flujo general de desarrollo: detectar stack, correr tests, compilar, formatear, lint, dev server y scripts del proyecto. |
| `project-navigator` | `project-navigator` | Abrir, ubicar y resumir proyectos locales conocidos usando memoria, índice de archivos y VS Code. |
| `git-guardian` | `git-guardian` | Operaciones Git seguras: estado, ramas, commits, diffs, logs y protección ante push/reset/clean destructivo. |
| `node-nextjs-helper` | `node-nextjs-helper` | Ayuda especializada para Node, pnpm, Next.js, React, NestJS, Turbopack, Prisma y errores de build. |
| `go-rbot-helper` | `go-rbot-helper` | Skill especializada para RBot en Go: compilar, testear, revisar módulos, MCP, skills, SQLite y voz. |
| `docker-devops-helper` | `docker-devops-helper` | Manejo seguro de Docker, Docker Compose, contenedores, logs, redes, volúmenes y diagnósticos de servicios locales. |
| `database-prisma-postgres` | `database-prisma-postgres` | Ayuda segura para PostgreSQL, Prisma, migraciones, seeds, generación de cliente y diagnóstico de conexión. |
| `network-tools` | `network-tools` | Diagnóstico de red local, DNS, puertos propios, conectividad y servicios sin realizar acciones ofensivas. |
| `clipboard-notes` | `clipboard-notes` | Manejo de portapapeles y notas rápidas en Wayland/Linux con wl-clipboard. |
| `screen-capture-helper` | `screen-capture-helper` | Capturas de pantalla en Wayland/Hyprland para análisis visual, pruebas del escritorio y documentación. |
| `voice-command-cleaner` | `voice-command-cleaner` | Limpieza de comandos de voz, corrección de errores de Whisper, normalización de intención y activación robusta. |
| `testing-chaos-suite` | `testing-chaos-suite` | Generador y ejecutor de pruebas extremas para validar el orquestador, skills, voz, rutas, navegador, música y seguridad. |
| `clean-hexagonal-cli` | `clean-hexagonal-cli` | Skill especializada para usar una CLI de arquitectura limpia/hexagonal, generar estructura, módulos, casos de uso y documentación. |
## Instalación

**Nota:** Este paquete de habilidades ahora viene preintegrado y gestionado por el instalador universal de RBot. 

Si instalas RBot a través del instalador oficial (`install.sh`), estas skills se ubicarán automáticamente en:
`~/.local/share/rbot/skills/`

Si eres desarrollador, simplemente ejecuta el script `setup_dev.sh` en la raíz del proyecto para enlazar esta carpeta mediante symlinks.

```bash
# Escanear las habilidades instaladas en la memoria de RBot
./bin/rbot skills scan
```

## Recomendación importante

No mantengas activas skills antiguas con triggers muy genéricos si instalas este pack.

Especialmente revisa:

- `web-search`: evita triggers como `"qué es"`, `"quién es"`, `"dime sobre"` sin mencionar Internet.
- `youtube-music`: evita triggers como `"busca en youtube"` si también usas búsquedas web o navegación normal.
- `file-manager`: separa lectura de escritura/borrado para que el orquestador no ejecute una acción peligrosa por confusión.

## Orden recomendado de prioridad

1. `security-guard`
2. `router-core`
3. `voice-command-cleaner`
4. `memory-manager`
5. Skills de archivos
6. Skills de ventanas/navegador/música
7. Skills de desarrollo
8. Skills de diagnóstico
9. Skills de pruebas

## Herramientas asumidas

El contenido está escrito para un agente con herramientas equivalentes a:

- `system.run_command`
- `files.search_index`
- `files.read_file`
- `files.list_directory`
- `files.create_file`
- `files.create_directory`
- `browser.search`
- `browser.read_url`
- `browser.open_url`
- `browser.youtube_play`
- `browser.youtube_search`
- `apps.search`
- `apps.launch`
- `memory.search`
- `memory.save`

Si alguna herramienta todavía no existe en tu agente, puedes mantener la skill como especificación y mapearla después al nombre real de tus herramientas internas.

## Filosofía del pack

Cada skill debe ser estrecha, predecible y auditable. El problema de muchos agentes locales no es la falta de triggers, sino triggers demasiado amplios que activan la herramienta equivocada.

Este pack intenta que RBot:

- No abra Google cuando debe leer un archivo.
- No abra otra pestaña de YouTube si solo debe cambiar música.
- No borre archivos sin confirmar.
- No lea secretos.
- No confunda ventanas con pestañas.
- No ejecute comandos destructivos por una frase ambigua.
