---
name: system-control
description: Control seguro de volumen, brillo, suspensión, apagado, reinicio, bloqueo de pantalla y acciones mecánicas del sistema Linux.
version: 2.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "sube el volumen"
  - "baja el volumen"
  - "silencia"
  - "quita el silencio"
  - "sube el brillo"
  - "baja el brillo"
  - "bloquea la pantalla"
  - "suspende"
  - "apaga la computadora"
  - "reinicia la computadora"
permissions:
  - exec:amixer
  - exec:pactl
  - exec:brightnessctl
  - exec:loginctl
  - exec:systemctl
---

# Skill: System Control

Controla acciones del sistema operativo.

## Audio

Preferir PipeWire/PulseAudio si está disponible:

```bash
pactl set-sink-volume @DEFAULT_SINK@ +10%
pactl set-sink-volume @DEFAULT_SINK@ -10%
pactl set-sink-mute @DEFAULT_SINK@ toggle
```

Fallback:

```bash
amixer set Master 10%+
amixer set Master 10%-
amixer set Master toggle
```

## Brillo

```bash
brightnessctl set +10%
brightnessctl set 10%-
```

## Energía

Requieren confirmación:

```bash
systemctl poweroff
systemctl reboot
systemctl suspend
```

Bloquear pantalla puede ejecutarse si hay comando disponible:

```bash
loginctl lock-session
```

## Reglas

1. Apagar/reiniciar/suspender siempre requiere confirmación.
2. Para volumen y brillo, ejecutar directamente.
3. No usar `sudo` salvo que el sistema lo exija y el usuario confirme.
