---
name: file-reader-search
description: Búsqueda, lectura, listado y resumen seguro de archivos y carpetas locales sin abrir aplicaciones externas.
version: 2.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "busca el archivo"
  - "encuentra el archivo"
  - "dónde está"
  - "lee el archivo"
  - "léeme el archivo"
  - "qué dice"
  - "contenido de"
  - "resume el archivo"
  - "resume la carpeta"
  - "qué hay en la carpeta"
  - "lista la carpeta"
permissions:
  - filesystem:read
  - db:sqlite
---

# Skill: File Reader Search

Gestiona consultas de lectura y búsqueda local. Nunca debe abrir navegador ni editor salvo que el usuario lo pida explícitamente.

## Activación

Usar esta skill para:

- Buscar archivos o carpetas.
- Leer contenido.
- Resumir documentos.
- Listar directorios.
- Explicar qué contiene una carpeta.

## No activación

No usar para:

- Borrar, mover, renombrar o crear archivos. Eso corresponde a `file-writer-safe`.
- Buscar información en Internet. Eso corresponde a `web-research`.
- Abrir archivos en VS Code. Eso corresponde a `app-launcher` o `project-navigator`.

## Resolución de rutas

1. Si la ruta es explícita, normalizar `~`.
2. Si solo hay nombre, usar primero `files.search_index(query="<nombre>")`.
3. Si hay varias coincidencias, priorizar:
   - Rutas en proyectos conocidos.
   - Rutas modificadas recientemente.
   - Coincidencia exacta de nombre.
4. Si el archivo está en una ruta bloqueada, delegar a `security-guard`.

## Herramientas

- `files.search_index(query="<nombre o patrón>")`
- `files.read_file(path="<ruta>")`
- `files.list_directory(path="<ruta>")`
- `files.summarize_directory(path="<ruta>")` si existe.

## Respuesta

- Para archivos cortos: resumen + puntos clave.
- Para archivos largos: estructura, propósito, secciones importantes.
- Para carpetas: árbol reducido, tipos de archivo, archivos importantes.

## Ejemplos

- "busca mi archivo tesis" → `files.search_index(query="tesis")`.
- "lee package.json de este proyecto" → `files.read_file(path="<ruta>/package.json")`.
- "qué hay en Descargas" → `files.list_directory(path="~/Descargas")`.
