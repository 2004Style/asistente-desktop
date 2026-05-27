# Arquitectura - Resumen de cambios recientes

Este documento resume las decisiones arquitectónicas y los cambios aplicados en Sprint 1/2 relativos a proveedores LLM, onboarding, secretos y consolidación de código.

## Eje: proveedores LLM (single source of truth)
- Se consolidó la lógica de proveedores en `internal/llm/`:
  - `internal/llm/provider.go` (contrato Provider)
  - `internal/llm/ollama`, `internal/llm/openai`, `internal/llm/compatible`
  - `internal/llm/registry.go` y `internal/llm/manager.go` para registro, selección del activo y persistencia ligera
- Se eliminó el paquete `internal/ollama/`. Todos los llamadores usan ahora `internal/llm`.

## Configuración y bootstrap
- Nuevo archivo `config/providers.yaml` con estructura `ProvidersConfig` (enabled, type, auth_mode, base_url, secret_ref, model).
- `internal/llm/bootstrap` construye el `Registry` a partir de `providers.yaml`. Si no hay proveedores habilitados, devuelve error explícito.
- El daemon (`rbotd`) y el CLI (`rbot`) consumen el registry construido por el bootstrap.

## Onboarding (rbot setup / onboard)
- Nuevo comando: `rbot setup` / `rbot onboard`.
- Dos modos:
  - interactivo: wizard por consola
  - no-interactivo: `--provider`, `--model`, `--base-url`, `--secret-ref`, `--yes` (acepta defaults)
- El onboarding escribe `config/providers.yaml` junto a `rbot.yaml` (por defecto en el mismo directorio de config) sin almacenar claves en claro.

## Secret management
- Nuevo package `internal/secrets` con resolvers por esquema:
  - `env:` (env var) — siempre disponible
  - `keyring:` (keyring resolver) — utiliza `github.com/zalando/go-keyring` para obtener secretos del almacén del sistema
- El bootstrap usa `secret_ref` (ej. `env:OPENAI_API_KEY` o `rbot/OPENAI_KEY` en keyring) para resolver claves sin persistirlas.

## Seguridad y políticas
- `internal/policy` y `internal/security` mantienen reglas inmutables y bloqueo de paths sensibles (expandido para `.ssh`, `.aws`, `.config/gh`, cookies, `.pem`/`.key`, etc.).
- Por seguridad, `skills.remote_install_enabled` se deshabilita por defecto mientras se perfecciona el pipeline de validación.

## HUD/GTK
- HUD GTK (gotk3) se aisló con build tag `hud`. En builds normales/CI se compila un *stub* que no requiere librerías nativas.
- Para compilar/validar el HUD: `go test -tags hud ./internal/hud && go build -tags hud ./cmd/rbot-hud`.

## Migración y limpieza
- `internal/ollama` fue eliminado tras migrar consumidores a `internal/llm/ollama`.
- Si hay integraciones externas que usen el paquete eliminado (poco probable), revisar import paths.

## Tests y CI
- CI (`.github/workflows/ci.yml`) corre `go test ./...` — gate principal.
- Las pruebas nuevas cubren onboarding, provider bootstrap, secrets resolver y manager provider-scoped operations.

## Recomendaciones futuras
- Implementar backend de secretos más rico (keyring + OS secret managers) y documentar las trade-offs.
- Implementar validación y staging para `skills` remotas antes de habilitar `remote_install_enabled` por defecto.
- Añadir una validación opcional HUD job en CI (tag `hud`) en runners preparados con GTK.
- Agregar PRs más pequeños para cada área de refactor grande (orchestrator, tools/<...>) para reducir carga de revisión.

