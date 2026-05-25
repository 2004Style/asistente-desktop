---
name: router-core
description: Núcleo de enrutamiento para decidir qué habilidad debe activarse, evitar conflictos y resolver órdenes ambiguas.
version: 1.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "ronald"
  - "rbot"
  - "oye ronald"
  - "ey ronald"
  - "go ronald"
permissions: []
---

# Skill: Router Core

Esta habilidad no ejecuta acciones finales. Sirve para ordenar el razonamiento del agente antes de llamar a otras herramientas.

## Objetivo

Evitar que RBot abra aplicaciones, pestañas, búsquedas o comandos incorrectos cuando la orden del usuario pueda interpretarse de varias formas.

## Reglas Globales de Enrutamiento

1. **Primero clasifica la intención**
   - Información web → `web-research`.
   - Música o reproducción en YouTube → `youtube-media-control`.
   - Abrir, cerrar, mover o enfocar ventanas → `window-workspace-manager`.
   - Archivos locales → `file-reader-search` o `file-writer-safe`.
   - Aplicaciones instaladas → `app-launcher`.
   - Desarrollo → `developer-workflow`, `git-guardian`, `node-nextjs-helper`, `go-rbot-helper`, etc.
   - Diagnóstico del sistema → `linux-diagnostics`.
   - Memoria o alias personales → `memory-manager`.

2. **No uses una skill solo porque una palabra coincide**
   - "busca el archivo factura.pdf" NO es web: es `file-reader-search`.
   - "busca en YouTube Numb" NO es web general: es `youtube-media-control`.
   - "qué ventanas tengo abiertas" NO es web: es `window-workspace-manager`.
   - "abre YouTube" NO es música necesariamente: es `browser-session-manager`.

3. **Una orden = una acción principal**
   - Si el usuario pide "pon música", no abras Google y luego YouTube.
   - Si el usuario pide "lee un archivo", no abras VS Code ni navegador.
   - Si el usuario pide "busca en internet", no abras 5 pestañas.

4. **Si ya existe una ventana o pestaña útil, reutilízala**
   - Primero inspecciona ventanas abiertas si la acción puede duplicar navegador, YouTube, música o VS Code.
   - Evita abrir otra pestaña si puedes enfocar o reutilizar una existente.

5. **Prioridad de skills ante conflicto**
   - Seguridad: `security-guard`
   - Rutas y archivos: `file-reader-search`, `file-writer-safe`
   - Ventanas y navegador: `window-workspace-manager`, `browser-session-manager`
   - Música: `youtube-media-control`
   - Web general: `web-research`
   - Desarrollo: skills de desarrollo
   - Diagnóstico: skills de sistema

## Manejo de ambigüedad

Pregunta solo si la acción puede ser dañina o si faltan datos esenciales. Si la acción es segura, haz la mejor inferencia y actúa.

## Ejemplos

- "qué tengo abierto en el navegador" → usar `window-workspace-manager`.
- "ponme cumbia" → usar `youtube-media-control`.
- "busca en internet qué es NATS" → usar `web-research`.
- "busca mi archivo tesis.docx" → usar `file-reader-search`.
- "abre mi proyecto Convertsystems" → usar `project-navigator`.
