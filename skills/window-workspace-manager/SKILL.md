---
name: window-workspace-manager
description: Control avanzado de ventanas y workspaces en Linux/Hyprland, incluyendo inspección de ventanas del navegador.
version: 2.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "qué ventanas tengo abiertas"
  - "qué tengo abierto"
  - "ventanas abiertas"
  - "pestañas del navegador"
  - "ventanas del navegador"
  - "mueve esta ventana"
  - "cambia de workspace"
  - "manda esta ventana"
  - "cierra esta ventana"
  - "enfoca"
  - "ve al workspace"
permissions:
  - exec:hyprctl
  - exec:jq
---

# Skill: Window Workspace Manager

Controla ventanas, workspaces y estado visual del escritorio en Hyprland.

## Regla crítica

No confundas "ventanas del navegador" con "todas las pestañas". Con `hyprctl` normalmente solo se obtiene el título de la pestaña activa por ventana.

## Consultas útiles

### Todas las ventanas

Ejecutar:

```bash
hyprctl clients -j | jq -r '.[] | "Clase: \(.class)\nTítulo: \(.title)\nWorkspace: \(.workspace.name)\nPID: \(.pid)\n"'
```

### Solo navegadores

Ejecutar:

```bash
hyprctl clients -j | jq -r '.[] | select(.class | test("firefox|brave|chromium|chrome|vivaldi|edge"; "i")) | "Navegador: \(.class)\nTítulo: \(.title)\nWorkspace: \(.workspace.name)\nPID: \(.pid)\n"'
```

## Acciones

- Cerrar ventana activa:
  - `hyprctl dispatch closewindow active`
- Cambiar workspace:
  - `hyprctl dispatch workspace <n>`
- Mover ventana activa:
  - `hyprctl dispatch movetoworkspace <n>`
- Enfocar por clase:
  - `hyprctl dispatch focuswindow class:<class>`

## Confirmaciones

Pedir confirmación para cerrar ventanas si:
- La ventana parece contener trabajo no guardado.
- El título incluye editor, documento, formulario, terminal con proceso o instalación.

## Ejemplos

- "qué ventanas tengo abiertas en el navegador" → listar ventanas browser.
- "mueve esta ventana al workspace 3" → `hyprctl dispatch movetoworkspace 3`.
- "cierra esta ventana" → cerrar activa, salvo riesgo evidente.
