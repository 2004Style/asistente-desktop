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

# Preguntar tipo de modelos a instalar (Ligero vs HQ/Profesional)
INSTALL_HQ=0
if [ -t 0 ]; then
	echo
	echo "${BLUE}====================================================${NC}"
	echo "${YELLOW}      Configuración de Calidad de Modelos de Voz    ${NC}"
	echo "${BLUE}====================================================${NC}"
	echo "Elige la calidad de la transcripción y reconocimiento de voz local:"
	echo "  1) Modo Ligero [Recomendado para equipos estándar]"
	echo "     - Whisper Tiny (75MB) + Vosk Small (20MB)"
	echo "     - Descargas rápidas, menor consumo de RAM y CPU."
	echo "  2) Modo Alta Calidad / Pesado [Recomendado para experiencia tipo Jarvis]"
	echo "     - Whisper Small (460MB) + Vosk Profesional (1.4GB)"
	echo "     - Gran precisión, requiere ~2GB de espacio y más recursos."
	echo
	printf "Elige una opción (1 o 2) [Por defecto: 1]: "
	read -r OPT
	if [ "$OPT" = "2" ]; then
		INSTALL_HQ=1
	fi
else
	echo "${YELLOW}Instalación en modo no interactivo. Usando Modo Ligero por defecto.${NC}"
	echo "Podrás actualizar a Alta Calidad después ejecutando: python3 scripts/download_hq_models.py"
fi

echo "${YELLOW}Verificando modelos de IA...${NC}"

PIPER_MODEL="$DATA_DIR/models/piper/es_ES-davefx-medium.onnx"
PIPER_CONFIG="$DATA_DIR/models/piper/es_ES-davefx-medium.onnx.json"

if [ "$INSTALL_HQ" -eq 1 ]; then
	WHISPER_MODEL="$DATA_DIR/models/whisper/ggml-small.bin"
	WHISPER_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"
	VOSK_URL="https://alphacephei.com/vosk/models/vosk-model-es-0.42.zip"
	VOSK_ZIP="vosk-model-es-0.42.zip"
	VOSK_DIR_NAME="vosk-model-es-0.42"
else
	WHISPER_MODEL="$DATA_DIR/models/whisper/ggml-tiny.bin"
	WHISPER_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
	VOSK_URL="https://alphacephei.com/vosk/models/vosk-model-small-es-0.42.zip"
	VOSK_ZIP="vosk-model-small-es-0.42.zip"
	VOSK_DIR_NAME="vosk-model-small-es-0.42"
fi

if [ ! -f "$PIPER_MODEL" ]; then
	echo "${BLUE}Descargando modelo Piper (es_ES-davefx-medium)...${NC}"
	curl -fsSL --progress-bar -o "$PIPER_MODEL" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx"
fi

if [ ! -f "$PIPER_CONFIG" ]; then
	curl -fsSL -o "$PIPER_CONFIG" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json"
fi

if [ ! -f "$WHISPER_MODEL" ]; then
	echo "${BLUE}Descargando modelo Whisper ($(basename "$WHISPER_MODEL"))...${NC}"
	curl -fsSL --progress-bar -o "$WHISPER_MODEL" "$WHISPER_URL"
fi

# Descargar y extraer el modelo de Vosk en $DATA_DIR/config/vosk_model
VOSK_TARGET_DIR="$DATA_DIR/config/vosk_model"
if [ ! -d "$VOSK_TARGET_DIR" ] || [ ! "$(ls -A "$VOSK_TARGET_DIR" 2>/dev/null)" ]; then
	echo "${BLUE}Descargando modelo de Vosk ($(basename "$VOSK_URL"))...${NC}"
	mkdir -p "$DATA_DIR/config"
	VOSK_TMP_DIR="$(mktemp -d)"
	if curl -fsSL --progress-bar "$VOSK_URL" -o "$VOSK_TMP_DIR/$VOSK_ZIP"; then
		echo "${YELLOW}Extrayendo modelo Vosk...${NC}"
		python3 -c "import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])" "$VOSK_TMP_DIR/$VOSK_ZIP" "$VOSK_TMP_DIR"
		rm -rf "$VOSK_TARGET_DIR"
		mv "$VOSK_TMP_DIR/$VOSK_DIR_NAME" "$VOSK_TARGET_DIR"
		echo "${GREEN}¡Modelo Vosk instalado con éxito!${NC}"
	else
		echo "${RED}Error al descargar el modelo de Vosk.${NC}"
	fi
	rm -rf "$VOSK_TMP_DIR"
fi

if [ "$INSTALL_HQ" -eq 1 ] && [ -f "$CONFIG_DIR/rbot.yaml" ]; then
	echo "${YELLOW}Configurando Whisper en modo Small en rbot.yaml...${NC}"
	sed -i 's|models/ggml-tiny.bin|models/ggml-small.bin|g' "$CONFIG_DIR/rbot.yaml" 2>/dev/null || true
	sed -i 's|models/whisper/ggml-tiny.bin|models/whisper/ggml-small.bin|g' "$CONFIG_DIR/rbot.yaml" 2>/dev/null || true
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
