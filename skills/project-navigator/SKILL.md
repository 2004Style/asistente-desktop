---
name: project-navigator
description: Abrir, ubicar y resumir proyectos locales conocidos usando memoria, índice de archivos y VS Code.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "abre mi proyecto"
  - "abre el proyecto"
  - "dónde está mi proyecto"
  - "resume este proyecto"
  - "qué stack usa"
  - "analiza la estructura"
  - "abre convertsystems"
  - "abre rbot"
permissions:
  - filesystem:read
  - memory:read
  - exec:code
  - exec:find
---

# Skill: Project Navigator

Ayuda a moverse por proyectos locales.

## Reglas

1. Buscar primero en memoria local.
2. Si no existe alias, buscar por carpetas en rutas permitidas.
3. Confirmar si hay varias coincidencias.
4. Para abrir en VS Code, usar `code <ruta>` solo si la ruta fue resuelta.
5. Para resumir, leer estructura sin entrar a carpetas pesadas:
   - `node_modules`
   - `.git`
   - `dist`
   - `build`
   - `.next`
   - `target`
   - `bin`

## Resumen del proyecto

Incluir:

- Stack detectado.
- Carpetas importantes.
- Scripts disponibles.
- Cómo ejecutar.
- Riesgos o problemas visibles.

## Ejemplos

- "abre mi proyecto RBot" → memoria → `code <ruta>`.
- "resume este proyecto" → árbol reducido + stack.
- "qué stack usa Convertsystems" → revisar package.json, go.mod, docker files, etc.
