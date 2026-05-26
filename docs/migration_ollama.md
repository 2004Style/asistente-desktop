# Notas de migración: eliminación de internal/ollama

Resumen

Se eliminó el paquete legacy `internal/ollama/` y se consolidó su funcionalidad en `internal/llm/ollama/` y en el bootstrap del registry de proveedores. Esta nota ayuda a reviewers y mantenedores a entender la transición.

Qué se movió
- Todo el cliente HTTP y adaptadores se mantuvieron en `internal/llm/ollama`.
- El antiguo `internal/ollama/client.go` se eliminó tras verificar que no había imports.

Puntos a revisar en PR
- Asegurarse de que no existan import paths remotos a `internal/ollama` en otros repos o scripts.
- Verificar que las migraciones de DB incluyen `llm_providers` y `llm_models_cache` (ya añadidas en migrations.go).
- Verificar comportamiento de fallback: si no hay providers habilitados, el bootstrap registra el provider legacy configurado en `rbot.yaml` como fallback.

Rollback
- Si por alguna razón es necesario revertir, la rama `rbot/migrate/ollama-remove` contiene el commit. Revertir ese commit restaura el paquete eliminado.

Notas operativas
- Cambios probados: `go test ./...` y `go test -cover ./...` pasaron en entorno de desarrollo.
- CI ejecuta `go test ./...` como gate principal.

