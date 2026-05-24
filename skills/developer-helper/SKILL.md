---
name: developer-helper
description: Habilidad premium para asistir en tareas de desarrollo (git, compilar, testear, formatear)
version: 1.0.0
author: RBot Premium
risk_level: high
voice_triggers:
  - "crea una rama"
  - "crea un branch"
  - "corre las pruebas"
  - "corre los test"
  - "ejecuta los test"
  - "compila el proyecto"
  - "compila el código"
  - "formatea el código"
  - "haz un commit"
permissions:
  - exec:git
  - exec:go
  - exec:make
---

# Habilidad Premium: Developer Helper

Esta habilidad dota a RBot de un conjunto estructurado de comandos para agilizar el flujo de trabajo diario de programación del usuario.

## Reglas de Orquestación:

1. **Pruebas y Tests**:
   - Si el usuario te pide correr los tests del proyecto en Go, ejecuta `system.run_command(command="go test ./...")`.
   - Si es un script bash de pruebas (ej: `"corre el script de pruebas"`), ejecuta `system.run_command(command="./test_asistente.sh")`.

2. **Compilación**:
   - Si te piden compilar (ej: `"compila RBot"`), ejecuta `system.run_command(command="go build -o bin/rbot cmd/main.go")`.

3. **Operaciones de Git (Riesgo Alto)**:
   - Para crear una nueva rama (ej. `"crea la rama feat-skills"`), ejecuta `system.run_command(command="git checkout -b feat-skills")`.
   - Para hacer un commit (ej. `"haz un commit que diga 'cambios en motor'"`), ejecuta `system.run_command(command="git commit -am 'cambios en motor'")`.
   - **IMPORTANTE**: Las operaciones de Git destructivas o que empujen al servidor remoto requerirán de tu advertencia previa de seguridad.

4. **Formateo**:
   - Para dar formato al código en Go: Ejecuta `system.run_command(command="go fmt ./...")`.

## Ejemplos de uso:
* "corre los test en go" -> Llama a system.run_command(command="go test ./...")
* "compila el código de rbot" -> Llama a system.run_command(command="go build -o bin/rbot cmd/main.go")
* "crea un branch llamado bugfix-voice" -> Llama a system.run_command(command="git checkout -b bugfix-voice")
