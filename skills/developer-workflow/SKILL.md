---
name: developer-workflow
description: Flujo general de desarrollo: detectar stack, correr tests, compilar, formatear, lint, dev server y scripts del proyecto.
version: 3.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "corre los test"
  - "ejecuta los test"
  - "compila el proyecto"
  - "levanta el proyecto"
  - "inicia el proyecto"
  - "corre el servidor"
  - "formatea el código"
  - "corre lint"
  - "arregla formato"
permissions:
  - filesystem:read
  - exec:go
  - exec:npm
  - exec:pnpm
  - exec:yarn
  - exec:dotnet
  - exec:make
  - exec:docker
---

# Skill: Developer Workflow

Gestiona tareas de desarrollo sin asumir el stack a ciegas.

## Detección de stack

Antes de ejecutar:

- `package.json` → Node/Next/Nest/React.
- `pnpm-lock.yaml` → usar `pnpm`.
- `package-lock.json` → usar `npm`.
- `yarn.lock` → usar `yarn`.
- `go.mod` → Go.
- `*.csproj` o `*.sln` → .NET.
- `docker-compose.yml` o `compose.yml` → Docker Compose.
- `Makefile` → revisar targets.

## Reglas

1. Detectar raíz del proyecto.
2. Leer scripts antes de ejecutar.
3. No correr migraciones, deploys ni comandos destructivos sin confirmación.
4. Si hay tests pesados, avisar y ejecutar si el usuario pidió explícitamente.
5. Si el comando falla, resumir error y sugerir siguiente paso.

## Comandos por stack

### Go

```bash
go test ./...
go test -count=1 ./...
go fmt ./...
go build ./...
```

### Node/pnpm

```bash
pnpm install
pnpm dev
pnpm build
pnpm test
pnpm lint
pnpm format
```

### npm

```bash
npm run dev
npm run build
npm test
npm run lint
```

### .NET

```bash
dotnet build
dotnet test
dotnet run
```

## Ejemplos

- "corre los test" → detectar stack y ejecutar comando adecuado.
- "compila este proyecto" → ejecutar build según stack.
- "levanta el backend" → leer scripts y ejecutar dev/start correcto.
