# Configuración de Base de Datos y Protocolo MCP

Este documento detalla el backend de base de datos de RBot y la configuración del Model Context Protocol (MCP).

---

## 1. Persistencia de Datos (SQLite)

RBot utiliza una base de datos **SQLite** embebida para almacenar perfiles, histórico de selección de modelos, configuraciones persistentes y caché de descubrimiento de modelos.

### Tablas Principales

1. **`llm_providers`**:
   Almacena la selección del proveedor activo e histórico.
   * `provider_name` (TEXT, PK): Identificador único (ej. `openai`, `google_gemini`).
   * `provider_type` (TEXT): Tipo base del proveedor (`openai`, `ollama`, `compatible`).
   * `base_url` (TEXT): URL base.
   * `api_key_hash` (TEXT): Firma ofuscada de la clave API para visualización segura.
   * `model_id` (TEXT): Identificador del modelo configurado.
   * `active_profile` (TEXT): Perfil de ejecución asociado.
   * `is_active` (INTEGER): `1` si es la selección cargada en caliente, `0` en caso contrario.

2. **`llm_models_cache`**:
   Cachea la lista de modelos de los proveedores para acelerar los tiempos de carga en la UI de ajustes.
   * `provider_name` (TEXT): Proveedor dueño del modelo.
   * `model_id` (TEXT): Identificador técnico del modelo.
   * `model_name` (TEXT): Nombre para visualización en pantalla.
   * `family` (TEXT): Familia del modelo.
   * `size` (TEXT): Tamaño del modelo (útil para Ollama).
   * `tool_calling`, `streaming`, `vision`, `conversation_state` (INTEGER): Atributos booleanos de capacidades del modelo.
   * `cached_at` (TIMESTAMP): Fecha de la última actualización de descubrimiento.

---

## 2. Integración MCP (Model Context Protocol)

El asistente puede comportarse como cliente/servidor de MCP para interactuar con sistemas externos y exponer sus herramientas a modelos compatibles de forma estandarizada.

### Características del Servidor MCP
* **Ubicación de Configuración**: Definida en `config/rbot.yaml` bajo la sección `mcp`.
* **Transporte**: Comunicaciones locales seguras mediante entrada/salida estándar (stdio) o HTTP SSE.
* **Exposición de Herramientas**: Las herramientas del sistema registradas en `internal/tools/` (gestión de archivos, ejecución de comandos, búsquedas) se traducen dinámicamente al formato JSON-RPC de MCP.
* **Seguridad de MCP**: Toda llamada de lectura/escritura de herramientas a través del servidor MCP pasa por el filtro de seguridad de paths e inmutabilidad de políticas de RBot (`internal/security`).
