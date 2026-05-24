---
name: file-manager
description: Habilidad avanzada para la gestión, lectura, resumen, creación y búsqueda de archivos o directorios locales
version: 2.0.0
author: RBot Premium
risk_level: high
voice_triggers:
  - "crea la carpeta"
  - "crea el directorio"
  - "crea un archivo"
  - "crea el archivo"
  - "lee el archivo"
  - "leeme el archivo"
  - "contenido de"
  - "contenido del"
  - "resumen de la carpeta"
  - "resumen del archivo"
  - "dame un resumen"
  - "qué hay en la carpeta"
  - "qué hay en"
  - "elimina el archivo"
  - "borra el archivo"
  - "elimina la carpeta"
  - "borra la carpeta"
  - "lee el contenido"
  - "qué dice el archivo"
permissions:
  - filesystem:read
  - filesystem:write
---

# Habilidad Premium: File Manager Avanzado

Esta habilidad dota al agente de instrucciones precisas para interactuar con el sistema de archivos del usuario.

## REGLA FUNDAMENTAL:
Cuando el usuario pide **leer**, **ver**, **resumir** o **consultar** el contenido de un archivo, DEBES:
1. Usar la herramienta `files.read_file` con la ruta correcta.
2. Presentar el contenido de forma **resumida y natural** al usuario.
3. **NUNCA** abras el navegador ni otra aplicación — solo lee el archivo.

## RESOLUCIÓN INTELIGENTE DE RUTAS:
El usuario puede referirse a archivos de forma natural. Tú debes resolver la ruta:
- "doc.txt en Descargas/test-asistente" → `path="~/Descargas/test-asistente/doc.txt"`
- "nota.txt de la carpeta test-asistente" → `path="~/Descargas/test-asistente/nota.txt"` o buscar con `files.search_index`
- "el archivo tareas.txt" → Buscar primero con `files.search_index(query="tareas.txt")` si no conoces la ruta exacta.
- Las rutas con `~` se expanden a `/home/<usuario>/`.

## Reglas de Orquestación:

1. **Lectura y Resumen de Archivos**:
   - Si te piden "lee el archivo X", "contenido de X", "dame un resumen de X", "qué dice X":
     - Ejecuta `files.read_file(path="<ruta>")`.
     - Presenta el resultado de forma natural y resumida.
   - Si no conoces la ruta exacta, usa `files.search_index(query="<nombre>")` primero.

2. **Creación de Directorios**:
   - Ejecuta `files.create_directory(path="<ruta>")`.

3. **Creación de Archivos**:
   - Ejecuta `files.create_file(path="<ruta>", content="<contenido>")`.

4. **Borrado de Archivos (Crítico)**:
   - Para eliminar: Ejecuta `files.delete_file(path="<ruta>")`.
   - **IMPORTANTE**: Si no conoces la ruta exacta, busca primero con `files.search_index`.
   - Informa al usuario que estás solicitando permiso antes de ejecutar.

5. **Listado de Carpetas**:
   - Ejecuta `files.list_directory(path="<ruta>")`.

## Ejemplos de uso:
* "crea una carpeta proyectos en Documentos" → `files.create_directory(path="~/Documentos/proyectos")`
* "lee el archivo doc.txt de Descargas/test-asistente" → `files.read_file(path="~/Descargas/test-asistente/doc.txt")`
* "dame un resumen de doc.txt en Descargas/test-asistente" → `files.read_file(path="~/Descargas/test-asistente/doc.txt")` + resumir
* "contenido de doc.txt en Descargas/test-asistente" → `files.read_file(path="~/Descargas/test-asistente/doc.txt")`
* "elimina nota.txt de Descargas/test-asistente" → `files.delete_file(path="~/Descargas/test-asistente/nota.txt")`
* "qué hay en la carpeta Descargas" → `files.list_directory(path="~/Descargas")`
