# RBot como runtime local extensible para Linux

## Visión general del proyecto

RBot, tal como lo has ido definiendo, no es simplemente “un bot con Ollama”, sino un **runtime local de automatización personal para Linux**: un sistema residente que escucha, razona, planea, aplica políticas de seguridad, ejecuta herramientas reales, recuerda tareas, notifica, muestra presencia visual y puede ampliarse con skills y un workspace editable. En tu propio blueprint ya aparece esa evolución hacia una arquitectura modular con `rbot`, `rbotd`, `rbotctl`, `rbot-hud`, `EventBus`, `ToolRegistry`, `PolicyEngine`, `Intent Router`, `Planner`, scheduler, HUD y workspace. fileciteturn0file0

La idea general del proyecto es unir en un solo sistema cuatro cosas que rara vez están bien integradas al mismo tiempo: **interfaz natural**, **control real del sistema**, **seguridad fuerte** y **extensibilidad local-first**. En la práctica, eso significa que RBot debe poder pasar de “entiendo lo que me dices” a “ejecuto una acción real, la audito, la confirmo si es riesgosa, la visualizo en pantalla, la recuerdo después y además puedo cambiar de proveedor de modelo sin romper el resto del runtime”. Ese diseño de componentes y sus fases ya está reflejado en tu documentación de trabajo. fileciteturn0file0turn0file5turn0file4turn0file3turn0file2turn0file1

La corrección estratégica más importante que sale de esta investigación es esta: **RBot no debe depender de Ollama; debe depender de una interfaz de proveedor LLM**. Esa separación coincide con patrones que hoy ya usan herramientas modernas. El SDK de agentes de OpenAI recomienda separar **modelo** y **transporte/proveedor**, elegir modelos explícitamente en producción y usar superficies de provider/adapter cuando haya una pila mixta. OpenClaw, por su parte, también separa proveedor, modelo y runtime, y en su onboarding pide explícitamente proveedor, autenticación y modelo por defecto. citeturn17view0turn19view0turn19view3turn19view4

Eso enlaza exactamente con lo que pediste: **desde la instalación debe poder elegirse el proveedor**, y **después debe existir una interfaz donde cambiar proveedor y modelo sin editar archivos a mano**. Ese patrón no es inventado: OpenClaw tiene un onboarding guiado que configura proveedor/auth/workspace/daemon y acepta proveedores personalizados con modo de compatibilidad, base URL, API key y model ID; OpenHands, a su vez, obliga a seleccionar `LLM Provider`, `LLM Model` y `API Key` en el primer arranque o desde `Settings`, y también permite `Base URL` y `Custom Model`. citeturn19view0turn19view1turn19view2turn12view0turn12view1

Desde el punto de vista del usuario, la visión correcta para RBot es esta: **local-first, pero no local-only**. En modo Ollama local, el backend puede hablar con `http://localhost:11434`, donde no hace falta autenticación y los modelos se descubren por `GET /api/tags`; en modo OpenAI, la API usa claves Bearer y expone `GET /v1/models`; y en las superficies de Codex oficiales, el acceso puede hacerse con cuenta de ChatGPT o con API key según el cliente y el caso de uso. Eso significa que RBot debe poder trabajar con privacidad local cuando conviene, y con proveedores remotos cuando necesites más capacidad o un modelo concreto. citeturn9view0turn9view1turn6view0turn7view1turn13view0turn13view1turn13view2

## Proveedores, instalación y cambio desde la interfaz

La respuesta corta a tu requisito es: **sí, debe hacerse exactamente así**. RBot debe incorporar la elección de proveedor **desde el onboarding inicial**, y después debe dejar cambiar **proveedor, autenticación y modelo** desde una interfaz de configuración independiente del HUD. Esa conclusión está bien alineada con el estado actual de herramientas comparables: OpenClaw ya configura “model provider and auth” en el onboarding y OpenHands ya resuelve la elección de proveedor/modelo en un popup inicial o en `Settings`. citeturn19view0turn12view0turn12view1

### Lo que debe ocurrir durante la instalación

Durante la instalación o primer arranque, RBot debería ofrecer un asistente tipo `rbot setup` o `rbot onboard` con un flujo parecido al de OpenClaw: elegir proveedor, método de autenticación, workspace, daemon y parámetros básicos. OpenClaw describe exactamente ese enfoque: su onboarding configura **provider + auth**, workspace, gateway y daemon; además, para proveedores no listados, permite escoger un **modo de compatibilidad**, introducir **Base URL**, **API key** y **Model ID**. citeturn19view0turn19view1

Ese wizard de RBot debería presentar, como mínimo, estas rutas:

- **Ollama local**: detectar si Ollama está instalado y si responde en `localhost:11434`; si no está disponible, ofrecer instalarlo o dejar la configuración pendiente. La propia documentación oficial de Ollama distingue instalación por Linux, macOS y Windows, y deja claro que la API local funciona en `http://localhost:11434` sin autenticación. citeturn14search0turn14search2turn14search3turn9view0
- **Ollama remoto / ollama.com**: pedir `Base URL` y `OLLAMA_API_KEY`, porque el acceso directo a `https://ollama.com/api` sí requiere clave Bearer. citeturn9view0
- **OpenAI API**: pedir `OPENAI_API_KEY`, guardar la referencia al secreto y permitir listar modelos por `GET /v1/models`. La API oficial de OpenAI usa API keys por Bearer y documenta el endpoint de listado de modelos. citeturn6view0turn7view1
- **OpenAI/Codex por suscripción**: permitir un modo de autenticación tipo ChatGPT/Codex sign-in cuando esa superficie aplique. OpenAI documenta que Codex está disponible con planes de ChatGPT y que sus clientes CLI/IDE permiten autenticarse con cuenta de ChatGPT; además, OpenClaw documenta de forma explícita la separación entre ruta de suscripción/OAuth y ruta de API key para OpenAI/Codex. citeturn13view0turn13view1turn13view2turn19view4turn20view0
- **Proveedor compatible personalizado**: pedir compatibilidad OpenAI-like, Base URL, API key opcional y model ID. Esta capacidad ya aparece como patrón en el onboarding de OpenClaw. citeturn19view0

### Lo que debe ocurrir después en la interfaz

Cambiar proveedor **no debería hacerse desde el HUD**. Tu propia planificación de la fase visual ya deja claro que el HUD no es el lugar para un panel completo de configuración y que su foco debe ser **estado visual + notificaciones + presencia**, no administración del sistema. fileciteturn0file1

Por eso, la opción correcta es una **interfaz de control o Settings UI separada**. OpenHands lo resuelve con un popup inicial y un botón de `Settings` donde quedan agrupados `LLM Provider`, `LLM Model`, `API Key` y `Base URL`; OpenClaw, en su lado, separa el dashboard/Control UI de su asistente visual y expone además comandos de modelos como `models list`, `models set` y `models status`. citeturn12view0turn12view1turn19view1turn20view0

Para RBot, esa interfaz debería mostrar:

- proveedor activo;
- modelo activo;
- método de autenticación;
- estado de conexión;
- catálogo de modelos disponibles;
- capacidades del modelo seleccionado;
- botón de prueba;
- selector de proveedor por clic;
- selector de modelo dentro del proveedor;
- fallback opcional.

Y además debería convivir con CLI para administración rápida:

```bash
rbotctl providers list
rbotctl providers status
rbotctl providers use ollama
rbotctl providers use openai
rbotctl models list --provider ollama
rbotctl models list --provider openai
rbotctl models switch ollama qwen3.5
rbotctl models switch openai gpt-5.5
```

### Arquitectura mínima recomendada para proveedores

La mejor decisión aquí es no mezclar **proveedor**, **modelo**, **runtime** y **canal** en una sola cadena de configuración. OpenClaw separa explícitamente esas capas, y OpenAI Agents SDK también insiste en separar el contrato de **modelo** del de **transport/proveedor**. citeturn19view4turn17view0

La interfaz mínima para RBot debería ser algo así:

```go
type Provider interface {
    Name() string
    AuthModes() []string
    ListModels(ctx context.Context) ([]ModelInfo, error)
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    SupportsToolCalling() bool
    SupportsStreaming() bool
    SupportsConversationState() bool
    SupportsVision() bool
}
```

Y el layout recomendado:

```txt
internal/llm/
├── provider.go
├── registry.go
├── manager.go
├── capabilities.go
├── auth.go
├── ollama/
│   ├── provider.go
│   └── client.go
├── openai/
│   ├── provider.go
│   └── client.go
└── compatible/
    ├── provider.go
    └── client.go
```

Ese diseño no es caprichoso. La propia documentación muestra que las capacidades no son idénticas entre proveedores. OpenAI Responses soporta herramientas, streaming, entrada multimodal y continuidad de conversación; Ollama soporta tool calling y compatibilidad con `/v1/responses`, pero en esa compatibilidad no ofrece el sabor stateful completo de `previous_response_id` o `conversation`. Por eso RBot debe guardar **capabilities por proveedor/modelo**, y no asumir paridad total. citeturn8view1turn8view2turn8view3turn9view2turn9view3

### Checklist transversal de proveedores y modelos

- [ ] Crear `internal/llm/provider.go`.
- [ ] Crear `internal/llm/registry.go`.
- [ ] Crear `internal/llm/manager.go`.
- [ ] Implementar provider `ollama`.
- [ ] Implementar provider `openai`.
- [ ] Implementar provider `compatible` para endpoints OpenAI-like.
- [ ] Soportar `auth_mode` por proveedor.
- [ ] Soportar `none` para Ollama local.
- [ ] Soportar `api_key` para OpenAI.
- [ ] Soportar `chatgpt_oauth` / `codex_signin` donde aplique.
- [ ] Crear comando `rbot setup` o `rbot onboard`.
- [ ] Detectar Ollama local durante onboarding.
- [ ] Permitir proveedor personalizado con `Base URL` + `API key` + `model ID`.
- [ ] Implementar `providers.list`.
- [ ] Implementar `providers.status`.
- [ ] Implementar `providers.set_default`.
- [ ] Implementar `models.list`.
- [ ] Implementar `models.current`.
- [ ] Implementar `models.switch`.
- [ ] Cachear catálogo de modelos por proveedor.
- [ ] Guardar `capabilities_json` por modelo.
- [ ] Diferenciar capacidades: tools, streaming, vision, conversation state.
- [ ] Añadir una interfaz de configuración separada del HUD para cambiar proveedor/modelo.
- [ ] Mantener un fallback CLI completo aunque la interfaz gráfica falle.

### Limitaciones y decisiones aún abiertas

Hay tres decisiones que conviene dejar explícitas:

Primero, **si el cambio de proveedor por UI debe llegar ya en una interfaz web ligera antes de la fase final**, o si prefieres que el primer camino sea solo `CLI + wizard`, dejando el panel gráfico de configuración para el cierre del proyecto. Como el HUD no debe inflarse con Settings, lo más limpio es que el **wizard llegue temprano** y el **panel completo llegue más adelante**. fileciteturn0file1

Segundo, **cómo almacenar secretos**. Las fuentes oficiales dejan claro que las API keys son sensibles y deben vivir del lado servidor o en un almacén seguro, no expuestas en cliente. RBot debería usar variables de entorno, backend de secretos o referencias seguras, no claves planas en la UI. citeturn6view0turn6view1

Tercero, **si quieres soportar login OAuth/Codex nativo dentro de RBot o delegarlo a un helper externo**. La investigación confirma que ese patrón existe en productos como OpenClaw y en clientes oficiales de Codex, pero su integración exacta depende de cuánto quieras implementar del flujo de autenticación. citeturn13view0turn13view1turn19view4turn20view0

## Objetivo general y objetivos específicos

El objetivo general del proyecto es **convertir RBot en un runtime local, seguro, extensible y multi-proveedor para Linux**, capaz de recibir órdenes por voz o texto, razonar con un modelo elegible por el usuario, planificar acciones, aplicar políticas, ejecutar herramientas del sistema, gestionar memoria y productividad, y ofrecer una presencia visual útil sin sacrificar control local ni trazabilidad. Ese es exactamente el sentido del roadmap que ya has construido. fileciteturn0file0

Los objetivos específicos, consolidados y ajustados con la investigación de proveedores, quedan así:

- separar el sistema en binarios funcionales;
- convertirlo en daemon persistente;
- crear una capa IPC estable;
- construir un EventBus central;
- crear un Tool Registry universal;
- crear un Executor seguro y observable;
- aplicar un PolicyEngine fuerte;
- construir un Intent Router local con scoring;
- planificar órdenes compuestas;
- persistir confirmaciones;
- controlar escritorio, teclado, mouse, navegador y media;
- hacer a RBot proactivo con scheduler, tareas y recordatorios;
- dotarlo de HUD visual;
- crear un workspace editable;
- evolucionar skills con validación y permisos;
- añadir soporte multi-modelo y multi-proveedor;
- permitir elegir proveedor en el onboarding;
- permitir cambiar proveedor y modelo desde una interfaz de configuración, además del CLI. fileciteturn0file0

## Mapa de fases

Tu roadmap queda muy bien resumido en siete etapas progresivas: **runtime base**, **ejecución segura**, **orquestación inteligente**, **control del escritorio**, **productividad proactiva**, **HUD visual** y **plataforma extensible**. Ese mapa ya aparece de forma consistente en tu documentación y encaja muy bien con el orden natural de construcción de un asistente local serio. fileciteturn0file0

En una línea por fase, lo que se busca lograr es esto:

- **Fase 1**: dejar de ser un CLI monolítico y convertirse en runtime residente.
- **Fase 2**: sacar las herramientas del orquestador y pasarlas a una capa segura, reusable y auditable.
- **Fase 3**: entender mejor la intención del usuario, dividir órdenes, construir planes y persistir confirmaciones.
- **Fase 4**: ganar control real del entorno Linux.
- **Fase 5**: pasar de reactivo a proactivo con scheduler y productividad.
- **Fase 6**: dar presencia visual y feedback continuo.
- **Fase 7**: cerrar el proyecto como una plataforma configurable y ampliable. fileciteturn0file0turn0file5turn0file4turn0file3turn0file2turn0file1

Importante: **la capa de proveedores LLM no es una octava fase**. Es un eje transversal que debe comenzar en la fase de runtime y asentarse definitivamente en la fase de plataforma extensible. fileciteturn0file0

## Desarrollo detallado de las fases

### Runtime base

Esta es la fase inicial. Su meta es transformar RBot de un ejecutable centrado en comandos puntuales en un sistema con daemon, lifecycle, sockets, eventos y cliente de control. También es el mejor lugar para introducir el **wizard de onboarding** y la configuración base de proveedores, porque desde ese momento ya debes fijar dónde vive el daemon, dónde se guarda la configuración y cómo se elige el modelo activo. Esa dirección está alineada con tu blueprint y con el patrón de onboarding guiado visto en OpenClaw. fileciteturn0file0 citeturn19view0turn19view1

**Checklist de la fase**

- [ ] Separar `cmd/rbot`, `cmd/rbotd`, `cmd/rbotctl`.
- [ ] Crear `internal/runtime/daemon.go`.
- [ ] Crear `internal/runtime/lifecycle.go`.
- [ ] Crear `internal/runtime/eventbus.go`.
- [ ] Crear `internal/ipc/server.go` y `client.go`.
- [ ] Definir `rbot.sock` y `events.sock` en `XDG_RUNTIME_DIR`.
- [ ] Añadir lock de instancia única.
- [ ] Crear servicio `systemd` de usuario para `rbotd`.
- [ ] Mantener comandos legacy mínimos mientras dura la transición.
- [ ] Crear `rbot setup` / `rbot onboard`.
- [ ] Permitir elegir proveedor durante la instalación.
- [ ] Guardar `active_provider`, `active_model` y auth básica en config.
- [ ] Implementar `rbotctl status`.
- [ ] Implementar `rbotctl providers status`.
- [ ] Implementar `rbotctl models current`.
- [ ] Probar arranque, parada y reconexión del daemon.

### Ejecución segura

Esta fase corresponde a mover la ejecución de herramientas fuera del orquestador grande y concentrarla en `ToolHandler`, `Registry`, `Executor` y `PolicyEngine`. Aquí nace el contrato universal de tools, la auditoría y la idea de riesgo por nivel. Tu plan de trabajo ya lo define muy claramente con `tool_registry`, `action_log`, adaptación MCP y bloqueo de comandos peligrosos. fileciteturn0file5

**Checklist de la fase**

- [ ] Crear `ToolHandler`.
- [ ] Crear `ToolResult`.
- [ ] Crear `Registry`.
- [ ] Crear `Executor`.
- [ ] Crear `PlanResult`.
- [ ] Crear `PolicyDecision`.
- [ ] Crear `PolicyEngine`.
- [ ] Crear `tool_registry` en SQLite.
- [ ] Extender `action_log` con `plan_id`.
- [ ] Migrar tools de `desktop`, `browser`, `files` y `system`.
- [ ] Añadir `system.run_command_safe`.
- [ ] Forzar timeouts por paso.
- [ ] Bloquear `rm -rf`, `sudo` peligroso y `curl | sh`.
- [ ] Registrar y ejecutar adapters MCP como tools.
- [ ] Emitir `tool.started`, `tool.finished`, `tool.failed`.
- [ ] Guardar toda ejecución en auditoría.
- [ ] Escribir tests de registry, executor y policy.

### Orquestación inteligente

En esta etapa RBot deja de reaccionar solo por frases “directas” y empieza a trabajar con normalización, scoring, extracción de slots, separación de órdenes compuestas, construcción de planes y confirmaciones persistentes. En tu diseño, esta fase también añade `pending_confirmations` en SQLite para ejecutar exactamente el plan aprobado sin volver a razonar desde cero. fileciteturn0file4

**Checklist de la fase**

- [ ] Crear `normalizer.go`.
- [ ] Limpiar wake words y errores frecuentes de transcripción.
- [ ] Crear `intent.go`.
- [ ] Crear `router.go`.
- [ ] Crear `scoring.go`.
- [ ] Crear `slots.go`.
- [ ] Crear `splitter.go` con regla verbal estricta.
- [ ] Evitar splits erróneos como “rock y metal”.
- [ ] Crear `planner/plan.go`.
- [ ] Crear `planner/builder.go`.
- [ ] Crear lógica de dependencias y recovery.
- [ ] Crear `pending_confirmations` en SQLite.
- [ ] Detectar “sí”, “confirmo”, “acepto”, “no”, “cancela”.
- [ ] Ejecutar el plan persistido sin re-LLM.
- [ ] Revalidar PolicyEngine antes de ejecutar.
- [ ] Emitir `intent.detected`, `plan.created`, `policy.confirmation_required`, `confirmation.accepted`, `confirmation.cancelled`.

### Control del escritorio

Aquí RBot gana manos sobre Linux: ventanas, teclado, mouse, pestañas, YouTube, música, volumen y navegación. Tu plan ya contempla detector de entorno, backends Hyprland/Sway/X11/Noop, session manager de navegador y media control con `playerctl` y `wpctl/pactl`. fileciteturn0file3

**Checklist de la fase**

- [ ] Crear detector de capacidades del entorno.
- [ ] Detectar Wayland, X11, Hyprland, Sway y utilidades instaladas.
- [ ] Persistir `environment_capabilities`.
- [ ] Crear `WindowManager`.
- [ ] Implementar backend Hyprland.
- [ ] Implementar backend Sway.
- [ ] Implementar backend X11.
- [ ] Implementar backend Noop.
- [ ] Crear `desktop.list_windows`, `active_window`, `focus_window`, `close_window`, `move_window`.
- [ ] Crear `InputController`.
- [ ] Crear mapeo de voz a teclas.
- [ ] Implementar backends `xdotool`, `wtype`, `ydotool`.
- [ ] Crear `input.type_text`, `press_key`, `hotkey`, `mouse_move`, `mouse_click`, `mouse_scroll`.
- [ ] Bloquear escritura de contraseñas y acciones críticas sin confirmación.
- [ ] Crear `browser.open_or_reuse` y `youtube_open_or_reuse`.
- [ ] Crear control multimedia: `play`, `pause`, `resume`, `next`, `previous`, `volume_up`, `volume_down`, `mute`, `status`.
- [ ] Asegurar que “cambia la música” resuelva a `media.next` y no a abrir navegador.
- [ ] Probar todas las rutas de foco, texto, pestañas y media.

### Productividad proactiva

Esta fase convierte a RBot en asistente de agenda, tareas y tiempo. Tu plan ya define `tasks`, `reminders`, `meetings`, `scheduled_jobs`, `timeparser`, quiet hours, notificación por escritorio/voz/HUD/sonido y recuperación tras reinicio. También deja claro que los jobs diferidos nunca deben saltarse el `PolicyEngine`. fileciteturn0file2

**Checklist de la fase**

- [ ] Crear tablas `tasks`, `reminders`, `meetings`, `scheduled_jobs`.
- [ ] Añadir `locked_at`, `locked_by`, `completed_at` a jobs.
- [ ] Crear índices por `status/run_at`.
- [ ] Guardar fechas en RFC3339/UTC.
- [ ] Añadir timezone configurable.
- [ ] Definir recurrencias con RRULE.
- [ ] Crear `timeparser` en español.
- [ ] Soportar expresiones relativas y recurrentes.
- [ ] Devolver `confidence` y `ambiguous`.
- [ ] Crear scheduler persistente.
- [ ] Ejecutar recovery al iniciar `rbotd`.
- [ ] Marcar jobs tardíos o expirados según política.
- [ ] Crear `notification_log`.
- [ ] Implementar `notify.desktop`, `notify.voice`, `notify.hud`, `notify.sound`.
- [ ] Implementar Quiet Hours.
- [ ] Crear tools `tasks.*`, `reminders.*`, `meetings.*`.
- [ ] Separar reminder textual de ejecución real de tools.
- [ ] Hacer que toda acción programada pase por PolicyEngine.
- [ ] Añadir skills de productividad.
- [ ] Probar recordatorios, meetings, repetición y bloqueos de seguridad.

### HUD visual

Esta fase añade la presencia visual de RBot: escuchar, pensar, planear, ejecutar, hablar, notificar y desvanecerse al dormir. Tu plan lo acota bien como un proceso separado (`rbot-hud`) que consume eventos NDJSON y no ejecuta herramientas. Y, muy importante para lo que pediste, también deja fuera de esta fase el panel completo de configuración. fileciteturn0file1

**Checklist de la fase**

- [ ] Crear `cmd/rbot-hud/main.go`.
- [ ] Elegir backend visual y validarlo temprano.
- [ ] Crear `internal/hud/events.go`, `state.go`, `client.go`, `mapper.go`, `renderer.go`.
- [ ] Implementar reconexión automática a `events.sock`.
- [ ] Definir estados: sleeping, listening, transcribing, thinking, planning, executing, speaking, notification, error, disconnected.
- [ ] Añadir `voice.audio_level` y `tts.audio_level` con rate limit.
- [ ] Crear `NotificationCard` y `ConfirmationCard` separadas.
- [ ] Añadir cola visual de notificaciones.
- [ ] Evitar que el HUD robe foco cuando el sistema lo permita.
- [ ] Exponer `rbotctl hud show`, `hide` y `test`.
- [ ] Mostrar, como mucho, un badge visual del proveedor/modelo activo; no meter aún todo el panel de configuración.
- [ ] Validar wake/listening/thinking/executing/speaking/end-to-end.

### Plataforma extensible

Esta es la fase de cierre. Aquí RBot se convierte de verdad en plataforma: workspace editable, skills más potentes, políticas endurecibles, memoria visible, shortcuts seguros, validación, staging, hash, cuarentena y herramientas de gestión. Y aquí es donde encaja mejor tu requisito de **una interfaz de configuración real para proveedor/modelo**, porque ya no es el HUD, sino un panel de control del runtime. Tu blueprint ya ubica el cierre del proyecto en workspace + skills + políticas editables + memoria visible, y esta investigación recomienda añadir ahí el **Control UI de configuración del modelo** como remate natural. fileciteturn0file0 citeturn12view0turn12view1turn19view1turn20view0

**Checklist de la fase**

- [ ] Crear workspace por defecto.
- [ ] Crear `AGENTS.md`, `IDENTITY.md`, `TOOLS.md`, `POLICIES.md`, `MEMORY.md`, `TASKS.md`, `SHORTCUTS.md`.
- [ ] Crear `internal/workspace/*`.
- [ ] Crear `ContextBuilder`.
- [ ] Validar que `POLICIES.md` no relaje reglas críticas del core.
- [ ] Convertir shortcuts a `Plan`.
- [ ] Pasar shortcuts por PolicyEngine.
- [ ] Ampliar `internal/skills/*` con parser, validator, installer, quarantine, permissions, examples.
- [ ] Proteger instalación de ZIP malicioso.
- [ ] Añadir `status` avanzado a skills.
- [ ] Añadir migraciones versionadas.
- [ ] Crear tools `workspace.*`, `skills.*`, `shortcuts.*`, `policies.*`, `memory.*`.
- [ ] Emitir eventos de workspace/skills.
- [ ] Crear panel de configuración separado del HUD para:
  - [ ] cambiar proveedor;
  - [ ] cambiar modelo;
  - [ ] ver auth y estado;
  - [ ] probar conexión;
  - [ ] configurar `Base URL`;
  - [ ] administrar fallback;
  - [ ] ver capacidades del modelo;
  - [ ] recargar configuración sin reiniciar.
- [ ] Permitir también todo eso desde `rbotctl`.
- [ ] Probar cambio de Ollama local a OpenAI/Codex sin tocar el orquestador.
- [ ] Probar instalación y validación de skills locales.
- [ ] Probar rechazo de políticas y triggers inseguros.

## Checklist integral del proyecto

Este checklist consolida todas las fases y añade el eje transversal de proveedores/modelos, que ahora pasa a ser un requisito central del proyecto. La idea final es que RBot quede como un sistema instalable, seguro, gobernable y reutilizable, no como una demo acoplada a un solo backend. fileciteturn0file0 citeturn17view0turn19view0turn19view3turn20view0

### Runtime y despliegue

- [ ] `rbot`, `rbotd`, `rbotctl` y `rbot-hud` separados.
- [ ] `rbotd` funcionando como daemon persistente.
- [ ] services `systemd --user` para daemon y HUD.
- [ ] `rbot setup` / `rbot onboard`.
- [ ] onboarding repetible sin destruir configuración previa.
- [ ] status/health checks del sistema.
- [ ] logs claros y trazables.
- [ ] bloqueo de doble instancia.
- [ ] limpieza de sockets obsoletos.
- [ ] shutdown limpio.

### IPC, eventos y observabilidad

- [ ] JSON-RPC para comandos.
- [ ] NDJSON para eventos.
- [ ] `EventBus` thread-safe.
- [ ] colas o buffers para subscribers lentos.
- [ ] eventos de vida del daemon.
- [ ] eventos de voz.
- [ ] eventos de tools.
- [ ] eventos de policy/confirmación.
- [ ] eventos de scheduler.
- [ ] eventos de HUD.
- [ ] eventos de workspace/skills.
- [ ] correlación por `plan_id` y/o `request_id`.

### Proveedores, modelos y autenticación

- [ ] abstracción `Provider`.
- [ ] registry de proveedores.
- [ ] provider `ollama`.
- [ ] provider `openai`.
- [ ] provider `compatible` para endpoints OpenAI-like.
- [ ] modo `ollama local` sin auth.
- [ ] modo `ollama remote/cloud` con API key.
- [ ] modo `openai api_key`.
- [ ] modo `codex/chatgpt sign-in` donde aplique.
- [ ] selección de proveedor en onboarding.
- [ ] selección de modelo en onboarding.
- [ ] detección/listado de modelos por proveedor.
- [ ] `models.list`.
- [ ] `models.current`.
- [ ] `models.switch`.
- [ ] `providers.list`.
- [ ] `providers.status`.
- [ ] `providers.set_default`.
- [ ] `auth_mode` por proveedor.
- [ ] caché de catálogo por proveedor.
- [ ] tabla de capacidades por modelo.
- [ ] fallback entre proveedores/modelos.
- [ ] health check por proveedor.
- [ ] indicadores de cuota/errores cuando sea posible.
- [ ] interfaz de configuración separada del HUD para cambiar proveedor/modelo.
- [ ] equivalentes CLI para todas las operaciones de configuración.

### Seguridad y gobierno

- [ ] `PolicyEngine` central.
- [ ] niveles `low/medium/high/critical`.
- [ ] confirmación obligatoria para `high`.
- [ ] bloqueo absoluto para `critical`.
- [ ] revalidación post-confirmación.
- [ ] auditoría completa en `action_log`.
- [ ] bloqueo de `.ssh`, `.env`, claves y rutas prohibidas.
- [ ] bloqueo de `rm -rf`, `sudo` destructivo, `curl | sh`.
- [ ] bloqueo de escritura de contraseñas.
- [ ] bloqueo de envíos/pagos sin confirmación fuerte.
- [ ] jobs programados obligados a pasar por policy.
- [ ] workspace incapaz de relajar reglas críticas.
- [ ] skills incapaces de saltarse policy.
- [ ] secretos fuera de cliente/HUD.

### Ejecución y tools

- [ ] `ToolHandler` universal.
- [ ] Tool Registry con enable/disable.
- [ ] Executor con timeout por paso.
- [ ] resultados normalizados.
- [ ] planes secuenciales ejecutables.
- [ ] integración MCP mediante adapters.
- [ ] adapters y tools auditados.
- [ ] tests por familia de tools.
- [ ] errores legibles y recuperables.

### Orquestación y razonamiento

- [ ] normalización de entrada.
- [ ] corrección de transcripción.
- [ ] router con scoring.
- [ ] triggers y negative triggers.
- [ ] slots por dominio.
- [ ] división multi-intent segura.
- [ ] planner con dependencias.
- [ ] recovery de planes.
- [ ] persistencia de confirmaciones.
- [ ] re-ejecución exacta del plan confirmado.
- [ ] thresholds de confianza.
- [ ] fallback LLM conservador.

### Control del escritorio y automatización

- [ ] detector de sesión/compositor.
- [ ] backends Hyprland/Sway/X11/Noop.
- [ ] WindowManager.
- [ ] InputController.
- [ ] control de navegador con reuso.
- [ ] control de YouTube sin duplicar.
- [ ] control de media con `playerctl`.
- [ ] control de volumen.
- [ ] políticas específicas de desktop/input/browser/media.
- [ ] pruebas reales de foco, hotkeys, click y scroll.

### Productividad y tiempo

- [ ] tablas `tasks`, `reminders`, `meetings`, `scheduled_jobs`.
- [ ] parser de tiempo natural en español.
- [ ] RRULE para recurrencias.
- [ ] scheduler persistente.
- [ ] recovery tras reinicio.
- [ ] jobs tardíos y expirados.
- [ ] recordatorio textual separado de acción executable.
- [ ] notificaciones por escritorio, voz, HUD y sonido.
- [ ] `notification_log`.
- [ ] Quiet Hours.
- [ ] skills de productividad.
- [ ] pruebas de recordatorios y reuniones.

### HUD y experiencia visual

- [ ] `rbot-hud` separado del daemon.
- [ ] reconexión automática.
- [ ] estados visuales completos.
- [ ] audio level con smoothing.
- [ ] tarjetas de notificación.
- [ ] tarjetas de confirmación.
- [ ] cola visual.
- [ ] sin ejecución directa de tools desde el HUD.
- [ ] sin panel de configuración pesado dentro del HUD.
- [ ] compatibilidad degradable en Wayland/X11.
- [ ] comandos de prueba para desarrollo visual.

### Workspace, skills y configuración viva

- [ ] workspace auto-creado.
- [ ] watcher de cambios.
- [ ] validator de políticas.
- [ ] memoria visible.
- [ ] shortcuts seguros.
- [ ] schema avanzado de skills.
- [ ] instalación en staging.
- [ ] hash SHA256.
- [ ] estados `disabled/experimental/enabled/trusted/quarantined`.
- [ ] cuarentena por fallos.
- [ ] examples positive/negative.
- [ ] migraciones versionadas.
- [ ] tools de gestión de workspace/skills.
- [ ] panel de settings/control para runtime.
- [ ] recarga en caliente cuando aplique.

### Calidad, pruebas y liberación

- [ ] tests unitarios por paquete.
- [ ] tests de integración daemon ↔ ipc ↔ executor.
- [ ] tests de policy y rutas peligrosas.
- [ ] tests de provider registry.
- [ ] tests de `ListModels` y `SwitchModel`.
- [ ] pruebas manuales guiadas por fase.
- [ ] scripts de smoke test.
- [ ] documentación de instalación.
- [ ] documentación de configuración.
- [ ] documentación de skills/workspace.
- [ ] estrategia de backup para SQLite y workspace.
- [ ] criterios de “release candidate”.
- [ ] changelog y migraciones probadas.

### Criterio de proyecto terminado

- [ ] Puedo instalar RBot y elegir proveedor en el onboarding.
- [ ] Puedo usar Ollama local sin autenticar.
- [ ] Puedo usar OpenAI mediante API key.
- [ ] Puedo usar una superficie tipo Codex/ChatGPT donde corresponda.
- [ ] Puedo listar modelos disponibles del proveedor activo.
- [ ] Puedo cambiar proveedor y modelo desde CLI.
- [ ] Puedo cambiar proveedor y modelo desde una interfaz de configuración.
- [ ] El cambio de proveedor no rompe planner, policy, tools ni HUD.
- [ ] RBot sigue siendo local-first aunque no sea Ollama-only.
- [ ] El sistema completo queda gobernado por políticas, auditado y extensible.