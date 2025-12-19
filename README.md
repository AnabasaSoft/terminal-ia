# 🤖 Terminal Aumentada por IA 

[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://golang.org)
[![Ollama](https://img.shields.io/badge/Ollama-Framework-lightgrey.svg)](https://ollama.com/)
[![Licencia](https://img.shields.io/badge/Licencia-Propietaria-red.svg)](#-licencia)

Un shell interactivo en Go que utiliza el poder de los modelos de Ollama para convertir lenguaje natural en comandos de terminal, directamente en tu flujo de trabajo.

![Captura de pantalla](https://github.com/danitxu79/terminal-ia/blob/master/Captura%2001.png)
![Captura de pantalla 2](https://github.com/danitxu79/terminal-ia/blob/master/Captura%2002.png)

---

## 💡 Características Principales

Ejecución Multi-Comando (¡Nuevo!): La aplicación ahora puede ejecutar sugerencias de la IA que contengan múltiples comandos separados por punto y coma (;) o saltos de línea. Esto permite a la IA sugerir scripts y pipes complejos (ej. git pull; go build; ./mi_app) que se ejecutan secuencialmente.

Contexto de Archivos Local: La IA escanea automáticamente los archivos y directorios más relevantes de tu directorio de trabajo actual (CWD) e inyecta esa información en el prompt de sistema. Esto hace que las sugerencias de comandos sean contextuales y específicas (ej. si tienes un archivo data.json y pides /dame el contenido, la IA sugerirá directamente cat data.json).

Historial Semántico: Usa /buscar <intención> (ej. /buscar reiniciar el servidor) para encontrar comandos en tu historial basándote en el significado, no en el texto exacto. El sistema utiliza embeddings para encontrar el comando más relevante que hayas ejecutado con éxito en el pasado.

Chat con Memoria: El modo /chat <pregunta> ahora recuerda el contexto de tu conversación. Puedes hacer preguntas de seguimiento y la IA recordará lo que se dijo antes. Usa /reset para limpiar la memoria del chat.

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
| `/config` | Menú interactivo para cambiar modelo, modo auto y limpiar historiales. |
| `/reset` | Limpia el historial de la conversación de `/chat`. |
| `/traducir <idioma> <texto>` | Traduce un texto (ej. `/traducir fr hola`). |
| `/model` | Vuelve a mostrar el menú de selección de modelos. |
| `/ask` | Desactiva el modo de auto-ejecución. |
| `/help` | Muestra el menú de ayuda. |
| `cd <directorio>` | Cambia de directorio (manejado internamente). |
| `exit` o `Ctrl+D` | Cierra la terminal de IA. |


## 📜 Licencia

Este proyecto se ofrece bajo un modelo de licenciamiento dual:

### 1\. Licencia Pública (LGPLv3)

Este software está disponible bajo la **GNU Lesser General Public License v3.0 (LGPLv3)**.

Puedes usarlo libremente de acuerdo con los términos de la LGPLv3, lo cual es ideal para proyectos de código abierto. En resumen, esto significa que si usas esta biblioteca (especialmente si la modificas), debes cumplir con las obligaciones de la LGPLv3, como publicar el código fuente de tus modificaciones a esta biblioteca y permitir que los usuarios la reemplacen.

Puedes encontrar el texto completo de la licencia en el archivo `LICENSE` de este repositorio.

### 2\. Licencia Comercial (Privativa)

Si los términos de la LGPLv3 no se ajustan a tus necesidades, ofrezco una licencia comercial alternativa.

Necesitarás una licencia comercial si, por ejemplo:

  * Deseas incluir el código en un software propietario (código cerrado) sin tener que publicar tus modificaciones.
  * Necesitas enlazar estáticamente (static linking) la biblioteca con tu aplicación propietaria.
  * Prefieres no estar sujeto a las obligaciones y restricciones de la LGPLv3.

La licencia comercial te otorga el derecho a usar el código en tus aplicaciones comerciales de código cerrado sin las restricciones de la LGPLv3, a cambio de una tarifa.

Para adquirir una licencia comercial o para más información, por favor, pónte en contacto conmigo:


## ✉️ Contacto

Creado por **Daniel Serrano Armenta**

* `dani.eus79@gmail.com`
* Encuéntrame en GitHub: `@danitxu79`
* Portafolio: `https://danitxu79.github.io/`


<div align="center">
  <br/>
  <p><code>>_ sudo buy-me-a-coffee --theme=dark --force</code></p>
  <a href="https://www.buymeacoffee.com/danitxu" target="_blank">
    <img src="https://cdn.buymeacoffee.com/buttons/v2/default-black.png" alt="Buy Me A Coffee" style="height: 50px !important;width: 180px !important; box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;-webkit-box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;">
  </a>
  <br/>
</div>
