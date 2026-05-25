---
name: clipboard-notes
description: Manejo de portapapeles y notas rápidas en Wayland/Linux con wl-clipboard.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "copia esto"
  - "copia al portapapeles"
  - "pega esto"
  - "qué tengo copiado"
  - "guarda una nota"
  - "nota rápida"
permissions:
  - exec:wl-copy
  - exec:wl-paste
  - filesystem:write
---

# Skill: Clipboard Notes

Gestiona portapapeles y notas rápidas.

## Reglas

1. Leer portapapeles puede contener datos sensibles. Confirmar si el usuario no lo pidió explícitamente.
2. No leer contraseñas, tokens o claves si se detectan.
3. Para copiar texto generado, usar `wl-copy`.
4. Para notas, guardar en una ruta configurada o preguntar una vez y recordar.

## Comandos

```bash
wl-paste
printf '%s' "<texto>" | wl-copy
```

## Ejemplos

- "copia esto al portapapeles" → `wl-copy`.
- "qué tengo copiado" → leer, pero filtrar secretos.
- "guarda una nota rápida que diga..." → crear nota.
