---
name: go-rbot-helper
description: Skill especializada para RBot en Go: compilar, testear, revisar módulos, MCP, skills, SQLite y voz.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "compila rbot"
  - "testea rbot"
  - "corre pruebas de rbot"
  - "revisa el orquestador"
  - "escanea skills"
  - "prueba el asistente"
permissions:
  - filesystem:read
  - exec:go
  - exec:make
  - exec:bash
---

# Skill: Go RBot Helper

Especializada para el propio proyecto RBot escrito en Go.

## Comandos principales

```bash
go test ./...
go test -count=1 -v ./...
go fmt ./...
go vet ./...
go build -o bin/rbot cmd/main.go
./bin/rbot skills scan
./bin/rbot index paths
./bin/rbot index apps
```

## Reglas

1. Antes de compilar, verificar si estás en la raíz del proyecto.
2. Para pruebas de voz, no asumir hardware disponible.
3. Para tests del orquestador, preferir mocks.
4. Para skills nuevas, correr `./bin/rbot skills scan`.
5. Si falla el build, mostrar archivo, línea y causa probable.

## Áreas a revisar en RBot

- `internal/agent/orchestrator.go`
- `internal/skills/manager.go`
- `internal/security/permissions.go`
- `internal/files/finder.go`
- `internal/db/sqlite.go`
- `internal/mcp/client.go`
- `internal/voice/audio.go`

## Ejemplos

- "compila RBot" → build.
- "corre todas las pruebas de RBot" → go test.
- "escanea las skills" → skills scan.
