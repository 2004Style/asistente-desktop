---
name: screen-capture-helper
description: Capturas de pantalla en Wayland/Hyprland para análisis visual, pruebas del escritorio y documentación.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "toma captura"
  - "captura la pantalla"
  - "screenshot"
  - "mira mi pantalla"
  - "analiza la pantalla"
  - "captura esta ventana"
permissions:
  - exec:grim
  - exec:slurp
  - exec:hyprctl
  - filesystem:write
---

# Skill: Screen Capture Helper

Toma capturas de pantalla o ventanas en Wayland/Hyprland.

## Reglas

1. Pedir confirmación si la captura puede contener datos sensibles.
2. Guardar capturas en una carpeta segura como `~/Pictures/RBot`.
3. No subir imágenes a Internet salvo que el usuario lo pida y confirme.
4. Para análisis visual local, usar herramienta disponible del agente si existe.

## Comandos

Pantalla completa:

```bash
grim ~/Pictures/RBot/captura-$(date +%Y%m%d-%H%M%S).png
```

Región:

```bash
grim -g "$(slurp)" ~/Pictures/RBot/captura-$(date +%Y%m%d-%H%M%S).png
```

Ventanas:

```bash
hyprctl activewindow -j
```

## Ejemplos

- "captura la pantalla" → grim.
- "captura una región" → grim + slurp.
- "qué ventana está activa" → hyprctl activewindow.
