RBot — Runtime local de automatización para Linux

Resumen

RBot es un runtime local-first para automatizar tareas en Linux: escucha por voz/texto, planea, aplica políticas, ejecuta tools auditables y es extensible vía skills y providers LLM. Este repositorio contiene el daemon (rbotd), cliente CLI (rbotctl/rbot), HUD (opt-in) y la plataforma de skills.

Quickstart (desarrollo)

1) Clonar y preparar (dev):

```bash
git clone https://github.com/2004Style/asistente-desktop.git
cd asistente-desktop
# Instalar Go >= 1.20 (según go.mod)
go test ./...           # correr la suite de tests local
```

2) Onboarding (CLI):

Interactivo (wizard):

```bash
# Usa el archivo de config en ./config/rbot.yaml cuando trabajás en repo
./cmd/rbot setup
# o
rbot setup  # si lo instalaste en PATH
```

No-interactivo (scripts / CI / provisioning):

```bash
# Provider OpenAI non-interactive (usa variable de entorno OPENAI_API_KEY)
rbot setup --provider=openai --model=gpt-4o-mini --secret-ref=env:OPENAI_API_KEY --yes

# Provider compatible
rbot setup --provider=my-llm --model=my-model --base-url=https://llm.local --secret-ref=env:LLM_API_KEY --yes
```

3) Usar rbotctl para administración (requiere rbotd corriendo)

```bash
# List providers registered
rbotctl providers list

# Show active provider
rbotctl providers status

# Switch active provider
rbotctl providers use ollama

# List models for provider (provider-scoped)
rbotctl models list --provider ollama

# Switch model or switch provider+model
rbotctl models switch qwen2.5:7b
rbotctl models switch openai gpt-4o-mini
```

Seguridad y secretos

- Nunca se guardan claves en texto plano. Onboarding pide `secret_ref` como `env:NAME` o `keyring:service/name`.
- Implementado por ahora:
  - env:ENV_VAR (por defecto)
  - keyring:service/name (usa go-keyring, requiere que el servicio esté disponible en la máquina)

HUD y build tags

- El HUD GTK está aislado con el build tag `hud`. La suite de CI y `go test ./...` corren un stub por defecto.
- Para validar/compilar el HUD en entornos con GTK dev libs instaladas:

```bash
# Validación HUD (solo en runners preparados)
go test -tags hud ./internal/hud
go build -tags hud -o bin/rbot-hud ./cmd/rbot-hud
```

CI

- El workflow CI ejecuta `go test ./...` como gate principal.
- Tests importantes añadidos: onboarding, provider bootstrap, secrets resolver, manager provider-scoped.

Dónde están las cosas (mapa rápido)

- cmd/ - binarios: rbot, rbotd, rbotctl, rbot-hud (opt-in)
- internal/llm - provider contract, providers (ollama/openai/compatible), registry, manager
- internal/onboarding - onboarding wizard
- internal/secrets - secret resolvers (env, keyring)
- config/rbot.yaml - config principal (dev: config/rbot.yaml)
- config/providers.yaml - providers config (escrito por onboarding)

Desktop GUI (Fyne)

Se añadió una interfaz de escritorio mínima usando Fyne en `cmd/rbot-settings-ui` para configurar proveedor, modelo y secret-ref desde GUI.

Requisitos (Linux):
- Go >= 1.20
- Dependencias del sistema para Fyne (puede variar según distro). En Debian/Ubuntu típicamente:
  - libgl1-mesa-dev libglu1-mesa-dev xorg-dev libx11-dev
  - On Debian/Ubuntu: `sudo apt install libgl1-mesa-dev xorg-dev libx11-dev`
- Para keyring: el paquete `libsecret-1-dev` y configuración si querés usar keyring en Linux.

Cómo ejecutar (dev):

1) Iniciá el daemon en otra terminal (recomendado):

```bash
# desde el repo, o si ya compilaste:
./bin/rbotd &
```

2) Ejecutá el GUI (binario preferido) o con go run:

```bash
# si compilaste:
./bin/rbot-settings-ui
# o en desarrollo:
go run cmd/rbot-settings-ui
```

Smoke script

Hay un script auxiliar `scripts/smoke_gui.sh` que intenta arrancar `rbotd` (si existe en ./bin) y lanzar la UI en foreground. Es útil para pruebas manuales en desktop.

Contribuir

- Hacé cambios en ramas pequeñas y PRs revisables. El repo está configurado para gate de tests.
- Antes de abrir PR: `go test ./...` y documentá el cambio en `docs/`.

Contacto

Si querés que haga el PR, push o cualquier cambio adicional, decime y lo hago.
