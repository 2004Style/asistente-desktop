---
name: web-research
description: Investigación web, lectura de URLs, resumen de páginas y búsquedas informativas sin crear pestañas innecesarias.
version: 3.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "busca en internet"
  - "busca en google"
  - "investiga en internet"
  - "googlea"
  - "busca información sobre"
  - "lee esta página"
  - "resume esta página"
  - "resume este sitio"
  - "qué es en internet"
  - "quién es en internet"
permissions:
  - network
  - exec:xdg-open
---

# Skill: Web Research

Investiga información pública en Internet y resume contenido.

## Mejoras sobre la versión anterior

No usar triggers demasiado amplios como "qué es", "quién es", "dime sobre" o "cuéntame sobre" porque generan falsos positivos. Esas frases pueden ser respondidas por el modelo o por otra skill.

## Activación correcta

Usar cuando el usuario explícitamente pida:

- Buscar en Internet.
- Googlear.
- Investigar información actual.
- Leer o resumir una URL.
- Buscar noticias, documentación, precios, versiones o información cambiante.

## No activación

No usar cuando:

- El usuario busca un archivo local.
- El usuario quiere reproducir música.
- El usuario solo quiere abrir una página.
- El usuario pide explicación general y no requiere Internet.

## Orquestación

1. Si hay URL explícita:
   - Usar `browser.read_url(url="<url>")`.
   - Resumir contenido real.
2. Si hay búsqueda:
   - Usar `browser.search(query="<consulta>")`.
   - No abrir más de una pestaña.
3. Si el usuario pide "busca y resume":
   - Ejecutar búsqueda.
   - Dar resumen breve con advertencia si no se leyó una fuente concreta.
4. Para información actual, preferir lectura real de páginas cuando exista herramienta.

## Ejemplos

- "busca en internet NATS vs MQTT" → `browser.search(query="NATS vs MQTT")`.
- "lee convertsystems.site y resume" → `browser.read_url(url="https://convertsystems.site")`.
- "googlea clima de Cajamarca" → búsqueda web.
