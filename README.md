# 🤖 Terminal Aumentada por IA

[![Go Version](https://img.shields.io/badge/Go-1.20+-blue.svg)](https://golang.org)
[![Ollama](https://img.shields.io/badge/Ollama-Framework-lightgrey.svg)](https://ollama.com/)
[![Licencia](https://img.shields.io/badge/Licencia-Propietaria-red.svg)](#-licencia)

Un shell interactivo en Go que utiliza el poder de los modelos de Ollama para convertir lenguaje natural en comandos de terminal, directamente en tu flujo de trabajo.

![https://github.com/danitxu79/terminal-ia/blob/master/Captura%2001.png](httpsbutton)


---

## 💡 Características Principales

* **Traducción de Lenguaje Natural:** Escribe `// <tu consulta>` (ej. `// encontrar todos los archivos .log de más de 100MB`) y la IA generará el comando de shell por ti.
* **Ejecución Segura:** Confirma cada comando sugerido por la IA con un simple `[s/N/X]`.
* **Modo Auto-Ejecución:** Activa el modo de "confianza" (`X`) para ejecutar comandos automáticamente (se desactiva con `//ask`).
* **Selector de Modelos Dinámico:** Cambia de modelo de IA (`llama3`, `codellama`, `phi3`, etc.) en cualquier momento con el comando `//model`.
* **Arranque Rápido:** Calienta el modelo seleccionado al inicio para respuestas instantáneas.
* **Interfaz Pulida:** Logos dinámicos en degradado de color para cada familia de modelos.
* **Shell Integrado:** Maneja comandos `cd` internamente y ejecuta todos los demás comandos de `bash` de forma nativa.

## 📋 Prerrequisitos

Antes de empezar, asegúrate de tener:

1.  **Go (v1.20+)** instalado.
2.  **Ollama** instalado y ejecutándose en segundo plano.
3.  **Modelos Descargados:** Asegúrate de tener los modelos que quieras usar (ej. `ollama pull codellama`, `ollama pull llama3`).

## 🚀 Cómo Empezar

1.  **Clona el repositorio:**
    ```bash
    git clone [https://github.com/](https://github.com/danitxu79/terminal-ia].git
    cd [terminal-ia]
    ```

2.  **Instala las dependencias de Go:**
    ```bash
    go mod tidy
    ```

3.  **Ejecuta la aplicación:**
    ```bash
    go run main.go
    ```
    (El programa te guiará para seleccionar un modelo al inicio).

## ⌨️ Comandos Especiales

| Comando | Acción |
| :--- | :--- |
| `// <tu consulta>` | Envía una consulta al modelo de IA seleccionado. |
| `//model` | Vuelve a mostrar el menú de selección de modelos. |
| `//ask` | Desactiva el modo de auto-ejecución. |
| `cd <directorio>` | Cambia de directorio (manejado internamente). |
| `exit` o `Ctrl+D` | Cierra la terminal de IA. |

## 📜 Licencia

Copyright (c) 2025 [Tu Nombre]. Todos los derechos reservados.

Este proyecto es **Freeware**. Se te permite usar y distribuir este software gratuitamente para fines no comerciales. No se permite la modificación, reventa o uso comercial sin el permiso explícito del autor.

Ver el archivo `LICENSE` para más detalles.

## ✉️ Contacto

Creado por **[Tu Nombre]**

* `dani.eus79@gmail.com`
* Encuéntrame en GitHub: `@[danitxu79]`
