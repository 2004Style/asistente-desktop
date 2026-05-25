---
name: node-nextjs-helper
description: Ayuda especializada para Node, pnpm, Next.js, React, NestJS, Turbopack, Prisma y errores de build.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "error de next"
  - "error de react"
  - "error de pnpm"
  - "error de npm"
  - "error de prisma"
  - "corre next"
  - "levanta next"
  - "levanta nest"
  - "build de next"
permissions:
  - filesystem:read
  - exec:pnpm
  - exec:npm
  - exec:node
  - exec:npx
---

# Skill: Node NextJS Helper

Especializada en proyectos frontend/backend JavaScript/TypeScript.

## Detección

Leer:

- `package.json`
- `pnpm-lock.yaml`
- `next.config.*`
- `tsconfig.json`
- `prisma/schema.prisma`
- `.env.example` si existe, nunca `.env`.

## Reglas

1. Si hay `pnpm-lock.yaml`, usar `pnpm`.
2. No usar `npm install` en proyectos pnpm.
3. No leer `.env`.
4. No ejecutar migraciones Prisma sin confirmación.
5. Para errores de build, ejecutar comando y resumir el primer error real, no todo el log.

## Comandos útiles

```bash
pnpm dev
pnpm build
pnpm lint
pnpm test
pnpm prisma generate
pnpm prisma migrate dev
```

`migrate dev` requiere confirmación.

## Diagnóstico común

- Múltiples lockfiles.
- Root directory incorrecto.
- Tipos de Node faltantes.
- Variables de entorno ausentes.
- Prisma client no generado.
- Next/Turbopack workspace root warning.
- Componentes React usados fuera de su provider/contexto.

## Ejemplos

- "arregla el error de Next" → leer logs + package.json + sugerir.
- "corre build" → `pnpm build` si corresponde.
- "genera prisma" → `pnpm prisma generate`.
