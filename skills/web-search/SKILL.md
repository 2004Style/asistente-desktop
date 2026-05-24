---
name: web-search
description: Búsquedas de información avanzadas en Internet y Google con capacidad de resumir resultados
version: 2.0.0
author: RBot Premium
risk_level: low
voice_triggers:
  - "busca en internet"
  - "busca en google"
  - "busca información"
  - "busca información sobre"
  - "googlea"
  - "encuentra información sobre"
  - "qué es"
  - "quién es"
  - "investiga sobre"
  - "investiga"
  - "dime sobre"
  - "cuéntame sobre"
  - "información sobre"
  - "biografía de"
  - "historia de"
  - "teoría de"
permissions:
  - exec:xdg-open
  - network
---

# Habilidad Premium: Búsqueda Web Inteligente

Cuando el usuario te pida buscar información, investigar un tema o "googlear" algo.

## REGLA FUNDAMENTAL:
Esta habilidad se activa para **búsquedas de información en internet**. Incluye:
- "busca en internet qué es la física cuántica"
- "busca en google la biografía de Einstein"
- "googlea el clima de hoy"
- "investiga sobre la revolución francesa"
- "dime sobre la teoría de la relatividad"

## REGLA DE RESUMEN:
Cuando el usuario pide **"busca X y hazme/dame un resumen"**, debes:
1. Ejecutar `browser.search(query="<tema>")` para abrir la búsqueda.
2. Responder conversacionalmente con una **explicación breve y natural** del tema basándote en tu conocimiento general.
3. **NO abras más de una pestaña**. Solo la búsqueda principal.
4. **NO ejecutes herramientas adicionales** como `desktop.open_app` — solo `browser.search`.

## Orquestación:
- Extrae el tema o consulta de búsqueda de forma precisa.
- Si el usuario te pide "buscar en internet/google", ejecuta `browser.search(query="<tema>")`.
- Si te hace una pregunta directa, puedes resolverla basándote en tu propia información.

## Respuestas Conversacionales:
- Confirma de forma natural y sin códigos de herramienta.
- **NUNCA** pronuncies enlaces crudos ni URLs.
- Si te pidieron un resumen, ofrece 2-3 oraciones informativas de forma natural.

## Ejemplos de uso:
- "busca en internet cómo hacer una tortilla" -> `browser.search(query="cómo hacer una tortilla")`
- "busca en google la biografía de Einstein y resume" -> `browser.search(query="biografía de Albert Einstein")` + resumen conversacional
- "googlea el clima actual en Barcelona" -> `browser.search(query="clima actual en Barcelona")`
- "investiga sobre el descubrimiento de la penicilina" -> `browser.search(query="descubrimiento de la penicilina")`
