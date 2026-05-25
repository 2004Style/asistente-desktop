---
name: linux-diagnostics
description: Diagnóstico de Linux/Arch/Hyprland: GPU NVIDIA, audio, red, disco, memoria, servicios, logs y rendimiento.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "diagnostica"
  - "revisa mi sistema"
  - "qué está fallando"
  - "por qué no funciona"
  - "revisa nvidia"
  - "revisa el audio"
  - "revisa internet"
  - "revisa docker"
  - "mira los logs"
permissions:
  - exec:uname
  - exec:lsmod
  - exec:lspci
  - exec:nvidia-smi
  - exec:journalctl
  - exec:systemctl
  - exec:ip
  - exec:df
  - exec:free
---

# Skill: Linux Diagnostics

Diagnostica problemas del sistema sin modificar configuración.

## Reglas

1. Primero recopilar evidencia.
2. No instalar paquetes ni editar archivos.
3. Si se detecta una solución, explicar y pedir confirmación antes de modificar.
4. Para problemas recurrentes, guardar diagnóstico útil en memoria si el usuario lo permite.

## Comandos comunes

### Sistema

```bash
uname -a
cat /etc/os-release
uptime
free -h
df -h
```

### GPU NVIDIA

```bash
nvidia-smi
pacman -Q | grep -E "nvidia|linux"
lsmod | grep nvidia
lspci -k | grep -A 3 -E "VGA|3D"
journalctl -b -p err --no-pager | grep -i nvidia
```

### Hyprland

```bash
hyprctl version
hyprctl monitors
hyprctl clients -j
```

### Audio

```bash
pactl info
pactl list short sinks
pactl list short sources
```

### Red

```bash
ip a
ip route
resolvectl status
ping -c 3 1.1.1.1
```

## Respuesta

Dar diagnóstico por capas:

- Síntoma observado.
- Evidencia.
- Causa probable.
- Acción recomendada.
- Comando de corrección, si el usuario confirma.
