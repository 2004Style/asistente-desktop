---
name: arch-package-manager
description: Gestión segura de paquetes en Arch Linux con pacman/yay, búsquedas, instalación, actualización y limpieza con confirmación.
version: 1.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "instala"
  - "desinstala"
  - "actualiza arch"
  - "actualiza el sistema"
  - "busca paquete"
  - "qué paquete instala"
  - "limpia paquetes"
permissions:
  - exec:pacman
  - exec:yay
---

# Skill: Arch Package Manager

Gestiona paquetes en Arch Linux.

## Reglas de seguridad

1. Buscar paquetes no requiere confirmación.
2. Instalar requiere confirmación si usa `sudo`.
3. Desinstalar siempre requiere confirmación.
4. Actualizar todo el sistema requiere confirmación.
5. Nunca ejecutar `pacman -Rns` sin mostrar paquetes objetivo.

## Comandos

### Buscar

```bash
pacman -Ss <paquete>
yay -Ss <paquete>
```

### Información

```bash
pacman -Si <paquete>
pacman -Qi <paquete>
```

### Instalar

```bash
sudo pacman -S <paquete>
yay -S <paquete>
```

### Actualizar

```bash
sudo pacman -Syu
yay -Syu
```

### Limpiar caché

```bash
sudo pacman -Sc
```

## Ejemplos

- "busca el paquete jq" → buscar.
- "instala jq" → pedir confirmación y ejecutar.
- "desinstala brave" → pedir confirmación fuerte.
