# 🤖 Terminal Aumentada por IA 

[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://golang.org)
[![Ollama](https://img.shields.io/badge/Ollama-Framework-lightgrey.svg)](https://ollama.com/)
[![Licencia](https://img.shields.io/badge/Licencia-Propietaria-red.svg)](#-licencia)

Un shell interactivo en Go que utiliza el poder de los modelos de Ollama para convertir lenguaje natural en comandos de terminal, directamente en tu flujo de trabajo.

![Captura de pantalla](https://github.com/danitxu79/terminal-ia/blob/master/Captura%2001.png)
![Captura de pantalla 2](https://github.com/danitxu79/terminal-ia/blob/master/Captura%2002.png)

---

## 💡 Características Principales

Historial Semántico (¡Nuevo!): Usa /buscar <intención> (ej. /buscar reiniciar el servidor) para encontrar comandos en tu historial basándote en el significado, no en el texto exacto. El sistema utiliza embeddings para encontrar el comando más relevante que hayas ejecutado con éxito en el pasado.

Chat con Memoria (¡Nuevo!): El modo /chat <pregunta> ahora recuerda el contexto de tu conversación. Puedes hacer preguntas de seguimiento y la IA recordará lo que se dijo antes. Usa /reset para limpiar la memoria del chat.

Depuración Inteligente: Si un comando de shell falla, la IA lo analizará automáticamente y te explicará la causa del error y cómo solucionarlo.

Traducción de Comandos: Escribe /<tu consulta> (ej. /encontrar archivos .log) y la IA generará el comando de shell.

Traducción Rápida: Usa /traducir <idioma> <texto> para traducciones instantáneas (ej. /traducir en hola).

Ejecución Segura: Confirma cada comando sugerido por la IA con un simple [s/N/X].

Modo Auto-Ejecución: Activa el modo de "confianza" (X) para ejecutar comandos automáticamente (se desactiva con /ask).

Selector de Modelos Dinámico: Cambia de modelo de IA (llama3, codellama, etc.) en cualquier momento con el comando /model.

Interfaz Pulida: Logos dinámicos, un shell con historial (flechas arriba/abajo), autocompletado de comandos/rutas y output de ls coloreado.

Cancelación de Stream: Presiona Ctrl+C mientras la IA responde en modo /chat para cancelar la respuesta.


## 🚀 Instalación (Recomendado para Linux)

Este método usa el script de instalación para configurar todo automáticamente (Ollama + `terminal-ia`).

1.  **Clona el repositorio:**
    ```bash
    git clone [https://github.com/danitxu79/terminal-ia.git](https://github.com/danitxu79/terminal-ia.git)
    cd terminal-ia
    ```

2.  **(Opcional) Compila el binario:**
    *El repositorio ya incluye un binario pre-compilado (`terminal-ia`) para Linux x64.*
    *Si prefieres compilarlo tú mismo (o estás en otra arquitectura), asegúrate de tener Go (v1.20+) y ejecuta:*
    ```bash
    go build -o terminal-ia main.go
    ```

3.  **Ejecuta el script de instalación:**
    *El script pedirá tu contraseña `sudo` para instalar Ollama (añadiendo su repositorio oficial) y mover el binario `terminal-ia` a `/usr/local/bin`.*
    ```bash
    chmod +x install.sh
    sudo ./install.sh
    ```
    *El script te pedirá que elijas un modelo de IA (ej. `llama3`) para descargar.*

4.  **¡Listo!**
    Cierra tu terminal actual, abre una **nueva** terminal y escribe `terminal-ia` para empezar.

## 🛠️ Instalación (para Desarrolladores)

Si ya tienes Ollama y solo quieres ejecutar el código fuente:

1.  **Clona el repositorio:**
    ```bash
    git clone [https://github.com/danitxu79/terminal-ia.git](https://github.com/danitxu79/terminal-ia.git)
    cd terminal-ia
    ```
2.  **Instala dependencias:**
    ```bash
    go mod tidy
    ```
3.  **Ejecuta:**
    ```bash
    go run main.go
    ```

## ⌨️ Comandos Especiales

| Comando | Acción |
| :--- | :--- |
| `/<petición>` | Envía una consulta de shell a la IA (ej. `/listar archivos .go`). |
| `/buscar <intención> ` | Busca en el historial semántico (ej. `/buscar contar archivos go`). |
| `/chat <pregunta>` | Inicia una conversación de chat (ej. `/chat ¿qué es Docker?`). |
| `/reset` | Limpia el historial de la conversación de `/chat`. |
| `/traducir <idioma> <texto>` | Traduce un texto (ej. `/traducir fr hola`). |
| `/model` | Vuelve a mostrar el menú de selección de modelos. |
| `/ask` | Desactiva el modo de auto-ejecución. |
| `/help` | Muestra el menú de ayuda. |
| `cd <directorio>` | Cambia de directorio (manejado internamente). |
| `exit` o `Ctrl+D` | Cierra la terminal de IA. |

## 📜 Licencia

Copyright (c) 2025 Daniel Serrano Armenta. Todos los derechos reservados.

Este proyecto es **Freeware**. Se te permite usar y distribuir este software gratuitamente para fines no comerciales. No se permite la modificación, reventa o uso comercial sin el permiso explícito del autor.

Ver el archivo `LICENSE` para más detalles.

## ✉️ Contacto

Creado por **Daniel Serrano Armenta**

* `dani.eus79@gmail.com`
* Encuéntrame en GitHub: `@danitxu79`
