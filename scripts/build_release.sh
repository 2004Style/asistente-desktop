#!/usr/bin/env bash

set -e

# ==============================================================================
# build_release.sh
# Empaqueta RBot y sus dependencias (skills, configuración MCP) para distribución.
# Genera artefactos precompilados (.tar.gz) listos para subir a GitHub Releases.
# ==============================================================================

APP_NAME="rbot"
VERSION="${1:-1.0.0}" # Puede recibir la versión como argumento, por defecto 1.0.0
BUILD_DIR="build"
OUTPUT_DIR="release"

# Colores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}====================================================${NC}"
echo -e "${BLUE}       Creando Release de RBot v${VERSION}          ${NC}"
echo -e "${BLUE}====================================================${NC}"

# Limpiar directorios previos
rm -rf "$BUILD_DIR" "$OUTPUT_DIR"
mkdir -p "$BUILD_DIR" "$OUTPUT_DIR"

# Arquitecturas a soportar
ARCHITECTURES=("amd64" "arm64")

for ARCH in "${ARCHITECTURES[@]}"; do
    echo -e "\n${YELLOW}[1/4] Compilando para linux/${ARCH}...${NC}"
    
    TARGET_DIR="${BUILD_DIR}/${APP_NAME}-linux-${ARCH}"
    mkdir -p "${TARGET_DIR}/bin"
    mkdir -p "${TARGET_DIR}/share/rbot"
    mkdir -p "${TARGET_DIR}/config/rbot"

    # Compilar el binario
    GOOS=linux GOARCH=${ARCH} go build -ldflags="-w -s" -o "${TARGET_DIR}/bin/${APP_NAME}" cmd/main.go
    
    echo -e "${GREEN}Binario compilado exitosamente.${NC}"

    echo -e "${YELLOW}[2/4] Copiando recursos del proyecto...${NC}"
    # Copiar skills
    if [ -d "skills" ]; then
        cp -r skills "${TARGET_DIR}/share/rbot/"
    fi
    
    # Copiar configuración predeterminada (rbot.yaml y mcp_config.json)
    mkdir -p "${TARGET_DIR}/config/rbot"
    
    if [ -f "config/rbot.yaml" ]; then
        cp config/rbot.yaml "${TARGET_DIR}/config/rbot/"
    fi
    
    if [ -f "mcp/mcp_config.json" ]; then
        cp mcp/mcp_config.json "${TARGET_DIR}/config/rbot/"
    fi
    
    # Copiar documentación útil si el usuario quiere leerla
    cp README.md "${TARGET_DIR}/"
    cp docs/interferencias.md "${TARGET_DIR}/" 2>/dev/null || true

    echo -e "${YELLOW}[3/4] Empaquetando en ${APP_NAME}-linux-${ARCH}.tar.gz...${NC}"
    cd "${BUILD_DIR}"
    tar -czf "../${OUTPUT_DIR}/${APP_NAME}-linux-${ARCH}.tar.gz" "${APP_NAME}-linux-${ARCH}"
    cd ..
    
    echo -e "${GREEN}Artefacto linux-${ARCH} creado.${NC}"
done

echo -e "\n${YELLOW}[4/4] Limpiando...${NC}"
rm -rf "$BUILD_DIR"

echo -e "\n${BLUE}====================================================${NC}"
echo -e "${GREEN} ¡Releases creados en la carpeta 'release/'!         ${NC}"
echo -e "${BLUE}====================================================${NC}"
ls -lh release/
