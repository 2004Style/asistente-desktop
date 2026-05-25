---
name: file-writer-safe
description: Creación, edición, movimiento, renombrado y borrado seguro de archivos con confirmaciones por riesgo.
version: 1.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "crea un archivo"
  - "crea la carpeta"
  - "crea el directorio"
  - "escribe en el archivo"
  - "modifica el archivo"
  - "renombra"
  - "mueve el archivo"
  - "borra el archivo"
  - "elimina el archivo"
  - "borra la carpeta"
permissions:
  - filesystem:read
  - filesystem:write
---

# Skill: File Writer Safe

Gestiona operaciones que modifican el sistema de archivos.

## Riesgo

- Crear carpeta vacía: bajo.
- Crear archivo nuevo en ruta permitida: bajo/medio.
- Sobrescribir archivo existente: alto.
- Renombrar o mover: medio/alto.
- Borrar: crítico.

## Reglas

1. Nunca sobrescribas sin comprobar si el archivo existe.
2. Para borrar, mostrar ruta exacta y pedir confirmación.
3. Para carpetas con muchos archivos, pedir confirmación reforzada.
4. Nunca operar sobre rutas bloqueadas.
5. Si la ruta es ambigua, buscar primero con `files.search_index`.

## Herramientas

- `files.create_directory(path="<ruta>")`
- `files.create_file(path="<ruta>", content="<contenido>")`
- `files.write_file(path="<ruta>", content="<contenido>")`
- `files.move_path(source="<origen>", target="<destino>")`
- `files.delete_file(path="<ruta>")`
- `files.delete_directory(path="<ruta>")`

## Ejemplos

- "crea una carpeta pruebas en Documentos" → crear sin confirmación si ruta permitida.
- "crea un README en este proyecto" → verificar existencia, luego crear.
- "borra el archivo viejo.log" → buscar, mostrar ruta, pedir confirmación.
