#!/bin/bash

# Script de instalación para Terminal-IA
# v17.4 - Creado por Daniel Serrano Armenta

# --- Colores para la salida ---
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # Sin color

echo -e "${GREEN}Iniciando la instalación de Terminal-IA (v17.4)...${NC}"

# --- 1. Forzar ejecución como root (sudo) ---
# Si el usuario no es root ($EUID -ne 0), vuelve a ejecutar este mismo script con sudo
if [ "$EUID" -ne 0 ]; then
  echo -e "${YELLOW}Se requieren privilegios de administrador. Solicitando sudo...${NC}"
  # "$0" es el nombre de este script, "$@" son todos los argumentos que se le pasaron
  exec sudo "$0" "$@"
fi

# --- Si llegamos aquí, ya somos root ---
echo -e "${GREEN}✔ Privilegios de administrador obtenidos.${NC}"

# --- 2. Comprobar que el binario 'terminal-ia' existe ---
BINARY_NAME="terminal-ia"
# Comprueba si el binario está en el mismo directorio que el script
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
BINARY_PATH="$SCRIPT_DIR/$BINARY_NAME"

if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}Error: No se encontró el binario '$BINARY_NAME' en la carpeta:${NC}"
    echo -e "$SCRIPT_DIR"
    echo -e "${YELLOW}Por favor, primero compila el programa con:${NC}"
    echo -e "go build -o $BINARY_NAME main.go"
    exit 1
fi
echo -e "${GREEN}✔ Binario '$BINARY_NAME' encontrado en '$SCRIPT_DIR'.${NC}"

# --- 3. Instalar Ollama (usando el script oficial que añade el repo) ---
echo -e "${YELLOW}Comprobando/Instalando Ollama...${NC}"
if ! command -v ollama &> /dev/null; then
    echo "Ollama no está instalado. Instalando desde el script oficial (esto añadirá el repositorio)..."

    # Comprobar si curl está instalado
    if ! command -v curl &> /dev/null; then
        echo -e "${YELLOW}curl no está instalado. Intentando instalar curl...${NC}"
        # Intentar instalar curl (funciona en Debian/Ubuntu y Arch)
        if command -v apt-get &> /dev/null; then
            apt-get update && apt-get install -y curl
        elif command -v pacman &> /dev/null; then
            pacman -Sy --noconfirm curl
        elif command -v dnf &> /dev/null; then
            dnf install -y curl
        else
            echo -e "${RED}Error: No se pudo instalar 'curl'. Por favor, instálalo manualmente.${NC}"
            exit 1
        fi
    fi

    # Instalar Ollama (el script oficial detecta el S.O. y añade el repo)
    curl -fsSL https://ollama.com/install.sh | sh
    echo -e "${GREEN}✔ Ollama instalado correctamente.${NC}"
else
    echo -e "${GREEN}✔ Ollama ya está instalado.${NC}"
fi

# --- 4. Instalar un Modelo LLM (Interactivo) ---
echo -e "${YELLOW}--- Instalación de Modelo LLM ---${NC}"
echo "Necesitas al menos un modelo para que 'terminal-ia' funcione."
echo "Recomendados: 'llama3' (chat general) o 'codellama' (código)."

# Bucle para asegurar que se elija un modelo
while true; do
    read -p "Introduce el nombre del modelo a descargar (ej: llama3): " MODEL_NAME
    if [ -z "$MODEL_NAME" ]; then
        echo -e "${RED}No has introducido ningún nombre. Por favor, escribe un modelo.${NC}"
    else
        break
    fi
done

echo -e "${YELLOW}Descargando '$MODEL_NAME'... (esto puede tardar)${NC}"
# Ejecuta ollama pull (el servicio ya debería estar corriendo)
ollama pull "$MODEL_NAME"
echo -e "${GREEN}✔ Modelo '$MODEL_NAME' descargado.${NC}"


# --- 5. Instalar el binario 'terminal-ia' ---
INSTALL_PATH="/usr/local/bin"
echo -e "${YELLOW}Instalando '$BINARY_NAME' en $INSTALL_PATH...${NC}"

# Copiar el binario
cp "$BINARY_PATH" "$INSTALL_PATH/$BINARY_NAME"
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: No se pudo copiar el binario a $INSTALL_PATH.${NC}"
    exit 1
fi

# Darle permisos de ejecución
chmod +x "$INSTALL_PATH/$BINARY_NAME"
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: No se pudo dar permisos de ejecución al binario.${NC}"
    exit 1
fi

# --- 6. Finalización ---
echo -e "\n${GREEN}--- ¡Instalación Completada! ---${NC}"
echo -e "Ahora puedes abrir una NUEVA terminal y ejecutar tu programa desde"
echo -e "cualquier lugar simplemente escribiendo:"
echo -e "\n  ${YELLOW}terminal-ia\n"
