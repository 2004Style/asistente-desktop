#!/usr/bin/env bash

# Script para configurar modelos de RBot y compilar el binario en la carpeta bin/
# Autor: Antigravity

set -e # Terminar inmediatamente si algún comando falla

# Colores para salida en consola
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # Sin color

echo -e "${BLUE}====================================================${NC}"
echo -e "${BLUE}       Configuración y Compilación de RBot          ${NC}"
echo -e "${BLUE}====================================================${NC}"

# 1. Validar dependencias del sistema recomendadas
echo -e "\n${YELLOW}[1/4] Verificando dependencias del sistema...${NC}"
MISSING_DEPS=()

for cmd in go wget curl rec arecord whisper-cli piper-tts piper; do
    if ! command -v "$cmd" &> /dev/null; then
        if [ "$cmd" = "piper-tts" ] || [ "$cmd" = "piper" ]; then
            # Piper puede llamarse de ambas formas, validar al menos uno
            continue
        fi
        MISSING_DEPS+=("$cmd")
    fi
done

# Validación especial para Piper
if ! command -v piper &> /dev/null && ! command -v piper-tts &> /dev/null; then
    MISSING_DEPS+=("piper/piper-tts")
fi

if [ ${#MISSING_DEPS[@]} -ne 0 ]; then
    echo -e "${YELLOW}Aviso: Faltan las siguientes herramientas en tu sistema: ${MISSING_DEPS[*]}${NC}"
    echo -e "${YELLOW}Por favor, asegúrate de instalarlas después. Revisa 'docs/dependencies.md' para detalles.${NC}"
else
    echo -e "${GREEN}¡Todas las herramientas esenciales del sistema están presentes!${NC}"
fi

# 2. Descargar modelos de IA y configurar directorios locales
echo -e "\n${YELLOW}[2/4] Configurando directorios y descargando modelos locales...${NC}"
mkdir -p voices models

# Crear directorio local para habilidades (skills) y copiar las del repositorio
echo -e "${BLUE}Configurando habilidades locales en ~/.local/share/rbot/skills...${NC}"
mkdir -p ~/.local/share/rbot/skills
cp -r skills/* ~/.local/share/rbot/skills/ 2>/dev/null || true

# Descargar modelo Piper ONNX
PIPER_MODEL="voices/es_ES-davefx-medium.onnx"
PIPER_CONFIG="voices/es_ES-davefx-medium.onnx.json"
if [ ! -f "$PIPER_MODEL" ]; then
    echo -e "${BLUE}Descargando modelo de voz Piper (es_ES-davefx-medium)...${NC}"
    wget -O "$PIPER_MODEL" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx"
else
    echo -e "${GREEN}El modelo de voz Piper ya existe en '$PIPER_MODEL'.${NC}"
fi

if [ ! -f "$PIPER_CONFIG" ]; then
    echo -e "${BLUE}Descargando configuración de voz Piper...${NC}"
    wget -O "$PIPER_CONFIG" "https://huggingface.co/rhasspy/piper-voices/resolve/v1.0.0/es/es_ES/davefx/medium/es_ES-davefx-medium.onnx.json"
else
    echo -e "${GREEN}La configuración de voz Piper ya existe en '$PIPER_CONFIG'.${NC}"
fi

# Descargar modelo Whisper GGML (ggml-tiny.bin es el valor por defecto configurado y es 3.5x más rápido)
WHISPER_MODEL="models/ggml-tiny.bin"
if [ ! -f "$WHISPER_MODEL" ]; then
    echo -e "${BLUE}Descargando modelo de transcripción Whisper (ggml-tiny)...${NC}"
    wget -O "$WHISPER_MODEL" "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin"
else
    echo -e "${GREEN}El modelo Whisper GGML ya existe en '$WHISPER_MODEL'.${NC}"
fi

# 3. Compilar RBot
echo -e "\n${YELLOW}[3/4] Creando directorio bin/ y compilando RBot...${NC}"
mkdir -p bin

# Compilar
go build -o bin/rbot cmd/main.go
echo -e "${GREEN}¡Binario compilado con éxito en 'bin/rbot'!${NC}"

# 4. Finalizado
echo -e "\n${BLUE}====================================================${NC}"
echo -e "${GREEN} ¡Listo! RBot está preparado para ejecutarse.         ${NC}"
echo -e "${BLUE}====================================================${NC}"
echo -e "Puedes iniciar el motor de voz ejecutando:"
echo -e "  ${YELLOW}./bin/rbot voice${NC}"
echo -e "===================================================="
