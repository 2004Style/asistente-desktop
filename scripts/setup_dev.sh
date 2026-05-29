#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_HUD="${BUILD_HUD:-1}"

printf '%b\n' "${BLUE}====================================================${NC}"
printf '%b\n' "${BLUE}    Configurando Entorno de Desarrollo de RBot      ${NC}"
printf '%b\n' "${BLUE}====================================================${NC}"

if [ ! -d "skills" ] || [ ! -d "mcp" ] || [ ! -f "go.mod" ]; then
	printf '%b\n' "${RED}Error: Debes ejecutar este script desde la raíz del proyecto.${NC}"
	exit 1
fi

PROJECT_DIR="$ROOT_DIR"
DATA_DIR="${HOME}/.local/share/rbot"
CONFIG_DIR="${HOME}/.config/rbot"

printf '\n%b\n' "${YELLOW}[1/5] Verificando dependencias de desarrollo...${NC}"
MISSING_DEPS=()
for cmd in go curl arecord whisper-cli; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		MISSING_DEPS+=("$cmd")
	fi
done

if ! command -v "piper" >/dev/null 2>&1 && ! command -v "piper-tts" >/dev/null 2>&1; then
	MISSING_DEPS+=("piper/piper-tts")
fi

if [ "$BUILD_HUD" = "1" ] && ! command -v pkg-config >/dev/null 2>&1; then
	MISSING_DEPS+=("pkg-config (required for BUILD_HUD=1)")
fi

if [ ${#MISSING_DEPS[@]} -ne 0 ]; then
	printf '%b\n' "${RED}Aviso: faltan herramientas en tu sistema: ${MISSING_DEPS[*]}${NC}"
	printf '%b\n' "${YELLOW}Revisá 'docs/dependencies.md' antes de seguir.${NC}"
else
	printf '%b\n' "${GREEN}¡Todas las herramientas esenciales de desarrollo están presentes!${NC}"
fi

printf '\n%b\n' "${YELLOW}[2/5] Creando directorios base en el sistema...${NC}"
mkdir -p "${DATA_DIR}/models/piper"
mkdir -p "${DATA_DIR}/models/whisper"
mkdir -p "${CONFIG_DIR}"

SETUP_HQ="${SETUP_HQ:-}"
if [ -z "$SETUP_HQ" ]; then
	if [ -t 0 ]; then
		printf '%b\n' "${BLUE}====================================================${NC}"
		printf '%b\n' "${YELLOW}      Configuración de Calidad de Modelos de Voz    ${NC}"
		printf '%b\n' "${BLUE}====================================================${NC}"
		printf 'Elige la calidad de los modelos de voz locales para desarrollo:\n'
		printf '  1) Modo Ligero [Whisper Tiny (75MB) + Vosk Small (20MB)]\n'
		printf '  2) Modo Alta Calidad [Whisper Small (460MB) + Vosk Profesional (1.4GB)]\n\n'
		printf 'Elige una opción (1 o 2) [Por defecto: 1]: '
		read -r OPT || OPT="1"
		if [ "$OPT" = "2" ]; then
			SETUP_HQ=1
		else
			SETUP_HQ=0
		fi
	else
		SETUP_HQ=0
	fi
fi

mkdir -p voices models config

PIPER_MODEL="voices/es_ES-davefx-medium.onnx"
PIPER_CONFIG="voices/es_ES-davefx-medium.onnx.json"

if [ "$SETUP_HQ" -eq 1 ]; then
	WHISPER_MODEL="models/ggml-small.bin"
	WHISPER_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"
	VOSK_URL="https://alphacephei.com/vosk/models/vosk-model-es-0.42.zip"
	VOSK_ZIP="vosk-model-es-0.42.zip"
	VOSK_DIR_NAME="vosk-model-es-0.42"
else
	WHISPER_MODEL="models/ggml-tiny.bin"
	WHISPER_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
	VOSK_URL="https://alphacephei.com/vosk/models/vosk-model-small-es-0.42.zip"
	VOSK_ZIP="vosk-model-small-es-0.42.zip"
	VOSK_DIR_NAME="vosk-model-small-es-0.42"
fi

printf '\n%b\n' "${YELLOW}[3/5] Verificando y descargando modelos locales de IA...${NC}"
if [ ! -f "$PIPER_MODEL" ]; then
	printf '%b\n' "${BLUE}Descargando modelo Piper ($PIPER_MODEL)...${NC}"
	curl -fsSL --progress-bar -o "$PIPER_MODEL" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx"
fi
if [ ! -f "$PIPER_CONFIG" ]; then
	curl -fsSL -o "$PIPER_CONFIG" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json"
fi

if [ ! -f "$WHISPER_MODEL" ]; then
	printf '%b\n' "${BLUE}Descargando modelo Whisper ($WHISPER_MODEL)...${NC}"
	curl -fsSL --progress-bar -o "$WHISPER_MODEL" "$WHISPER_URL"
fi

VOSK_TARGET_DIR="config/vosk_model"
if [ ! -d "$VOSK_TARGET_DIR" ] || [ ! "$(ls -A "$VOSK_TARGET_DIR" 2>/dev/null)" ]; then
	printf '%b\n' "${BLUE}Descargando modelo Vosk ($VOSK_ZIP)...${NC}"
	VOSK_TMP_DIR="$(mktemp -d)"
	if curl -fsSL --progress-bar "$VOSK_URL" -o "$VOSK_TMP_DIR/$VOSK_ZIP"; then
		printf '%b\n' "${YELLOW}Extrayendo modelo Vosk...${NC}"
		python3 -c "import zipfile, sys; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])" "$VOSK_TMP_DIR/$VOSK_ZIP" "$VOSK_TMP_DIR"
		rm -rf "$VOSK_TARGET_DIR"
		mv "$VOSK_TMP_DIR/$VOSK_DIR_NAME" "$VOSK_TARGET_DIR"
		printf '%b\n' "${GREEN}¡Modelo Vosk instalado con éxito!${NC}"
	else
		printf '%b\n' "${RED}Error al descargar el modelo de Vosk.${NC}"
	fi
	rm -rf "$VOSK_TMP_DIR"
fi

if [ "$SETUP_HQ" -eq 1 ] && [ -f "config/rbot.yaml" ]; then
	printf '%b\n' "${YELLOW}Configurando Whisper en modo Small en config/rbot.yaml...${NC}"
	sed -i 's|models/ggml-tiny.bin|models/ggml-small.bin|g' "config/rbot.yaml" 2>/dev/null || true
	sed -i 's|models/whisper/ggml-tiny.bin|models/whisper/ggml-small.bin|g' "config/rbot.yaml" 2>/dev/null || true
fi

printf '\n%b\n' "${YELLOW}[4/5] Configurando enlaces simbólicos (Symlinks)...${NC}"
rm -rf "${DATA_DIR}/skills"
rm -rf "${DATA_DIR}/models/piper"
rm -rf "${DATA_DIR}/models/whisper"
rm -f "${CONFIG_DIR}/mcp_config.json"
rm -f "${CONFIG_DIR}/rbot.yaml"

mkdir -p "${DATA_DIR}/models"
ln -s "${PROJECT_DIR}/skills" "${DATA_DIR}/skills"
ln -s "${PROJECT_DIR}/voices" "${DATA_DIR}/models/piper"
ln -s "${PROJECT_DIR}/models" "${DATA_DIR}/models/whisper"
ln -s "${PROJECT_DIR}/mcp/mcp_config.json" "${CONFIG_DIR}/mcp_config.json"

if [ -f "${PROJECT_DIR}/config/rbot.yaml" ]; then
	ln -s "${PROJECT_DIR}/config/rbot.yaml" "${CONFIG_DIR}/rbot.yaml"
	printf '%b\n' "${GREEN}✓ Symlink de rbot.yaml creado.${NC}"
fi

printf '%b\n' "${GREEN}✓ Symlink de Skills creado.${NC}"
printf '%b\n' "${GREEN}✓ Symlink de Modelos creado (Piper -> voices, Whisper -> models).${NC}"
printf '%b\n' "${GREEN}✓ Symlink de MCP creado.${NC}"

printf '\n%b\n' "${YELLOW}[5/5] Compilando binarios locales para pruebas...${NC}"
mkdir -p bin

go build -o bin/rbot ./cmd/rbot
go build -o bin/rbotd ./cmd/rbotd
go build -o bin/rbotctl ./cmd/rbotctl
go build -o bin/rbot-settings-gio ./cmd/rbot-settings-gio

go build -o bin/rbot-hud ./cmd/rbot-hud
if [ "$BUILD_HUD" = "1" ]; then
	go build -tags "gtk_3_18 hud" -o bin/rbot-hud ./cmd/rbot-hud
	printf '%b\n' "${GREEN}HUD nativo compilado con -tags "gtk_3_18 hud".${NC}"
else
	printf '%b\n' "${YELLOW}HUD nativo no compilado. Para activarlo: BUILD_HUD=1 ./scripts/setup_dev.sh${NC}"
fi

printf '%b\n' "${GREEN}¡Binarios compilados con éxito en 'bin/'!${NC}"
printf '%b\n' "${BLUE}====================================================${NC}"
printf '%b\n' "${GREEN} ¡Entorno de desarrollo configurado con éxito!      ${NC}"
printf '%b\n' "${BLUE}====================================================${NC}"
printf '%s\n' "Puedes arrancar el daemon en segundo plano:"
printf '%s\n' "  ./bin/rbotd &"
printf '%s\n' "Y usar el runner completo:"
printf '%s\n' "  ./scripts/dev.sh"
