#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# build_release.sh
# Empaqueta RBot y sus dependencias para distribución.
# Genera artefactos precompilados (.tar.gz) listos para publicar.
# ==============================================================================

APP_NAME="rbot"
VERSION="${1:-${VERSION:-1.0.0}}"
BUILD_DIR="build"
OUTPUT_DIR="release"
BUILD_HUD="${BUILD_HUD:-0}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

printf '%b\n' "${BLUE}====================================================${NC}"
printf '%b\n' "${BLUE}       Creando Release de RBot v${VERSION}          ${NC}"
printf '%b\n' "${BLUE}====================================================${NC}"

rm -rf "$BUILD_DIR" "$OUTPUT_DIR"
mkdir -p "$BUILD_DIR" "$OUTPUT_DIR"

build_binary() {
	local target_dir="$1"
	local binary="$2"
	shift 2
	local pkg="$1"
	shift || true
	if [ "$#" -gt 0 ]; then
		go build "$@" -ldflags="-w -s" -o "${target_dir}/bin/${binary}" "$pkg"
	else
		go build -ldflags="-w -s" -o "${target_dir}/bin/${binary}" "$pkg"
	fi
}

# Detectar arquitectura por defecto
HOST_ARCH="$(uname -m)"
case "$HOST_ARCH" in
	x86_64) HOST_ARCH="amd64" ;;
	aarch64|arm64) HOST_ARCH="arm64" ;;
	*) HOST_ARCH="amd64" ;;
esac

ARCHITECTURES=("${1:-$HOST_ARCH}")
BASE_BINARIES=("rbot" "rbotd" "rbotctl" "rbot-settings-gio")

for ARCH in "${ARCHITECTURES[@]}"; do
	printf '\n%b\n' "${YELLOW}[1/4] Compilando para linux/${ARCH}...${NC}"

	TARGET_DIR="${BUILD_DIR}/${APP_NAME}-linux-${ARCH}"
	mkdir -p "${TARGET_DIR}/bin"
	mkdir -p "${TARGET_DIR}/share/rbot"
	mkdir -p "${TARGET_DIR}/config/rbot"
	mkdir -p "${TARGET_DIR}/systemd"
	mkdir -p "${TARGET_DIR}/scripts"

	for binary in "${BASE_BINARIES[@]}"; do
		build_binary "$TARGET_DIR" "$binary" "./cmd/${binary}"
	done

	if [ "$BUILD_HUD" = "1" ]; then
		build_binary "$TARGET_DIR" "rbot-hud" "./cmd/rbot-hud" -tags "gtk_3_18 hud"
	fi

	printf '%b\n' "${GREEN}Binarios compilados exitosamente.${NC}"

	printf '%b\n' "${YELLOW}[2/4] Copiando recursos del proyecto...${NC}"
	if [ -d "skills" ]; then
		cp -r skills "${TARGET_DIR}/share/rbot/"
	fi

	if [ -f "config/rbot.yaml" ]; then
		cp config/rbot.yaml "${TARGET_DIR}/config/rbot/"
	fi

	if [ -f "mcp/mcp_config.json" ]; then
		cp mcp/mcp_config.json "${TARGET_DIR}/config/rbot/"
	fi

	if [ -f "README.md" ]; then
		cp README.md "${TARGET_DIR}/"
	fi

	if [ -d "systemd" ]; then
		cp systemd/*.service "${TARGET_DIR}/systemd/" 2>/dev/null || true
	fi

	if [ -d "scripts" ]; then
		cp -r scripts/* "${TARGET_DIR}/scripts/"
	fi

	if [ -f "install.sh" ]; then
		cp install.sh "${TARGET_DIR}/"
	fi

	printf '%b\n' "${YELLOW}[3/4] Empaquetando en ${APP_NAME}-linux-${ARCH}.tar.gz...${NC}"
	(cd "$BUILD_DIR" && tar -czf "../${OUTPUT_DIR}/${APP_NAME}-linux-${ARCH}.tar.gz" "${APP_NAME}-linux-${ARCH}")
	printf '%b\n' "${GREEN}Artefacto linux-${ARCH} creado.${NC}"
done

printf '\n%b\n' "${YELLOW}[4/4] Limpiando...${NC}"
rm -rf "$BUILD_DIR"

printf '\n%b\n' "${BLUE}====================================================${NC}"
printf '%b\n' "${GREEN} ¡Releases creados en la carpeta 'release/'!         ${NC}"
printf '%b\n' "${BLUE}====================================================${NC}"
ls -lh release/
