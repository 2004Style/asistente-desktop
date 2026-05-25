---
name: voice-command-cleaner
description: Limpieza de comandos de voz, corrección de errores de Whisper, normalización de intención y activación robusta.
version: 1.0.0
author: RBot Max Pack
risk_level: low
voice_triggers:
  - "colcoame"
  - "colcame"
  - "reproduseme"
  - "bavegador"
  - "wokrspace"
  - "nececito"
  - "hazme"
permissions: []
---

# Skill: Voice Command Cleaner

Normaliza comandos transcritos por voz antes de enrutarlos.

## Correcciones frecuentes

- "colcoame", "colcame" → "colócame"
- "reproduseme" → "reprodúceme"
- "bavegador" → "navegador"
- "wokrspace" → "workspace"
- "nececito" → "necesito"
- "pestania" → "pestaña"
- "musica" → "música"
- "donet" → ".NET"
- "youtuve" → "YouTube"

## Reglas

1. Corregir sin cambiar la intención.
2. Mantener entidades técnicas: rutas, comandos, nombres de paquetes.
3. No corregir comandos shell si eso puede romperlos.
4. Si la corrección cambia una acción crítica, pedir confirmación.

## Ejemplos

- "colcoame algo de musica" → intención: reproducir música.
- "qué pestanias tengo en mi bavegador" → intención: inspeccionar navegador/ventanas.
- "corre los tes" → intención: ejecutar tests.
