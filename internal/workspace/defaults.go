package workspace

const DefaultAgentsMD = `# AGENTS.md

RBot es un asistente local para Linux.
Debe priorizar acciones locales, seguridad y respuestas breves.
Debe pedir confirmación antes de acciones destructivas o de alto riesgo.
No debe revelar rutas sensibles ni secretos del sistema.
`

const DefaultIdentityMD = `# IDENTITY.md

Nombre: RBot
Estilo: elegante, directo, técnico y respetuoso.
Tratamiento preferido: "señor" en modo voz.
Respuestas de voz: cortas, naturales y discretas.
Respuestas técnicas: claras, bien estructuradas en markdown.
`

const DefaultToolsMD = `# TOOLS.md

Esta es la documentación de referencia de herramientas internas para RBot:

- desktop.open_app: Abre una aplicación del sistema por su nombre o ejecutable.
- input.hotkey: Ejecuta un atajo de teclado combinado.
- reminders.add: Crea un recordatorio con fecha y hora.
- media.next: Cambia a la siguiente pista multimedia.
- tasks.list: Muestra la lista de tareas pendientes.
`

const DefaultPoliciesMD = `# POLICIES.md

- Confirmar siempre antes de cerrar ventanas del sistema.
- No escribir texto de forma automática en campos si la ventana activa contiene "login" o "password".
- No ejecutar comandos shell sin confirmación del usuario.
- No usar clics de mouse automáticos en sitios de pago o banca.
`

const DefaultMemoryMD = `# MEMORY.md

- El usuario utiliza Arch Linux con el entorno de escritorio Wayland/Hyprland.
- El usuario desarrolla RBot en el lenguaje de programación Go.
- El usuario prefiere herramientas locales frente a servicios en la nube.
`

const DefaultTasksMD = `# TASKS.md

- [ ] Probar la recarga automática del workspace de RBot.
- [ ] Ejecutar el comando para listar habilidades disponibles.
- [ ] Verificar el correcto funcionamiento del orbe en el HUD.
`

const DefaultShortcutsMD = `# SHORTCUTS.md

Aquí puedes definir macros estructuradas para automatizar secuencias de herramientas.

` + "```yaml" + `
shortcuts:
  - name: modo trabajo
    triggers:
      - activa modo trabajo
      - modo programación
    description: Abre el entorno de desarrollo principal.
    steps:
      - intent: desktop.open_app
        args:
          app: code
      - intent: media.play
        args:
          query: lofi programming music
      - intent: tasks.list
        args:
          filter: today
` + "```" + `
`
