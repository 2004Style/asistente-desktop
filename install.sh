#!/usr/bin/env sh
set -e

# ==============================================================================
# RBot - Instalador Universal
# Descarga el artefacto precompilado, ubica los recursos según XDG e instala los modelos.
# Uso: curl -fsSL https://raw.githubusercontent.com/2004Style/asistente-desktop/main/install.sh | sh
# ==============================================================================

APP_NAME="rbot"
REPO="2004Style/asistente-desktop"

# Rutas estándar XDG para instalación a nivel de usuario
BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/${APP_NAME}"
CONFIG_DIR="${HOME}/.config/${APP_NAME}"
TMP_DIR="$(mktemp -d)"

# Colores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo "${BLUE}====================================================${NC}"
echo "${BLUE}          Instalando RBot - Desktop Agent           ${NC}"
echo "${BLUE}====================================================${NC}"

# 1. Validar SO y Arquitectura
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "${RED}Error: Arquitectura no soportada: $ARCH${NC}"; exit 1 ;;
esac

if [ "$OS" != "linux" ]; then
    echo "${RED}Error: RBot actualmente solo soporta Linux.${NC}"
    exit 1
fi

ARCHIVE="${APP_NAME}-${OS}-${ARCH}.tar.gz"

# Como este repositorio puede no tener los releases publicados todavía,
# utilizamos un fallback local si el .tar.gz existe en la carpeta local (para desarrollo)
# De lo contrario, intentará descargarlo de GitHub.
if [ -f "release/${ARCHIVE}" ]; then
    echo "${YELLOW}Instalando desde artefacto local (release/${ARCHIVE})...${NC}"
    cp "release/${ARCHIVE}" "$TMP_DIR/$ARCHIVE"
else
    # Descarga desde releases de github
    URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"
    echo "${YELLOW}Descargando artefacto de ${URL}...${NC}"
    if ! curl -fsSL "$URL" -o "$TMP_DIR/$ARCHIVE"; then
        echo "${RED}Error: No se pudo descargar el release desde GitHub.${NC}"
        echo "${RED}Asegúrate de que el release ${ARCHIVE} esté publicado en ${REPO}.${NC}"
        rm -rf "$TMP_DIR"
        exit 1
    fi
fi

# 2. Descomprimir e Instalar
mkdir -p "$BIN_DIR" "$DATA_DIR/models/piper" "$DATA_DIR/models/whisper" "$CONFIG_DIR"

echo "${YELLOW}Descomprimiendo artefacto...${NC}"
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

# Instalar binarios
install -m 755 "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/bin/rbot" "$BIN_DIR/rbot"
install -m 755 "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/bin/rbotd" "$BIN_DIR/rbotd"
install -m 755 "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/bin/rbotctl" "$BIN_DIR/rbotctl"

# Copiar recursos (skills)
if [ -d "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/share/rbot" ]; then
    cp -R "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/share/rbot/." "$DATA_DIR/"
fi

# Copiar configuraciones por defecto (rbot.yaml, mcp_config.json) sin sobreescribir las del usuario
if [ -d "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/config/rbot" ]; then
    for conf_file in "$TMP_DIR/${APP_NAME}-${OS}-${ARCH}/config/rbot/"*; do
        if [ -f "$conf_file" ]; then
            filename=$(basename "$conf_file")
            if [ ! -f "$CONFIG_DIR/$filename" ]; then
                cp "$conf_file" "$CONFIG_DIR/$filename"
            fi
        fi
    done
fi

rm -rf "$TMP_DIR"

# 3. Descarga de modelos pesados directamente a la carpeta de usuario
echo "${YELLOW}Verificando modelos de IA...${NC}"

PIPER_MODEL="$DATA_DIR/models/piper/es_ES-davefx-medium.onnx"
PIPER_CONFIG="$DATA_DIR/models/piper/es_ES-davefx-medium.onnx.json"
WHISPER_MODEL="$DATA_DIR/models/whisper/ggml-tiny.bin"

if [ ! -f "$PIPER_MODEL" ]; then
    echo "${BLUE}Descargando modelo Piper (es_ES-davefx-medium)...${NC}"
    curl -fsSL -o "$PIPER_MODEL" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx"
fi

if [ ! -f "$PIPER_CONFIG" ]; then
    curl -fsSL -o "$PIPER_CONFIG" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json"
fi

if [ ! -f "$WHISPER_MODEL" ]; then
    echo "${BLUE}Descargando modelo Whisper (ggml-tiny)...${NC}"
    curl -fsSL -o "$WHISPER_MODEL" "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
fi

# 4. Verificar dependencias runtime
echo "${YELLOW}Verificando dependencias runtime...${NC}"
MISSING_DEPS=""
for cmd in arecord whisper-cli; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        MISSING_DEPS="$MISSING_DEPS $cmd"
    fi
done

if ! command -v "piper" >/dev/null 2>&1 && ! command -v "piper-tts" >/dev/null 2>&1; then
    MISSING_DEPS="$MISSING_DEPS piper"
fi

echo "${BLUE}====================================================${NC}"
echo "${GREEN} ¡Instalación de RBot completada!                   ${NC}"
echo "${BLUE}====================================================${NC}"
echo "Los binarios fueron instalados en: ${GREEN}$BIN_DIR${NC} (rbot, rbotd, rbotctl)"
echo "Asegúrate de que $BIN_DIR esté en tu \$PATH."
echo "Los recursos y modelos están en: ${GREEN}$DATA_DIR${NC}"
echo "La configuración de MCP está en: ${GREEN}$CONFIG_DIR${NC}"
 
if [ -n "$MISSING_DEPS" ]; then
    echo "\n${RED}Aviso:${NC} Parecen faltar algunas dependencias en tu sistema:${YELLOW}$MISSING_DEPS${NC}"
    echo "Instálalas (ej. apt install piper whisper sox) para que el modo de voz funcione correctamente."
fi
 
echo "\n${YELLOW}Para arrancar el daemon residente en segundo plano, ejecuta:${NC}"
echo "  rbotd &"
echo ""
echo "Para enviarle órdenes o consultar su estado, utiliza el controlador:"
echo "  rbotctl status"
echo "  rbotctl say \"Hola Ronald\""
echo ""
echo "Para tareas offline de mantenimiento (como indexación), usa:"
echo "  rbot index apps"
echo ""
