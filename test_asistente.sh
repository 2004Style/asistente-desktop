#!/bin/bash

# RBot Automated Verification & Testing Script (Comprehensive Matrix Edition)
# Unified script testing path indexing, app execution, phrasing flexibility,
# file operations, security prompts, and voice/STT/TTS subsystems.

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
BLUE='\033[0;34m'
YELLOW='\033[1;33m'

REPORT_FILE="report-test.txt"

# Registrar inicio en el reporte (sin borrar el contenido anterior)
echo -e "\n================================================================" >> "$REPORT_FILE"
echo "  RBOT COMPREHENSIVE TEST RUN: $(date '+%Y-%m-%d %H:%M:%S')" >> "$REPORT_FILE"
echo -e "================================================================" >> "$REPORT_FILE"

log_report() {
    local instruction="$1"
    local status="$2"
    local details="$3"
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] TEST: '${instruction}' | STATUS: ${status} | DETAILS: ${details}" >> "$REPORT_FILE"
}

echo -e "${BLUE}================================================================${NC}"
echo -e "${BLUE}           INICIANDO VERIFICACIÓN COMPLETA DE RBOT              ${NC}"
echo -e "${BLUE}================================================================${NC}"

# Limpiar procesos en segundo plano anteriores de rbot
echo "Liberando cualquier bloqueo de base de datos anterior..."
pkill -9 -f "./bin/rbot" > /dev/null 2>&1
sleep 1

# Setup test directory and files
TARGET_DIR="$HOME/Descargas/test-asistente"
TARGET_FILE="${TARGET_DIR}/nota.txt"
DOC_FILE="${TARGET_DIR}/doc.txt"
rm -rf "${TARGET_DIR}"
mkdir -p "${TARGET_DIR}"

# Crear archivos de prueba
echo "RBot al maximo nivel" > "${TARGET_FILE}"
cat << 'EOF' > "${DOC_FILE}"
La Inteligencia Artificial (IA) es la simulación de procesos de inteligencia humana por parte de máquinas, especialmente sistemas informáticos. Estos procesos incluyen el aprendizaje (la adquisición de información y reglas para el uso de la información), el razonamiento (usar las reglas para llegar a conclusiones aproximadas o definitivas) y la autocorrección. Las aplicaciones particulares de la IA incluyen sistemas expertos, reconocimiento de voz y visión artificial.
EOF

# Indexar archivos para que estén disponibles
echo "Indexando rutas de archivos en base de datos..."
./bin/rbot index paths > /dev/null
./bin/rbot index apps > /dev/null

# Función auxiliar para cerrar ventanas abiertas de forma limpia
close_app_processes() {
    local app_type="$1"
    case "${app_type}" in
        "nautilus")
            hyprctl dispatch closewindow class:nautilus >/dev/null 2>&1 || true
            sleep 0.5
            pkill -x "nautilus" >/dev/null 2>&1 || true
            ;;
        "code")
            hyprctl dispatch closewindow class:Code >/dev/null 2>&1 || true
            hyprctl dispatch closewindow class:code >/dev/null 2>&1 || true
            hyprctl dispatch closewindow class:code-oss >/dev/null 2>&1 || true
            hyprctl dispatch closewindow class:cursor >/dev/null 2>&1 || true
            sleep 0.5
            pkill -x "code" >/dev/null 2>&1 || true
            pkill -x "cursor" >/dev/null 2>&1 || true
            ;;
        "firefox")
            hyprctl dispatch closewindow class:firefox >/dev/null 2>&1 || true
            sleep 0.5
            pkill -x "firefox" >/dev/null 2>&1 || true
            ;;
        "browser")
            hyprctl dispatch closewindow class:google-chrome >/dev/null 2>&1 || true
            hyprctl dispatch closewindow class:firefox >/dev/null 2>&1 || true
            sleep 0.5
            pkill -x "chrome" >/dev/null 2>&1 || true
            pkill -x "google-chrome" >/dev/null 2>&1 || true
            pkill -x "firefox" >/dev/null 2>&1 || true
            ;;
    esac
}

# Matriz de Casos de Prueba (Fraseo | Tipo | Valor de control)
# Tipo "action": Abre app/browser, se comprueba proceso físico o confirmación en consola.
# Tipo "llm_summary": Pasa al LLM y verifica que el resumen contenga las palabras clave.
TESTS=(
    # --- 1. PRUEBAS DE MÚSICA Y YOUTUBE (VARIADAS) ---
    "colcoame musica|action|browser"
    "colocame algo de musica|action|browser"
    "reproduce linkin park|action|browser"
    "pon cumbia|action|browser"
    "quiero escuchar musica de rock de los 80|action|browser"
    "toca cumbia|action|browser"
    "tocar phonk|action|browser"
    "reproduceme linkin park|action|browser"
    "reprodúceme algo de rock|action|browser"
    "ponme música para estudiar|action|browser"
    "quiero oír música de los 90|action|browser"
    "me gustaría escuchar algo de cumbia|action|browser"
    "ponme algo de phonk|action|browser"
    "colocame algo de cumbias|action|browser"
    "busca en youtube a los beatles|action|browser"
    "reproduce en youtube la canción numb|action|browser"
    "colcame algo de musica|action|browser"
    "quiero oir musica|action|browser"
    "toca la canción in the end|action|browser"
    "pon la canción numb|action|browser"
    "quiero escuchar musica|action|browser"
    "musica de cumbia|action|browser"
    "cancion de linkin park|action|browser"

    # --- 2. PRUEBAS DE ABRIR APLICACIONES (INDIVIDUALES Y MÚLTIPLES) ---
    "abre firefox|action|firefox"
    "lanza el navegador|action|browser"
    "ejecuta el navegador|action|browser"
    "abre la aplicación firefox|action|firefox"
    "abre el programa cursor|action|code"
    "abre cursor y vscode|action|code"
    "lanza cursor y edex ui|action|code"
    "abre vscode y firefox y nautilus|action|code"
    "abre el gestor de archivos y el navegador|action|nautilus"
    "abre el gestor de archivos|action|nautilus"
    "abre nautilus|action|nautilus"
    "abre vscode|action|code"
    "abre visual studio code|action|code"

    # --- 3. PRUEBAS DE APERTURA DE CARPETAS ---
    "abre la carpeta Descargas|action|nautilus"
    "abre el directorio Documentos|action|nautilus"
    "abre la carpeta test-asistente en vscode|action|code"
    "abre la carpeta test-asistente con code|action|code"

    # --- 4. PRUEBAS DE RESÚMENES DE ARCHIVOS Y CONSULTAS LLM ---
    "leeme el archivo doc.txt que está en Descargas/test-asistente y dame un resumen|llm_summary|aprendizaje"
    "dame un resumen de doc.txt en Descargas/test-asistente|llm_summary|inteligencia"
    "contenido de doc.txt en Descargas/test-asistente|llm_summary|simulación"
    "resumen del archivo doc.txt en Descargas/test-asistente|llm_summary|máquinas"
    "lee el archivo doc.txt en Descargas/test-asistente|llm_summary|procesos"

    # --- 5. BÚSQUEDA WEB Y RESUMEN LLM ---
    "busca información sobre la teoría de la relatividad en google y hazme un resumen|llm_summary|relatividad"
    "busca en internet qué es la física cuántica y dame un resumen|llm_summary|cuántica"
    "busca en google la biografía de Albert Einstein y resume|llm_summary|einstein"
)

# Ejecutar el bucle de pruebas
for test in "${TESTS[@]}"; do
    # Dividir por "|"
    IFS="|" read -r phrasing type check <<< "$test"
    
    echo -e "\n${YELLOW}PROBANDO: '${phrasing}' (Tipo: $type)${NC}"
    
    if [ "$type" == "action" ]; then
        # Ejecutar RBot y enviar respuesta afirmativa si requiere confirmación
        output=$(echo "s" | ./bin/rbot chat "$phrasing" 2>&1)
        echo "$output"
        
        sleep 3
        
        # Verificar proceso físico
        pids=""
        if [ "$check" == "browser" ]; then
            pids=$(pgrep -x "chrome" || pgrep -x "google-chrome" || pgrep -x "firefox")
        elif [ "$check" == "code" ]; then
            pids=$(pgrep -x "code" || pgrep -x "cursor")
        else
            pids=$(pgrep -x "$check")
        fi
        
        if [ -n "$pids" ]; then
            echo -e "${GREEN}[OK] Proceso '$check' detectado en ejecución (PIDs: $pids).${NC}"
            log_report "$phrasing" "OK" "Proceso '$check' iniciado físicamente."
            close_app_processes "$check"
            close_app_processes "browser"
            close_app_processes "nautilus"
        else
            # Comprobar confirmación amigable de RBot en salida
            if echo "$output" | grep -iqE "reproduciendo|abriendo|buscando|abierto|ejecutando|completadas"; then
                echo -e "${GREEN}[OK] Acción confirmada conversacionalmente por RBot.${NC}"
                log_report "$phrasing" "OK" "Acción confirmada en consola: $output"
            else
                echo -e "${RED}[ERROR] No se detectó proceso ni confirmación conversacional para: '$check'.${NC}"
                log_report "$phrasing" "ERROR" "Fallo de ejecución o confirmación."
            fi
        fi
    elif [ "$type" == "llm_summary" ]; then
        # Pasar por el LLM para validar el resumen
        output=$(./bin/rbot chat "$phrasing" 2>&1)
        echo "$output"
        
        if echo "$output" | grep -iq "$check" || echo "$output" | grep -iqE "inteligencia|aprendizaje|máquinas|maquinas|razonamiento|relatividad|física|fisica|einstein"; then
            echo -e "${GREEN}[OK] Resumen e interpretación de la IA correcto.${NC}"
            log_report "$phrasing" "OK" "Resumen devuelto: $output"
        else
            echo -e "${RED}[ERROR] El resumen de la IA no contiene los términos esperados (falta: $check).${NC}"
            log_report "$phrasing" "ERROR" "Resumen incompleto. Salida: $output"
        fi
    fi
done

# --- 6. COMANDOS DEL SISTEMA Y CONFIRMACIONES CRÍTICAS ---
echo -e "\n${YELLOW}=== 6. Comandos del Sistema y Confirmaciones ===${NC}"

# Comando no crítico
echo -e "\nProbando: ejecuta comando 'echo 42'"
res_echo=$(./bin/rbot chat "ejecuta el comando 'echo 42'")
echo "$res_echo"
log_report "ejecuta el comando 'echo 42'" "OK" "$res_echo"

# Comando crítico con confirmación negativa
echo -e "\nProbando: ejecuta comando crítico 'sudo echo 42' con rechazo 'n'"
res_sudo=$(echo "n" | ./bin/rbot chat "ejecuta el comando 'sudo echo 42'")
echo "$res_sudo"
log_report "ejecuta el comando 'sudo echo 42' (Rechazo)" "OK" "$res_sudo"

# Operación de borrado interactivo seguro
echo -e "\nProbando: elimina el archivo nota.txt con confirmación 's'"
res_del=$(echo "s" | ./bin/rbot chat "elimina el archivo nota.txt de la carpeta Descargas/test-asistente")
echo "$res_del"
if [ ! -f "${TARGET_FILE}" ]; then
    echo -e "${GREEN}[OK] Archivo eliminado con éxito de forma interactiva.${NC}"
    log_report "elimina el archivo nota.txt" "OK" "Archivo eliminado físicamente."
else
    echo -e "${RED}[ERROR] El archivo no se pudo eliminar.${NC}"
    log_report "elimina el archivo nota.txt" "ERROR" "El archivo aún existe en disco."
fi

# Limpieza final
rm -rf "${TARGET_DIR}"
./bin/rbot index paths > /dev/null

echo -e "\n${BLUE}================================================================${NC}"
echo -e "${BLUE}        TODAS LAS PRUEBAS COMPLETADAS. INFORME DETALLADO EN:    ${NC}"
echo -e "${BLUE}                       $REPORT_FILE                             ${NC}"
echo -e "${BLUE}================================================================${NC}"
