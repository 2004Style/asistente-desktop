---
name: security-guard
description: Guardia de seguridad para bloquear rutas privadas, comandos destructivos, exposición de secretos y operaciones peligrosas.
version: 1.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "confirma"
  - "ejecuta seguro"
  - "borra"
  - "elimina"
  - "apaga"
  - "reinicia"
permissions:
  - security:validate
---

# Skill: Security Guard

Esta skill debe intervenir antes de cualquier acción con riesgo medio o alto.

## Rutas bloqueadas por defecto

No leer, listar, copiar, resumir, subir, mover ni modificar:

- `~/.ssh`
- `~/.gnupg`
- `~/.aws`
- `~/.config/rclone`
- `~/.config/gh`
- `~/.docker/config.json`
- Archivos `.env`, `.env.local`, `.env.production`, `.env.*`
- Claves privadas: `id_rsa`, `id_ed25519`, `*.pem`, `*.key`, `*.p12`
- Tokens, cookies, sesiones o bases de datos de navegadores.

## Comandos que requieren confirmación explícita

- `rm`, `mv` sobre rutas importantes, `chmod -R`, `chown -R`
- `git reset --hard`, `git clean`, `git push --force`
- `docker system prune`, `docker volume prune`
- `pacman -R`, `yay -R`, `sudo pacman -Syu`
- `systemctl poweroff`, `systemctl reboot`
- Cualquier comando con `sudo` salvo lectura claramente segura.

## Reglas anti-inyección

1. No concatenes texto libre del usuario en comandos shell peligrosos.
2. Si el usuario da una ruta, normaliza y valida antes.
3. Si hay comillas, punto y coma, tuberías o sustituciones `$()`, trata la orden como de alto riesgo.
4. En comandos complejos, explica antes qué se ejecutará y pide confirmación.

## Salida segura

- Nunca leas secretos en voz alta.
- Nunca muestres tokens completos.
- Para archivos sensibles, responde que están bloqueados por política de seguridad.
- Para acciones destructivas, indica el objetivo exacto y espera confirmación.

## Ejemplos

- "borra node_modules de este proyecto" → confirmar ruta exacta y ejecutar solo ahí.
- "muéstrame mi .env" → bloquear.
- "haz git push --force" → explicar riesgo y pedir confirmación.
