---
name: memory-manager
description: Manejo de memoria local SQLite/FTS5 para recordar rutas, alias, preferencias técnicas, proyectos y ejecuciones útiles.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "recuerda que"
  - "guarda que"
  - "memoriza"
  - "olvida que"
  - "dónde estaba"
  - "guarda esta ruta"
  - "recuerda esta carpeta"
permissions:
  - memory:read
  - memory:write
  - db:sqlite
---

# Skill: Memory Manager

Gestiona memoria local persistente para evitar búsquedas repetidas y mejorar el flujo del usuario.

## Qué guardar

Guardar solo información útil para futuras acciones:

- Alias de rutas: "mi proyecto RBot está en ~/Proyectos/asistente".
- Preferencias técnicas: "uso pnpm en proyectos Next.js".
- Aplicaciones preferidas: navegador por defecto, editor, terminal.
- Comandos verificados: scripts de test, build y arranque.
- Rutas encontradas tras búsquedas costosas.
- Últimos proyectos abiertos.

## Qué no guardar automáticamente

- Secretos, tokens, claves, contraseñas.
- Datos íntimos no necesarios.
- Conversaciones privadas.
- Información temporal que no servirá después.

## Reglas

1. Si el usuario dice "recuerda", usar `memory.save`.
2. Si el usuario pide una ruta ya conocida, consultar `memory.search`.
3. Si una ruta guardada ya no existe, buscar de nuevo y actualizar el alias.
4. Si hay varias coincidencias, mostrar las 3 mejores y pedir selección solo si ejecutar mal sería riesgoso.
5. Para datos sensibles, preguntar antes o rechazar si es secreto.

## Ejemplos

- "recuerda que mi proyecto Convertsystems está en ~/Proyectos/convertsystems" → guardar alias.
- "abre mi proyecto de RBot" → buscar alias y abrir con VS Code.
- "olvida la ruta de mi proyecto antiguo" → eliminar memoria asociada.
