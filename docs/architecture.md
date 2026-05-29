# Arquitectura - Resumen de cambios recientes

Este documento resume las decisiones arquitectónicas y los cambios aplicados en Sprint 1/2 relativos a proveedores LLM, onboarding, secretos y consolidación de código, incluyendo la nueva arquitectura orientada a capacidades declarativas.

## Eje: proveedores LLM (Capacidades Declarativas y Single Source of Truth)
- Se consolidó la lógica de proveedores en `internal/llm/`:
  - `internal/llm/provider.go` (contrato `Provider` e interfaz orientada a capacidades `ProviderCapability`)
  - `internal/llm/adapter.go` (adaptador genérico `ProviderAdapter` para encapsular la ejecución sin condicionales cableados en el núcleo)
  - `internal/llm/oauth.go` (flujo OAuth 2.0 PKCE con callback local)
  - `internal/llm/ollama`, `internal/llm/openai`, `internal/llm/compatible` (proveedores especializados y flexibles)
  - `internal/llm/registry.go` y `internal/llm/manager.go` para registro, selección del activo y persistencia ligera

### Diseño Basado en Capacidades
La arquitectura separa estrictamente la **autenticación** de la **facturación** y el **runtime** mediante tres dimensiones:
1. **`auth_mode`**: Define el método de inicio de sesión (`browser_oauth`, `api_key`, `adc`, `service_account`, `none`).
2. **`billing_mode`**: Especifica de dónde proviene el consumo asociado (`subscription` para cuentas plus/pro, `pay_as_you_go` por tokens consumidos, `credits` prepagados, `cloud_project` facturado a un proyecto de nube, `local` sin coste externo).
3. **`runtime_mode`**: Determina cómo se gestiona la sesión de ejecución (`official_cli_session` vía CLI oficial, `direct_api` consumo directo de endpoints, `gateway_api` ruteado por proxy/gateway como OpenRouter, `local_runtime` ejecución local por Ollama).

Gracias a esto, cualquier nuevo proveedor (ej. DeepSeek, Mistral, Groq, xAI, etc.) se puede añadir registrando su descriptor y sus capacidades en `providers.yaml` sin modificar la lógica interna del asistente.

## Configuración y bootstrap
- Nuevo esquema extendido en `config/providers.yaml` que soporta mapas de `auth_modes`, `billing_modes`, `runtime_modes` y el array de `capabilities` de cada proveedor.
- El bootstrap en `internal/llm/bootstrap/bootstrap.go` resuelve dinámicamente qué motor instanciar (`ollama` para `local_runtime` o `compatible` para endpoints en la nube) y mapea las cabeceras personalizadas (ej. `x-api-key` para Claude/Anthropic) leyendo la capacidad activa seleccionada.

## Flujo de Autenticación OAuth 2.0 PKCE (`browser_oauth`)
Cuando el usuario selecciona el método de autenticación por navegador:
1. El motor genera una semilla aleatoria para `state` (previene CSRF) y un `code_verifier` de 32 bytes codificado en Base64 seguro, derivando su firma `code_challenge` (S256).
2. Se inicia un servidor HTTP de callback temporal en `127.0.0.1:8085` (o puerto dinámico en caso de conflicto) que escucha la petición en `/auth/callback`.
3. Se abre el navegador por defecto del sistema y redirige al usuario a la página de autorización oficial del proveedor.
4. Tras validarse con éxito en el proveedor, el callback local recibe la autorización, valida el parámetro `state` de vuelta, intercambia el código por un token de sesión, muestra una página HTML de éxito moderna al usuario, y almacena el token de forma segura.

## Secret management
- Módulo `internal/secrets` con resolvers por esquema:
  - `env:` (env var) — siempre disponible.
  - `keyring:` (keyring resolver) — utiliza el llavero nativo del sistema (D-Bus/Secret Service) para almacenar y recuperar de forma segura las credenciales.
  - `plain:` (plain fallback) — en caso de fallos del llavero de sistema (ej. entorno headless sin sesión D-Bus activa), se realiza fallback guardando la referencia de forma local con permisos restringidos.

## HUD/GTK
- HUD GTK (gotk3) se aisló con build tag `hud`. En builds normales/CI se compila un *stub* que no requiere librerías nativas.
- Para compilar/validar el HUD: `go test -tags hud ./internal/hud && go build -tags hud ./cmd/rbot-hud`.

## Migración y limpieza
- Se removió el antiguo paquete `internal/ollama/` y la lógica inline de proveedores nulos. Todo se gestiona mediante el contrato general de `ProviderAdapter`.

## Tests y CI
- CI (`.github/workflows/ci.yml`) corre `go test ./...` — gate principal.
- Las pruebas se han adaptado para dar cobertura a la serialización del esquema de capacidades y la resolución de tokens PKCE simulados.
