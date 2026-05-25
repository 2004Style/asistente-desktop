---
name: database-prisma-postgres
description: Ayuda segura para PostgreSQL, Prisma, migraciones, seeds, generación de cliente y diagnóstico de conexión.
version: 1.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "prisma generate"
  - "prisma migrate"
  - "corre migraciones"
  - "genera prisma"
  - "revisa postgres"
  - "conecta postgres"
  - "corre seed"
permissions:
  - filesystem:read
  - exec:pnpm
  - exec:npx
  - exec:psql
  - exec:docker
---

# Skill: Database Prisma Postgres

Trabaja con Prisma y PostgreSQL sin exponer secretos.

## Reglas

1. Nunca leer `.env`.
2. Puede leer `.env.example`.
3. Migraciones y seeds requieren confirmación.
4. No ejecutar `prisma migrate reset` sin confirmación crítica.
5. Antes de tocar DB, identificar entorno: local, dev, staging o producción.

## Comandos

```bash
pnpm prisma generate
pnpm prisma migrate dev
pnpm prisma db push
pnpm prisma db seed
pnpm prisma studio
```

Críticos:

```bash
pnpm prisma migrate reset
dropdb
psql -c "DROP DATABASE ..."
```

## Diagnóstico

- Verificar `schema.prisma`.
- Verificar si el cliente Prisma fue generado.
- Verificar errores de tipos.
- Verificar contenedor Postgres si usa Docker.
- No mostrar credenciales.

## Ejemplos

- "genera prisma" → `pnpm prisma generate`.
- "corre migraciones" → explicar y pedir confirmación.
- "revisa por qué no conecta postgres" → diagnóstico sin leer secretos.
