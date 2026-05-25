---
name: browser-session-manager
description: Manejo de navegador evitando duplicar pestañas, abriendo URLs, enfocando ventanas y separando búsquedas de reproducción.
version: 1.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "abre el navegador"
  - "abre youtube"
  - "abre google"
  - "abre whatsapp web"
  - "abre una página"
  - "abre este link"
  - "abre esta url"
  - "ve a"
permissions:
  - exec:xdg-open
  - exec:hyprctl
  - network
---

# Skill: Browser Session Manager

Abre o enfoca sitios web sin confundirlos con búsquedas web ni música.

## Activación

Usar para:

- "abre YouTube"
- "abre WhatsApp Web"
- "abre convertsystems.site"
- "abre mi navegador"
- "abre este link"

## No activación

No usar para:

- "busca información sobre X" → `web-research`.
- "pon música de X" → `youtube-media-control`.
- "lee este archivo local" → `file-reader-search`.

## Reglas

1. Si el navegador ya está abierto, intenta enfocarlo antes de abrir otra instancia.
2. Para una URL explícita, usar `browser.open_url(url="<url>")` o `xdg-open`.
3. Para páginas comunes:
   - YouTube → `https://www.youtube.com`
   - WhatsApp Web → `https://web.whatsapp.com`
   - Gmail → `https://mail.google.com`
4. No abrir más de una pestaña por orden.
5. Si el usuario dice "solo abre", no busques ni resumas.

## Ejemplos

- "abre YouTube" → abrir YouTube, no reproducir música.
- "abre google" → abrir Google.
- "abre whatsapp web" → abrir WhatsApp Web.
