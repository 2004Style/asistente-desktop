# Prevención de Interferencias, Conflictos y Autoescucha

Este documento describe las medidas técnicas implementadas en RBot para evitar problemas de autoescucha (acoplamiento del sonido del asistente en el micrófono), conflictos de acceso a dispositivos de audio y concurrencia en archivos de bases de datos o sockets.

---

## 1. Control Anti-Eco y Autoescucha (Micrófono y TTS)

### El Problema
Al reproducir la voz del asistente por los altavoces a un volumen elevado, el micrófono del sistema puede capturar ese sonido ("autoescucha"), transcribirlo e interpretarlo como una nueva orden de voz del usuario. Esto genera un bucle infinito de auto-órdenes y respuestas falsas.

### Soluciones Implementadas
1. **Cooldown Post-Habla (Silenciado Temporal de Captura)**:
   * Al emitir una respuesta de texto a voz (TTS), el motor de grabación Vosk (`scripts/rbot-voice-vosk.py`) entra en un estado inactivo.
   * Al recibir la señal `tts.finished`, se aplica un cooldown preventivo de **1.8 segundos**.
   * Durante este lapso, el reconocedor de voz descarta todas las tramas de audio entrantes del micrófono, vacía la cola interna de buffers y purga el reconocedor ejecutando `rec.Result()`.
2. **Umbrales Activos de Silencio (Sox/Rec)**:
   * El comando de grabación inteligente `rec` filtra el ruido ambiental de fondo e interrumpe la grabación cuando detecta silencios largos, impidiendo el procesamiento de transcripciones de audio vacío.

---

## 2. Bloqueo de Recursos de Hardware de Audio (ALSA/PulseAudio/PipeWire)

### El Problema
Los dispositivos ALSA en Linux nativo pueden ser bloqueados por un único proceso de grabación o reproducción, impidiendo que otros procesos emitan sonidos (TTS) o graben (Vosk) al mismo tiempo.

### Soluciones Implementadas
* **Integración con Servidores de Audio Modernos**: Se recomienda el uso de **PulseAudio** o **PipeWire**. Sus mezcladores por software permiten que múltiples procesos accedan de forma simultánea a la misma tarjeta de captura y reproducción de audio sin colisionar.
* **Fallback a arecord**: En caso de fallas con el filtro inteligente de `sox`, el motor retrocede al comando estándar `arecord`, que usa flujos de ALSA compartidos.

---

## 3. Sockets y Concurrencia de Procesos IPC

### El Problema
Intentar levantar múltiples instancias del daemon principal (`rbotd`) o del panel de Ajustes en paralelo puede ocasionar conflictos de enlace en los puertos o sockets IPC (`rbot.sock` y `events.sock`).

### Soluciones Implementadas
* **Socket Locks únicos**: El daemon principal mantiene un archivo de socket unix ubicado en `/home/style/.local/share/rbot/rbot.sock` de forma exclusiva. Si una nueva instancia intenta iniciarse e IPC no puede conectarse, el socket antiguo y huérfano se limpia automáticamente para evitar fallos de inicialización persistentes.
* **Manejo Conversacional**: Si el daemon no está listo, el HUD responde de manera directa con un mensaje amigable al usuario indicando el estado en lugar de crashear el proceso.
