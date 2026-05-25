---
name: youtube-media-control
description: Reproducción de música y videos en YouTube evitando abrir pestañas duplicadas o activar búsquedas web equivocadas.
version: 3.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "pon música"
  - "pon musica"
  - "ponme música"
  - "ponme musica"
  - "reproduce música"
  - "reproduce musica"
  - "reprodúceme"
  - "reproduceme"
  - "quiero escuchar"
  - "quiero oír"
  - "quiero oir"
  - "pon la canción"
  - "pon la cancion"
  - "toca la canción"
  - "toca la cancion"
  - "ponme algo de"
  - "colócame algo de"
  - "colocame algo de"
  - "música de"
  - "musica de"
permissions:
  - network
  - exec:xdg-open
  - exec:hyprctl
---

# Skill: YouTube Media Control

Reproduce música o videos cuando la intención principal sea escuchar o reproducir.

## Activación

Usar para:

- "pon música"
- "pon música de cumbia"
- "reproduce Numb de Linkin Park"
- "quiero escuchar phonk"
- "ponme algo para programar"
- "toca In The End"

## No activación

No usar para:

- "abre YouTube" → `browser-session-manager`.
- "busca en YouTube tutorial de Docker" → puede ser `browser-session-manager` o `web-research`, no reproducción automática.
- "busca información de una canción" → `web-research`.
- "qué pestañas tengo en YouTube" → `window-workspace-manager`.

## Reglas anti-duplicado

1. Antes de abrir YouTube, revisar ventanas del navegador con `hyprctl clients -j`.
2. Si ya existe YouTube abierto, enfocar o reutilizar cuando la herramienta lo permita.
3. No abrir una nueva pestaña para cada cambio de canción si existe función `browser.youtube_play`.
4. No abrir Google para reproducir música.

## Orquestación

- Música genérica sin género:
  - `browser.youtube_play(query="música para trabajar")`
- Música genérica según gusto del usuario:
  - `browser.youtube_play(query="cumbia o bachata")`
- Tema/artista específico:
  - `browser.youtube_play(query="<tema o artista>")`
- Búsqueda explícita sin autoplay:
  - `browser.youtube_search(query="<consulta>")`

## Respuesta

Confirmar de forma corta, sin leer URLs.

## Ejemplos

- "ponme algo de rock de los 80" → `browser.youtube_play(query="rock de los 80")`.
- "reproduce Numb de Linkin Park" → `browser.youtube_play(query="Numb de Linkin Park")`.
- "abre YouTube" → NO usar esta skill.
