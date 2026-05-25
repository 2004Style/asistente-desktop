---
name: docker-devops-helper
description: Manejo seguro de Docker, Docker Compose, contenedores, logs, redes, volúmenes y diagnósticos de servicios locales.
version: 1.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "docker ps"
  - "levanta docker"
  - "levanta los contenedores"
  - "baja los contenedores"
  - "mira logs de docker"
  - "revisa contenedores"
  - "reinicia contenedor"
  - "limpia docker"
permissions:
  - exec:docker
  - exec:docker-compose
---

# Skill: Docker DevOps Helper

Gestiona Docker y Compose con seguridad.

## Acciones seguras

```bash
docker ps
docker ps -a
docker compose ps
docker compose logs --tail=100
docker images
docker volume ls
docker network ls
```

## Acciones medias

```bash
docker compose up -d
docker compose down
docker restart <contenedor>
docker compose restart <servicio>
```

## Acciones críticas

Requieren confirmación:

```bash
docker system prune
docker system prune -a
docker volume prune
docker compose down -v
docker rm
docker rmi
```

## Reglas

1. No borrar volúmenes sin confirmación fuerte.
2. Antes de reiniciar, mostrar contenedor/servicio afectado.
3. Para logs, limitar salida por defecto.
4. Para Compose, verificar archivo `docker-compose.yml` o `compose.yml`.

## Ejemplos

- "mira los logs del backend" → `docker compose logs --tail=100 backend`.
- "levanta los contenedores" → `docker compose up -d`.
- "limpia docker" → explicar riesgo y pedir confirmación.
