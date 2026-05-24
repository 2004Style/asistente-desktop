---
name: youtube-music
description: Habilidad avanzada para reproducir y buscar música y videos en YouTube con múltiples variaciones de fraseo
version: 2.0.0
author: RBot Premium
risk_level: low
voice_triggers:
  - "pon música"
  - "pon musica"
  - "ponme música"
  - "ponme musica"
  - "colócame"
  - "colocame"
  - "colcoame"
  - "colcame"
  - "reproduceme"
  - "reprodúceme"
  - "reproduseme"
  - "reproduce"
  - "reproducir"
  - "quiero escuchar"
  - "quiero oír"
  - "quiero oir"
  - "me gustaría escuchar"
  - "toca la canción"
  - "pon la canción"
  - "toca"
  - "tocar"
  - "colocame algo de"
  - "colcoame algo de"
  - "ponme algo de"
  - "busca en youtube"
  - "reproduce en youtube"
  - "musica"
  - "música"
  - "cancion"
  - "canción"
  - "cumbia"
  - "phonk"
  - "rock"
  - "reggaeton"
permissions:
  - exec:xdg-open
  - network
---

# Habilidad Premium: YouTube Music

Esta habilidad le permite a RBot resolver CUALQUIER orden relacionada con la reproducción o búsqueda de música y videos en YouTube.

## REGLA FUNDAMENTAL DE ACTIVACIÓN:
Esta habilidad se activa cuando el usuario quiere **escuchar música, reproducir canciones, poner temas o buscar videos musicales**. Se reconocen TODAS estas formas de pedirlo:
- "pon música", "ponme música", "colócame música", "reproduce música", "toca música"
- "quiero escuchar...", "quiero oír...", "me gustaría escuchar..."
- "colócame algo de...", "ponme algo de...", "reprodúceme algo de..."
- "música de cumbia", "canción de linkin park", "toca phonk"
- Cualquier variación con errores tipográficos: "colcoame", "colcame", "reproduseme", "musa"

## REGLA DE NO-ACTIVACIÓN:
**NO actives esta habilidad** si el usuario pide:
- Resúmenes de información ("busca en google X y hazme un resumen")
- Lectura de archivos ("lee el archivo doc.txt")
- Abrir YouTube sin contexto musical ("abre youtube")

## Instrucciones de Orquestación:
1. **Música genérica** (sin artista/tema): Ejecuta `browser.youtube_play(query="cumbia o bachata")`
2. **Artista o tema específico**: Ejecuta `browser.youtube_play(query="<lo que pidió>")`
3. **Búsqueda explícita**: Si dice "busca en youtube", ejecuta `browser.youtube_search(query="<consulta>")`

## Respuestas Conversacionales:
- Confirma de forma cálida y directa dirigiéndose como "señor".
- **Regla de Oro**: NUNCA pronuncies URLs. Di algo natural como *"Enseguida señor, reproduciendo cumbia en YouTube"*.

## Ejemplos de Fraseos Soportados:
* "oye ronald colócame algo de rock de los 80 en youtube" -> `browser.youtube_play(query="rock de los 80")`
* "reprodúceme la canción Numb de Linkin Park" -> `browser.youtube_play(query="Numb de Linkin Park")`
* "ponme música para trabajar" -> `browser.youtube_play(query="música para trabajar")`
* "pon música" -> `browser.youtube_play(query="cumbia o bachata")`
* "quiero escuchar algo de cumbia" -> `browser.youtube_play(query="cumbia")`
* "música de phonk" -> `browser.youtube_play(query="phonk")`
* "toca la canción in the end" -> `browser.youtube_play(query="in the end")`
* "colcoame algo de musica" -> `browser.youtube_play(query="cumbia o bachata")`
