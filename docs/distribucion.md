# Ciclo de Vida de Distribución y Empaquetado

RBot utiliza una arquitectura de distribución profesional basada en **artefactos compilados** (releases) y **enlaces simbólicos** para desarrollo. Esto garantiza que el código fuente y las herramientas de compilación (`go`) se mantengan en el entorno del desarrollador, mientras que el usuario final recibe una experiencia "Plug & Play".

---

## 1. El Entorno de Desarrollo (`setup_dev.sh`)

**Público objetivo:** Desarrolladores (Tú).

Cuando desarrollas RBot, necesitas que cualquier cambio en las habilidades (`skills/`) o configuraciones (`mcp/mcp_config.json`) se refleje de forma instantánea al ejecutar el asistente.

Para lograr esto, ejecuta en la raíz del proyecto:
```bash
./scripts/setup_dev.sh
```

**¿Qué hace?**
1. Verifica que tengas las herramientas de desarrollo necesarias instaladas (`go`, `curl`, `arecord`, etc.).
2. Valida que tengas las carpetas de modelos (`voices/` y `models/`) en tu proyecto.
3. Crea **Enlaces Simbólicos (Symlinks)** desde el estándar XDG de Linux (`~/.local/share/rbot/...`) apuntando directamente a tus carpetas de código.
4. Compila un binario local rápido en `bin/rbot`.

Gracias a esto, el sistema operativo lee tus archivos en tiempo real sin tener que copiarlos ni duplicarlos.

---

## 2. El Empaquetador de Artefactos (`build_release.sh`)

**Público objetivo:** Desarrolladores (Integración Continua / CI).

Cuando terminas de programar una nueva versión y deseas publicarla, no entregas el código fuente, sino un **Artefacto**.

```bash
./scripts/build_release.sh 1.0.0
```

**¿Qué hace?**
1. Utiliza la *compilación cruzada* de Go para generar binarios nativos hiper-optimizados (sin dependencias) para `amd64` (PC) y `arm64` (Raspberry Pi/Mac).
2. Empaqueta el binario junto a las carpetas `skills/` y `mcp/mcp_config.json` en archivos `.tar.gz`.
3. Los guarda en la carpeta local `release/`.

**Alojamiento (GitHub vs CDN):**
Una vez tienes el archivo `rbot-linux-amd64.tar.gz`, eres totalmente libre de decidir dónde alojarlo:
- **GitHub Releases:** Es lo más común. Subes el archivo a la pestaña de Releases de tu repositorio.
- **CDN Propio (Amazon S3, Cloudflare, etc.):** Puedes subir los `.tar.gz` a un servidor web extremadamente rápido o a un bucket estático en la nube.

---

## 3. El Instalador Universal (`install.sh`)

**Público objetivo:** Usuarios Finales.

El usuario final no requiere clonar el repositorio de GitHub ni compilar nada. Solamente ejecuta una línea en su terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/TuUsuario/asistente-desktop/main/install.sh | sh
```

**¿Qué hace?**
1. **Detección:** Revisa qué arquitectura tiene el usuario (Linux amd64 o arm64).
2. **Descarga:** Se conecta a la nube (GitHub o CDN) para descargar el `.tar.gz` correspondiente.
   *Nota: Si tú modificas el archivo `install.sh` y cambias la variable `URL="..."`, puedes hacer que el script descargue los releases desde `https://cdn.tu-dominio.com/rbot/rbot-linux-amd64.tar.gz` en lugar de GitHub. ¡Tú tienes el control total!*
3. **Mapeo XDG:** Descomprime el archivo y traslada las piezas a su hogar definitivo en Linux:
   - `bin/rbot` ➔ `~/.local/bin/rbot`
   - `share/rbot/skills` ➔ `~/.local/share/rbot/skills`
   - `config/rbot/mcp_config.json` ➔ `~/.config/rbot/mcp_config.json`
4. **Descarga de Pesos (Modelos):** Dado que empacar los pesados modelos de IA (100MB+) dentro de cada `.tar.gz` saturaría tu ancho de banda y tus releases, el instalador los descarga bajo demanda directamente desde HuggingFace y los ubica en el equipo del usuario de forma transparente.

Con este flujo, logras una distribución limpia, escalable y modular, lista para la distribución masiva.
