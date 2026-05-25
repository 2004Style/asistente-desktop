---
name: clean-hexagonal-cli
description: Skill especializada para usar una CLI de arquitectura limpia/hexagonal, generar estructura, módulos, casos de uso y documentación.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "crea arquitectura limpia"
  - "crea arquitectura hexagonal"
  - "genera módulo"
  - "genera caso de uso"
  - "usa la cli"
  - "clean hexagonal"
  - "hexagonal cli"
permissions:
  - filesystem:read
  - filesystem:write
  - exec:node
  - exec:pnpm
  - exec:npm
  - exec:go
---

# Skill: Clean Hexagonal CLI

Especializada en herramientas CLI para crear estructuras de arquitectura limpia/hexagonal y reducir consumo de tokens.

## Reglas

1. Antes de generar, detectar lenguaje/framework:
   - NestJS
   - Go
   - Java
   - C#
   - Python
   - Otros si la CLI lo soporta.
2. Preguntar solo datos esenciales:
   - Nombre de módulo.
   - Entidad principal.
   - Framework/lenguaje si no se detecta.
3. No sobrescribir carpetas existentes sin confirmación.
4. Respetar convenciones del proyecto actual.
5. Después de generar, mostrar árbol creado y siguientes pasos.

## Convenciones sugeridas

### NestJS / TypeScript

- `core/domain/entities`
- `core/domain/repositories`
- `core/application/ports/in`
- `core/application/ports/out`
- `core/application/use-cases`
- `infrastructure/adapters`
- `presentation/controllers`

### Go

- `internal/domain`
- `internal/application`
- `internal/ports`
- `internal/adapters`
- `cmd`

## Ejemplos

- "genera un módulo products con arquitectura hexagonal" → detectar proyecto y llamar CLI.
- "crea un caso de uso para registrar usuario" → generar use case + ports.
- "estructura limpia para NestJS" → usar convención NestJS.
