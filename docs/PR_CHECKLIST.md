PR checklist — RBot

Antes de solicitar review, usá esta checklist. Copiala en el cuerpo del PR o pegala en la plantilla.

- [ ] Tests: `go test ./...` pasa en local y en CI.
- [ ] Coverage: las unidades nuevas tienen tests. No exigir coverage mínimo por ahora, pero evitar regresiones.
- [ ] Security: No hay claves o tokens guardados en texto plano en el repo ni en logs.
- [ ] Secrets: Usar `env:VAR` o `keyring:service/name` para referencias de secretos.
- [ ] Backwards compatibility: Revisar CLI existentes (`rbot`, `rbotctl`) y comportamientos legacy.
- [ ] Docs: Actualizar README.md y docs/architecture.md si el cambio es de arquitectura o config.
- [ ] Migration notes: Si elimina paquetes o cambia DB schema, incluir nota de migración (docs/migration_*.md).
- [ ] Review workload: si PR > 400 líneas, dividir en PRs más pequeños por área (config+onboarding vs llm manager vs orchestrator refactor).
- [ ] Release/packaging: revisar scripts de build y systemd unit si aplica.

Solicitud de reviewers sugerida:
- `backend` (core runtime)
- `security` (policy/secrets)
- `docs` (README/architecture)

Instrucciones de verificación (para reviewers):
1. Correr `go test ./...`.
2. Verificar onboarding (interactive or non-interactive):
   - `rbot setup --provider=openai --model=gpt-4o-mini --secret-ref=env:OPENAI_API_KEY --yes`
   - Revisar que `config/providers.yaml` y `config/rbot.yaml` se han escrito correctamente en el directorio de config.
3. Verificar provider list/switch via `rbotctl` with a running daemon (manual):
   - `rbotctl providers list`
   - `rbotctl models list --provider openai`
   - `rbotctl models switch openai gpt-4o-mini`
4. Seguridad: buscar en diffs `api_key` sin `env:` o `keyring:`.
