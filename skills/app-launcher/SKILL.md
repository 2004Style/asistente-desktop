---
name: app-launcher
description: Abrir, enfocar y resolver aplicaciones instaladas desde .desktop, alias y base de datos local.
version: 1.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "abre"
  - "ejecuta"
  - "lanza"
  - "inicia"
  - "abre vscode"
  - "abre visual studio code"
  - "abre terminal"
  - "abre brave"
  - "abre firefox"
permissions:
  - apps:read
  - exec:gtk-launch
  - exec:xdg-open
  - exec:hyprctl
---

# Skill: App Launcher

Abre aplicaciones locales, priorizando enfocar una instancia existente antes de lanzar otra.

## Reglas

1. Si la aplicación ya está abierta, enfocar su ventana cuando tenga sentido.
2. Si el usuario pide una nueva instancia, lanzar otra instancia.
3. Resolver nombres naturales:
   - "código" → VS Code.
   - "navegador" → navegador preferido.
   - "terminal" → kitty.
   - "archivos" → gestor de archivos.
4. Usar el índice de `.desktop` antes de comandos manuales.
5. No abrir navegador para tareas de archivo local.

## Herramientas

- `apps.search(query="<nombre>")`
- `apps.launch(desktop_id="<id>")`
- `system.run_command(command="gtk-launch <desktop-id>")`
- `system.run_command(command="hyprctl dispatch focuswindow class:<class>")`

## Ejemplos

- "abre VS Code" → enfocar si existe; si no, lanzar.
- "abre mi navegador" → abrir navegador preferido.
- "abre la terminal" → lanzar kitty.
