#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BUILD_HUD="${BUILD_HUD:-0}"

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

printf '\n%b\n' "${YELLOW}[3/5] Verificando modelos locales de IA en el repositorio...${NC}"
if [ ! -d "voices" ] || [ ! -d "models" ]; then
	printf '%b\n' "${RED}Error: No se encontraron las carpetas 'voices' y 'models' en tu proyecto.${NC}"
	exit 1
fi
printf '%b\n' "${GREEN}Carpetas de modelos detectadas en el repositorio.${NC}"

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
	go build -tags hud -o bin/rbot-hud ./cmd/rbot-hud
	printf '%b\n' "${GREEN}HUD nativo compilado con -tags hud.${NC}"
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
