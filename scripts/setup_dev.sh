#!/usr/bin/env bash
set -e

# ==============================================================================
# setup_dev.sh
# Script completo para configurar el entorno de desarrollo de RBot.
# Valida dependencias, descarga modelos, crea enlaces simbólicos y compila.
# ==============================================================================

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}====================================================${NC}"
echo -e "${BLUE}    Configurando Entorno de Desarrollo de RBot      ${NC}"
echo -e "${BLUE}====================================================${NC}"

# 1. Asegurarnos de que estamos en la raíz del proyecto
if [ ! -d "skills" ] || [ ! -d "mcp" ]; then
    echo -e "${RED}Error: Debes ejecutar este script desde la raíz del proyecto.${NC}"
    exit 1
fi

PROJECT_DIR="$(pwd)"
DATA_DIR="${HOME}/.local/share/rbot"
CONFIG_DIR="${HOME}/.config/rbot"

# 2. Validar dependencias del sistema recomendadas 
echo -e "\n${YELLOW}[1/5] Verificando dependencias de desarrollo...${NC}"
MISSING_DEPS=()

for cmd in go curl arecord whisper-cli; do
    if ! command -v "$cmd" &> /dev/null; then
        MISSING_DEPS+=("$cmd")
    fi
done

if ! command -v "piper" &> /dev/null && ! command -v "piper-tts" &> /dev/null; then
    MISSING_DEPS+=("piper/piper-tts")
fi

if [ ${#MISSING_DEPS[@]} -ne 0 ]; then
    echo -e "${RED}Aviso: Faltan las siguientes herramientas en tu sistema: ${MISSING_DEPS[*]}${NC}"
    echo -e "${YELLOW}Por favor, asegúrate de instalarlas para que todo funcione. Revisa 'docs/dependencies.md'.${NC}"
else
    echo -e "${GREEN}¡Todas las herramientas esenciales de desarrollo están presentes!${NC}"
fi

# 3. Crear las estructuras de directorios del sistema
echo -e "\n${YELLOW}[2/5] Creando directorios base en el sistema...${NC}"
mkdir -p "${DATA_DIR}/models/piper"
mkdir -p "${DATA_DIR}/models/whisper"
mkdir -p "${CONFIG_DIR}"

# 3. Validar que las carpetas de modelos existan en el repositorio
echo -e "\n${YELLOW}[3/5] Verificando modelos locales de IA en el repositorio...${NC}"

if [ ! -d "voices" ] || [ ! -d "models" ]; then
    echo -e "${RED}Error: No se encontraron las carpetas 'voices' y 'models' en tu proyecto.${NC}"
    echo -e "${RED}Asegúrate de que existan y contengan los archivos .onnx y .bin requeridos.${NC}"
    exit 1
else
    echo -e "${GREEN}Carpetas de modelos detectadas en el repositorio.${NC}"
fi

# 4. Crear los Enlaces Simbólicos (La gran mejora para desarrollo)
echo -e "\n${YELLOW}[4/5] Configurando enlaces simbólicos (Symlinks)...${NC}"

# Eliminar versiones físicas anteriores si existen para evitar conflictos
rm -rf "${DATA_DIR}/skills"
rm -rf "${DATA_DIR}/models/piper"
rm -rf "${DATA_DIR}/models/whisper"
rm -f "${CONFIG_DIR}/mcp_config.json"
rm -f "${CONFIG_DIR}/rbot.yaml"

# Crear estructura contenedora si no existe
mkdir -p "${DATA_DIR}/models"

# Crear enlaces usando la ruta absoluta del proyecto
ln -s "${PROJECT_DIR}/skills" "${DATA_DIR}/skills"
ln -s "${PROJECT_DIR}/voices" "${DATA_DIR}/models/piper"
ln -s "${PROJECT_DIR}/models" "${DATA_DIR}/models/whisper"
ln -s "${PROJECT_DIR}/mcp/mcp_config.json" "${CONFIG_DIR}/mcp_config.json"

if [ -f "${PROJECT_DIR}/config/rbot.yaml" ]; then
    ln -s "${PROJECT_DIR}/config/rbot.yaml" "${CONFIG_DIR}/rbot.yaml"
    echo -e "${GREEN}✓ Symlink de rbot.yaml creado.${NC}"
fi

echo -e "${GREEN}✓ Symlink de Skills creado.${NC}"
echo -e "${GREEN}✓ Symlink de Modelos creado (Piper -> voices, Whisper -> models).${NC}"
echo -e "${GREEN}✓ Symlink de MCP creado.${NC}"

# 6. Compilar binario de desarrollo
echo -e "\n${YELLOW}[5/5] Compilando el binario local para pruebas...${NC}"
mkdir -p bin
go build -o bin/rbot cmd/main.go
echo -e "${GREEN}¡Binario compilado con éxito en 'bin/rbot'!${NC}"

echo -e "\n${BLUE}====================================================${NC}"
echo -e "${GREEN} ¡Entorno de desarrollo configurado con éxito!      ${NC}"
echo -e "${BLUE}====================================================${NC}"
echo -e "Puedes probar tu código local compilado usando:"
echo -e "  ${YELLOW}./bin/rbot voice${NC}"
echo -e "o ejecutándolo directamente sin compilar:"
echo -e "  ${YELLOW}go run cmd/main.go voice${NC}"
