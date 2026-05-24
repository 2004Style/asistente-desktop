---
name: system-control
description: Habilidad avanzada para el control de volumen, brillo, energía y ventanas del sistema operativo Linux
version: 1.0.0
author: RBot Premium
risk_level: medium
voice_triggers:
  - "sube el volumen"
  - "baja el volumen"
  - "silencia el audio"
  - "apaga la pantalla"
  - "reinicia la computadora"
  - "apaga el equipo"
  - "cierra la ventana"
permissions:
  - exec:amixer
  - exec:brightnessctl
  - exec:hyprctl
  - exec:systemctl
---

# Habilidad Premium: Control de Sistema

Esta habilidad permite controlar aspectos de hardware y ventanas del entorno de escritorio del usuario.

## Reglas de Ejecución:
1. **Volumen de Audio**:
   - Para subir el volumen: Ejecuta `system.run_command(command="amixer set Master 10%+")`.
   - Para bajar el volumen: Ejecuta `system.run_command(command="amixer set Master 10%-")`.
   - Para silenciar: Ejecuta `system.run_command(command="amixer set Master toggle")`.
2. **Control de Ventanas (Hyprland)**:
   - Para cerrar la ventana activa o enfocada: Ejecuta `system.run_command(command="hyprctl dispatch closewindow active")`.
3. **Energía**:
   - Para apagar el equipo (acción crítica): Pide confirmación y ejecuta `system.run_command(command="systemctl poweroff")`.
   - Para reiniciar el equipo (acción crítica): Pide confirmación y ejecuta `system.run_command(command="systemctl reboot")`.

## Ejemplos de uso:
- "sube el volumen por favor" -> Llama a system.run_command(command="amixer set Master 10%+")
- "cierra esta ventana" -> Llama a system.run_command(command="hyprctl dispatch closewindow active")
