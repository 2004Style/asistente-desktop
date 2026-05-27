#!/usr/bin/env sh
set -e

# ==============================================================================
# RBot - Instalador Universal
# Descarga el artefacto precompilado, ubica los recursos según XDG e instala los
# binarios publicados por el release.
# Uso: curl -fsSL https://raw.githubusercontent.com/2004Style/asistente-desktop/main/install.sh | sh
# ==============================================================================

APP_NAME="rbot"
REPO="2004Style/asistente-desktop"

BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/${APP_NAME}"
CONFIG_DIR="${HOME}/.config/${APP_NAME}"
SYSTEMD_DIR="${HOME}/.config/systemd/user"
TMP_DIR="$(mktemp -d)"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo "${BLUE}====================================================${NC}"
echo "${BLUE}          Instalando RBot - Desktop Agent           ${NC}"
echo "${BLUE}====================================================${NC}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
x86_64) ARCH="amd64" ;;
aarch64 | arm64) ARCH="arm64" ;;
*)
	echo "${RED}Error: Arquitectura no soportada: $ARCH${NC}"
	exit 1
	;;
esac

if [ "$OS" != "linux" ]; then
	echo "${RED}Error: RBot actualmente solo soporta Linux.${NC}"
	exit 1
fi

ARCHIVE="${APP_NAME}-${OS}-${ARCH}.tar.gz"

if [ -f "release/${ARCHIVE}" ]; then
	echo "${YELLOW}Instalando desde artefacto local (release/${ARCHIVE})...${NC}"
	cp "release/${ARCHIVE}" "$TMP_DIR/$ARCHIVE"
else
	URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"
	echo "${YELLOW}Descargando artefacto de ${URL}...${NC}"
	if ! curl -fsSL "$URL" -o "$TMP_DIR/$ARCHIVE"; then
		echo "${RED}Error: No se pudo descargar el release desde GitHub.${NC}"
		echo "${RED}Asegúrate de que el release ${ARCHIVE} esté publicado en ${REPO}.${NC}"
		rm -rf "$TMP_DIR"
		exit 1
	fi
fi

mkdir -p "$BIN_DIR" "$DATA_DIR/models/piper" "$DATA_DIR/models/whisper" "$CONFIG_DIR" "$SYSTEMD_DIR"

echo "${YELLOW}Descomprimiendo artefacto...${NC}"
tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

RELEASE_ROOT="$TMP_DIR/${APP_NAME}-${OS}-${ARCH}"
if [ ! -d "$RELEASE_ROOT" ]; then
	echo "${RED}Error: el artefacto no contiene ${RELEASE_ROOT}.${NC}"
	rm -rf "$TMP_DIR"
	exit 1
fi

if [ -d "$RELEASE_ROOT/bin" ]; then
	for binary in "$RELEASE_ROOT/bin/"*; do
		[ -f "$binary" ] || continue
		install -m 755 "$binary" "$BIN_DIR/$(basename "$binary")"
	done
fi

if [ -d "$RELEASE_ROOT/share/rbot" ]; then
	cp -R "$RELEASE_ROOT/share/rbot/." "$DATA_DIR/"
fi

if [ -d "$RELEASE_ROOT/config/rbot" ]; then
	for conf_file in "$RELEASE_ROOT/config/rbot/"*; do
		[ -f "$conf_file" ] || continue
		filename=$(basename "$conf_file")
		if [ ! -f "$CONFIG_DIR/$filename" ]; then
			cp "$conf_file" "$CONFIG_DIR/$filename"
		fi
	done
fi

if [ -d "$RELEASE_ROOT/systemd" ]; then
	for unit in "$RELEASE_ROOT/systemd/"*; do
		[ -f "$unit" ] || continue
		cp "$unit" "$SYSTEMD_DIR/$(basename "$unit")"
	done
fi

rm -rf "$TMP_DIR"

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
echo "Los binarios fueron instalados en: ${GREEN}$BIN_DIR${NC}"
echo "Los recursos y modelos están en: ${GREEN}$DATA_DIR${NC}"
echo "La configuración está en: ${GREEN}$CONFIG_DIR${NC}"
echo "Las unidades systemd del usuario están en: ${GREEN}$SYSTEMD_DIR${NC}"

if [ -n "$MISSING_DEPS" ]; then
	printf '\n%b\n' "${RED}Aviso:${NC} parecen faltar dependencias runtime:${YELLOW}${MISSING_DEPS}${NC}"
	echo "Instálalas para que el modo de voz funcione correctamente."
fi

echo
printf '%b\n' "${YELLOW}Para arrancar el daemon residente en segundo plano:${NC}"
echo "  rbotd &"
echo
printf '%b\n' "${YELLOW}Para correr el entorno de desarrollo completo:${NC}"
echo "  BUILD_HUD=1 ./scripts/dev.sh"
echo
printf '%b\n' "${YELLOW}Para gestionar providers y modelos desde la CLI:${NC}"
echo "  rbotctl providers list"
echo "  rbotctl models list"
echo "  rbotctl settings providers list   # alias de compatibilidad"
