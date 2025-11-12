// Copyright (c) 2025 Daniel Serrano Armenta. dani.eus79@gmail.com Todos los derechos reservados.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/peterh/liner"

	"github.com/charmbracelet/lipgloss"
	"github.com/ollama/ollama/api"
)

// --- Constantes del Programa ---
const (
	currentVersion       = "v25.0" // Persistencia de Chat + Embeddings Dedicados
	repoOwner            = "danitxu79"
	repoName             = "terminal-ia"
	historyFileName      = ".terminal_ia_history"
	embeddingHistoryFile = ".terminal_ia_embeddings.json"
	chatHistoryFile      = ".terminal_ia_chat_history.json"
	debugSystemPrompt    = "Eres un experto en depuración de comandos de Linux. Analiza el siguiente error de terminal (stderr), explica brevemente por qué ocurrió y proporciona una solución concisa que el usuario pueda copiar/pegar."
	embeddingModelName   = "nomic-embed-text" // Modelo dedicado para embeddings
)

// --- Estructuras y Variables Globales de Estilo ---
var (
	logoMap  map[string][]string
	colorMap map[string][]string

	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	cSystem = color.New(color.FgYellow).SprintFunc()
	cError  = color.New(color.FgRed, color.Bold).SprintFunc()
	cPrompt = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	cIA     = color.New(color.FgGreen).SprintFunc()
	cModel  = color.New(color.FgMagenta).SprintFunc()

	updateMessageChannel = make(chan string, 1)
	githubLatestVersion  = currentVersion

	// Historial de Chat y Semántico
	chatHistory []api.Message
	chatHistoryLock sync.Mutex // Mutex para proteger el chatHistory

	semanticHistory     []SemanticHistoryEntry
	semanticHistoryPath string
	semanticHistoryLock sync.Mutex // Mutex para proteger el acceso al historial

	// Variables de Ruta Globales (Corregidas)
	historyPath     string // Para el historial de liner
	chatHistoryPath string // Para el historial de chat
)

// Struct para Historial Semántico
type SemanticHistoryEntry struct {
	Command   string    `json:"command"`
	Embedding []float64 `json:"embedding"`
}

// Structs para APIs
type WttrWeatherDesc struct {
	Value string `json:"value"`
}
type WttrCurrentCondition struct {
	Temp_C      string            `json:"temp_C"`
	FeelsLikeC  string            `json:"FeelsLikeC"`
	WeatherDesc []WttrWeatherDesc `json:"weatherDesc"`
}
type WttrResponse struct {
	CurrentCondition []WttrCurrentCondition `json:"current_condition"`
	NearestArea      []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
		Country []struct {
			Value string `json:"value"`
		} `json:"country"`
	} `json:"nearest_area"`
}
type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

// clearScreen
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// loadLogos
func loadLogos() {
	file, err := os.Open("logos.json")
	if err != nil {
		fmt.Println(cError(fmt.Sprintf("Error: No se encontró logos.json. Saltando logos.")))
		logoMap = make(map[string][]string)
		return
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo leer logos.json: %v", err)))
	}
	if err := json.Unmarshal(bytes, &logoMap); err != nil {
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo parsear logos.json: %v", err)))
	}
}

// createColorMap
func createColorMap() {
	colorMap = map[string][]string{
		"llama":    {"#B721FF", "#21D4FD"},
		"mistral":  {"#FF8008", "#FFC837"},
		"gemma":    {"#007BFF", "#00C6FF"},
		"phi3":     {"#6A11CB", "#2575FC"},
		"deepseek": {"#1D2B64", "#F8CDDA"},
		"qwen":     {"#4E49E7", "#A849E7"},
		"gpt":      {"#74AA9C", "#2CB77F"},
		"default":  {"#FFFFFF", "#EAEAEA"},
	}
}

// getLogoKey
func getLogoKey(modelName string) string {
	lowerName := strings.ToLower(modelName)
	for key := range logoMap {
		if strings.Contains(lowerName, key) {
			return key
		}
	}
	return "default"
}

// printLogo
func printLogo(modelName string) {
	logoKey := getLogoKey(modelName)
	logoLines, ok := logoMap[logoKey]
	if !ok || len(logoLines) == 0 {
		return
	}
	colors, ok := colorMap[logoKey]
	if !ok {
		colors = colorMap["default"]
	}
	startColor, _ := colorful.Hex(colors[0])
	endColor, _ := colorful.Hex(colors[1])
	numLines := len(logoLines)
	for i, line := range logoLines {
		var t float64
		if numLines <= 1 {
			t = 1.0
		} else {
			t = float64(i) / float64(numLines-1)
		}
		interpolatedColor := startColor.BlendHcl(endColor, t)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(interpolatedColor.Hex()))
		fmt.Println(style.Render(line))
	}
}

// printHeader
func printHeader() {
	fmt.Println()
	fmt.Println(styleHeader.Render(fmt.Sprintf("  Terminal Aumentada por IA (%s)", currentVersion)))
	fmt.Println(styleHeader.Render("  Creado por: Daniel Serrano Armenta <dani.eus79@gmail.com>"))
	fmt.Println(styleHeader.Render("  Copyright (c) 2025 Daniel Serrano Armenta. Ver LICENSE para más detalles."))
	fmt.Println(styleHeader.Render(fmt.Sprintf("  Github: https://github.com/%s/%s", repoOwner, repoName)))
}

// --- printHelp ---
func printHelp() {
	fmt.Println()
	fmt.Println(cSystem("--- Ayuda: Comandos Disponibles ---"))
	fmt.Println(cPrompt("  /<petición> ") + cIA("- Pide un comando de shell (ej. /listar archivos .go)"))
	fmt.Println(cPrompt("  /buscar <intención> ") + cIA("- Busca en tu historial por significado (ej. /buscar reiniciar servidor)"))
	fmt.Println(cPrompt("  /chat <pregunta> ") + cIA("- Inicia una conversación de chat (ej. /chat ¿qué es Docker?)"))
	fmt.Println(cPrompt("  /reset       ") + cIA("- Limpia el historial de la conversación de /chat."))
	fmt.Println(cPrompt("  /tiempo <lugar>  ") + cIA("- Consulta el tiempo (sin API key) (ej. /tiempo Madrid)"))
	fmt.Println(cPrompt("  /traducir <idioma> <texto> ") + cIA("- Traduce un texto (ej. /traducir fr hola)"))
	fmt.Println(cPrompt("  /config      ") + cIA("- Menú de configuración: cambia modelo, modo auto, limpia historiales."))
	fmt.Println(cPrompt("  /model       ") + cIA("- Acceso directo: Muestra el selector de modelos."))
	fmt.Println(cPrompt("  /ask         ") + cIA("- Acceso directo: Desactiva el modo 'auto'."))
	fmt.Println(cPrompt("  /help        ") + cIA("- Muestra este menú de ayuda."))
	fmt.Println(cPrompt("  cd <dir>     ") + cIA("- Cambia el directorio actual (comando interno)."))
	fmt.Println(cPrompt("  exit / quit  ") + cIA("- Cierra la terminal de IA (también Ctrl+D)."))
	fmt.Println(cSystem("------------------------------------"))
	fmt.Println()
}

// --- Ollama Management Functions ---

// checkOllamaService intenta conectar y verificar que Ollama esté corriendo y accesible.
// Además, asegura que el modelo de Embeddings dedicado esté instalado.
func checkOllamaService(client *api.Client) error {
	maxAttempts := 3
	delay := 2 * time.Second

	// 1. Verificar la conexión al servicio Ollama
	for attempt := 0; attempt < maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.List(ctx)
		cancel()

		if err == nil {
			fmt.Println(cIA("IA> Ollama encontrado y activo."))

			// 2. Si hay conexión, procedemos a verificar los modelos esenciales.
			return ensureEssentialModels(client)
		}

		if attempt < maxAttempts-1 {
			fmt.Printf(cError("Advertencia: No se pudo conectar a Ollama (Intento %d/%d). Reintentando en %s...\n"), attempt+1, maxAttempts, delay)
			time.Sleep(delay)
		} else {
			return fmt.Errorf("fallo al conectar con Ollama después de %d intentos. Asegúrate de que el servicio esté corriendo (ej. 'ollama start' o 'ollama run'). Error: %v", maxAttempts, err)
		}
	}
	return nil
}

// ensureEssentialModels verifica que el modelo de Embeddings esté presente, descargándolo si es necesario.
func ensureEssentialModels(client *api.Client) error {
	ctx := context.Background()
	resp, err := client.List(ctx)
	if err != nil {
		return fmt.Errorf("fallo al listar modelos después de conectar: %v", err)
	}

	modelExists := false
	for _, model := range resp.Models {
		if strings.Contains(model.Name, embeddingModelName) {
			modelExists = true
			break
		}
	}

	if !modelExists {
		fmt.Println(cError(fmt.Sprintf("¡ADVERTENCIA! El modelo de Embeddings (%s) es esencial para /buscar y no está instalado.", embeddingModelName)))
		fmt.Println(cSystem(fmt.Sprintf("Intentando descargarlo automáticamente (ollama pull %s)...", embeddingModelName)))

		// Ejecutar el comando de descarga de Ollama
		cmd := exec.Command("ollama", "pull", embeddingModelName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("falló la descarga de %s. Por favor, ejecuta 'ollama pull %s' manualmente. Error: %v", embeddingModelName, embeddingModelName, err)
		}

		fmt.Println(cIA("IA> Descarga de Embeddings completa."))
	}

	return nil
}

// warmUpModel calienta el modelo de Chat (user selected) y el modelo de Embeddings.
func warmUpModel(client *api.Client, modelName string) {
	ctx := context.Background()

	// --- 1. Pre-calentar el endpoint 'generate' (Modelo de Chat) ---
	reqGen := &api.GenerateRequest{
		Model:     modelName, // Modelo de chat seleccionado por el usuario
		Prompt:    "hola",
		Stream:    new(bool),
	}
	genHandler := func(r api.GenerateResponse) error { return nil }

	if err := client.Generate(ctx, reqGen, genHandler); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Advertencia: Fallo al 'calentar' (generate - %s): %v", modelName, err)))
	}

	// --- 2. Pre-calentar el endpoint 'embeddings' (Modelo Dedicado) ---
	reqEmb := &api.EmbeddingRequest{
		Model:     embeddingModelName, // Modelo dedicado
		Prompt:    "hola",
	}

	if _, err := client.Embeddings(ctx, reqEmb); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Advertencia: Fallo al 'calentar' (embeddings - %s): %v", embeddingModelName, err)))
	}
}

// chooseModel (ACTUALIZADO: Incluye Descarga de Emergencia de CHAT y Filtro de Embeddings)
func chooseModel(client *api.Client, state *liner.State) string {
	fmt.Println(cSystem("Consultando modelos de Ollama disponibles..."))

	ctx := context.Background()
	resp, err := client.List(ctx)
	if err != nil {
		// Asumimos que checkOllamaService ya manejó la conexión.
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo listar los modelos de Ollama: %v", err)))
	}

	// --- Descarga de Emergencia para el Modelo de CHAT ---

	// Filtramos modelos de chat disponibles (excluyendo el de embeddings)
	var currentChatModels []api.ListModelResponse
	for _, model := range resp.Models {
		if !strings.Contains(model.Name, embeddingModelName) {
			currentChatModels = append(currentChatModels, model)
		}
	}

	if len(currentChatModels) == 0 {
		fmt.Println(cError("¡ADVERTENCIA! No se encontraron modelos de CHAT descargados."))
		fmt.Println(cSystem("Intentando descargar el modelo predeterminado 'llama3:8b' automáticamente..."))

		cmd := exec.Command("ollama", "pull", "llama3:8b")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Fatal(cError(fmt.Sprintf("Error fatal: Falló la descarga automática. Ejecuta 'ollama pull llama3:8b' manualmente. Error: %v", err)))
		}

		// Reintentar la lista de modelos para actualizar la UI
		resp, err = client.List(ctx)
		if err != nil {
			log.Fatal(cError("Error fatal: Fallo al re-listar modelos después de la descarga."))
		}

		// Volvemos a filtrar la lista actualizada
		currentChatModels = nil
		for _, model := range resp.Models {
			if !strings.Contains(model.Name, embeddingModelName) {
				currentChatModels = append(currentChatModels, model)
			}
		}

		if len(currentChatModels) == 0 {
			log.Fatal(cError("Error fatal: La descarga tuvo éxito, pero el modelo de CHAT sigue sin estar disponible. Reinicia la aplicación."))
		}

		fmt.Println(cIA("Descarga de CHAT completa. Modelos disponibles:"))
	}
	// --- Fin de la Descarga de Emergencia para CHAT ---


	fmt.Println(cSystem("--- Elige un modelo de IA ---"))
	fmt.Println(cSystem("------------------------------"))

	for i, model := range currentChatModels {
		fmt.Printf("%d: %s\n", i+1, model.Name)
	}
	fmt.Println(cSystem("------------------------------"))

	var choice int
	for {
		prompt := "Introduce el número del modelo: "
		input, err := state.Prompt(prompt)
		if err != nil {
			if err == io.EOF || err == liner.ErrPromptAborted {
				log.Fatal(cError("\nSelección cancelada. Saliendo."))
			}
			log.Fatal(cError(fmt.Sprintf("Error al leer la selección: %v", err)))
		}

		choice, err = strconv.Atoi(strings.TrimSpace(input))
		if err != nil || choice < 1 || choice > len(currentChatModels) {
			fmt.Println(cError("Selección inválida. Introduce un número de la lista."))
		} else {
			state.AppendHistory(input)
			break
		}
	}
	return currentChatModels[choice-1].Name
}

// saveHistory
func saveHistory(state *liner.State) {
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		// La ruta no existe o no se inicializó correctamente en main, la reconstruimos por seguridad.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al encontrar el home dir para guardar historial: %v", err)))
			return
		}
		historyPath = filepath.Join(home, historyFileName)
	}

	f, err := os.Create(historyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al crear archivo de historial: %v", err)))
		return
	}
	defer f.Close()

	if _, err := state.WriteHistory(f); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al escribir el historial: %v", err)))
	}
}

// checkVersion consulta la última versión en GitHub y la almacena.
func checkVersion() {
	client := &http.Client{Timeout: 3 * time.Second}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "terminal-ia-updater")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	if release.TagName != "" {
		// Normalizar versiones quitando la "v" inicial
		remoteVersion := strings.TrimPrefix(strings.ToLower(release.TagName), "v")
		localVersion := strings.TrimPrefix(strings.ToLower(currentVersion), "v")

		// 1. Almacenar la versión de GitHub (sin la 'v')
		githubLatestVersion = remoteVersion // <-- ¡ALMACENAR!

		// 2. Notificar si hay una actualización
		if remoteVersion != localVersion {
			msg := fmt.Sprintf("\n%s\n", cSystem(fmt.Sprintf(
				"  ¡Nueva versión %s disponible! (Estás en %s)",
									 release.TagName, currentVersion,
			)))
			updateMessageChannel <- msg
		}
	}
}

// --- main ---
func main() {
	loadLogos()
	createColorMap()
	clearScreen()

	go checkVersion()

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo crear el cliente de Ollama: %v", err)))
	}

	// --- Comprobación del servicio Ollama (Health Check) ---
	fmt.Println(cSystem("Verificando el servicio Ollama..."))
	if err := checkOllamaService(client); err != nil {
		log.Fatal(cError(fmt.Sprintf("\nError fatal: %v\n", err)))
	}
	// --- Fin Comprobación ---

	state := liner.NewLiner()
	defer state.Close()
	state.SetCtrlCAborts(true)

	// --- LÓGICA DE AUTO-COMPLETADO ---
	state.SetCompleter(func(line string) (c []string) {
		commands := []string{
			"/help",
			"/chat ",
			"/buscar ",
			"/reset",
			"/tiempo ",
			"/traducir ",
			"/model",
			"/ask",
			"exit",
			"quit",
			"cd ", "ls ", "cat ", "rm ", "mv ", "cp ", "mkdir ", "rmdir ",
			"grep ", "find ", "chmod ", "chown ", "touch ", "nano ", "vim ",
			"less ", "go ", "git ", "docker ",
		}

		for _, cmd := range commands {
			if strings.HasPrefix(cmd, line) {
				c = append(c, cmd)
			}
		}

		// NO autocompletar rutas para estos comandos
		if strings.HasPrefix(line, "/chat ") ||
			strings.HasPrefix(line, "/buscar ") ||
			strings.HasPrefix(line, "/tiempo ") ||
			strings.HasPrefix(line, "/traducir ") {
				return c
			}

			var pathPrefix string
			var partToComplete string
			lastSpace := strings.LastIndex(line, " ")
			if lastSpace == -1 {
				pathPrefix = ""
				partToComplete = line
			} else {
				pathPrefix = line[:lastSpace+1]
				partToComplete = line[lastSpace+1:]
			}

			globPattern := partToComplete + "*"
			if strings.HasPrefix(globPattern, "~/") {
				home, err := os.UserHomeDir()
				if err == nil {
					globPattern = filepath.Join(home, strings.TrimPrefix(globPattern, "~/"))
				}
			}

			files, _ := filepath.Glob(globPattern)
			for _, f := range files {
				if info, err := os.Stat(f); err == nil && info.IsDir() {
					c = append(c, pathPrefix+f+"/")
				} else {
					c = append(c, pathPrefix+f)
				}
			}
			return
	})
	// --- FIN DE LÓGICA DE AUTO-COMPLETADO ---

	home, err := os.UserHomeDir()
	if err == nil {
		// Inicializar rutas globales
		historyPath = filepath.Join(home, historyFileName) // ¡CORREGIDO: Asignación!
		semanticHistoryPath = filepath.Join(home, embeddingHistoryFile)
		chatHistoryPath = filepath.Join(home, chatHistoryFile) // ¡CORREGIDO: Asignación!

		// Cargar historial de liner
		if f, err := os.Open(historyPath); err == nil {
			state.ReadHistory(f)
			f.Close()
		}
		// Cargar historial semántico
		loadSemanticHistory()
		fmt.Printf(cSystem("Cargados %d comandos del historial semántico.\n"), len(semanticHistory))

		// Cargar historial de chat
		loadChatHistory()
		if len(chatHistory) > 1 {
			// El historial > 1 significa que hay mensajes de usuario/asistente (sin contar el system prompt)
			fmt.Printf(cSystem("Reanudando conversación de chat. (Mensajes cargados: %d)\n"), len(chatHistory)-1)
		}
	}
	defer saveHistory(state)

	selectedModel := chooseModel(client, state)

	clearScreen()
	msg := fmt.Sprintf("Cargando modelo \"%s\" en memoria...\n(Esto puede tardar unos segundos)", selectedModel)
	fmt.Println(cSystem(msg))
	warmUpModel(client, selectedModel)

	clearScreen()
	printLogo(selectedModel)
	printHeader()
	fmt.Println(cSystem("\n  Consejo: Escribe /help para ver todos los comandos."))

	var alwaysExecute bool = false
	var isFirstLoop bool = true

	for {
		select {
			case updateMsg := <-updateMessageChannel:
				fmt.Print(updateMsg)
			default:
		}

		if isFirstLoop {
			isFirstLoop = false
			fmt.Println()
		} else {
			fmt.Println(cSystem("──────────────────────────────────────────────────"))
		}

		var prompt string
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "(error dir)"
		} else {
			home, err := os.UserHomeDir()
			if err == nil && strings.HasPrefix(cwd, home) {
				cwd = "~" + strings.TrimPrefix(cwd, home)
			}
		}

		var promptPrefix string
		if alwaysExecute {
			promptPrefix = "ia (auto) "
		} else {
			promptPrefix = "ia "
		}
		prompt = fmt.Sprintf("%s[%s]> %s >>> ", promptPrefix, selectedModel, cwd)

		input, err := state.Prompt(prompt)
		if err != nil {
			if err == io.EOF {
				break
			} else if err == liner.ErrPromptAborted {
				fmt.Println(cSystem("\nPrompt cancelado. Escribe 'exit' o Ctrl+D para salir."))
				continue
			} else {
				fmt.Println(cError(fmt.Sprintf("Error al leer la entrada: %v", err)))
				continue
			}
		}

		if input != "" {
			state.AppendHistory(input)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			break
		}

		if strings.HasPrefix(input, "cd ") {
			dir := strings.TrimSpace(strings.TrimPrefix(input, "cd "))
			if dir == "" || dir == "~" {
				home, err := os.UserHomeDir()
				if err != nil {
					fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al encontrar el home dir: %v", err)))
					continue
				}
				dir = home
			}
			if err := os.Chdir(dir); err != nil {
				fmt.Fprintln(os.Stderr, cError(err.Error()))
			}
			continue

		} else if input == "/model" {
			// Acceso directo: Muestra el selector de modelos.
			selectedModel = chooseModel(client, state)
			clearScreen()
			msg := fmt.Sprintf("Cargando modelo \"%s\" en memoria...\n(Esto puede tardar unos segundos)", selectedModel)
			fmt.Println(cSystem(msg))
			warmUpModel(client, selectedModel)
			clearScreen()
			printLogo(selectedModel)
			printHeader()
			fmt.Println(cSystem("\n  Consejo: Escribe /help para ver todos los comandos."))
			isFirstLoop = true
			continue

		} else if input == "/ask" {
			// Acceso directo: Desactiva el modo 'auto'.
			alwaysExecute = false
			fmt.Println(cSystem("IA> Modo auto-ejecución desactivado. Se pedirá confirmación."))
			fmt.Println()
			continue

		} else if input == "/reset" {
			// Acceso directo: Limpia el historial de chat.
			chatHistory = nil
			fmt.Println(cSystem("IA> Historial de chat limpiado."))
			fmt.Println()
			continue

		} else if input == "/config" {
			// Comando central: Menú de configuración.
			newModel, newAutoState := handleConfigCommand(client, state, selectedModel, alwaysExecute)

			// Si el modelo cambió, forzamos la recarga de la UI y el calentamiento
			if newModel != selectedModel {
				selectedModel = newModel
				clearScreen()
				msg := fmt.Sprintf("Cargando modelo \"%s\" en memoria...\n(Esto puede tardar unos segundos)", selectedModel)
				fmt.Println(cSystem(msg))
				warmUpModel(client, selectedModel)
				clearScreen()
				printLogo(selectedModel)
				printHeader()
				fmt.Println(cSystem("\n  Consejo: Escribe /help para ver todos los comandos."))
				isFirstLoop = true
			}

			alwaysExecute = newAutoState
			continue

		} else if input == "/help" {
			printHelp()
			continue

		} else if strings.HasPrefix(input, "/tiempo ") {
			prompt := strings.TrimPrefix(input, "/tiempo ")
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Println(cError("IA> Petición de tiempo vacía. Escribe /tiempo <lugar>."))
				fmt.Println()
				continue
			}
			handleWeatherCommand(client, selectedModel, prompt)

		} else if strings.HasPrefix(input, "/traducir ") {
			prompt := strings.TrimPrefix(input, "/traducir ")
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Println(cError("IA> Petición de traducción vacía. Escribe /traducir <idioma> <texto>."))
				fmt.Println()
				continue
			}
			handleTranslateCommand(client, selectedModel, prompt)

		} else if strings.HasPrefix(input, "/chat ") {
			prompt := strings.TrimPrefix(input, "/chat ")
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Println(cError("IA> Petición de chat vacía. Escribe /chat seguido de tu pregunta."))
				fmt.Println()
				continue
			}
			handleChatCommand(client, selectedModel, prompt)

		} else if strings.HasPrefix(input, "/buscar ") {
			query := strings.TrimPrefix(input, "/buscar ")
			query = strings.TrimSpace(query)
			if query == "" {
				fmt.Println(cError("IA> Petición de búsqueda vacía. Escribe /buscar <intención>."))
				fmt.Println()
				continue
			}
			// La función handleSearchCommand ahora puede activar alwaysExecute
			if handleSearchCommand(client, state, selectedModel, query) {
				alwaysExecute = true
			}

		} else if strings.HasPrefix(input, "/") {
			prompt := strings.TrimPrefix(input, "/")
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Println(cError("IA> Petición de IA vacía. Escribe / seguido de tu consulta."))
				fmt.Println()
				continue
			}
			if alwaysExecute {
				handleIACommandAuto(client, selectedModel, prompt)
			} else {
				if handleIACommandConfirm(client, state, selectedModel, prompt) {
					alwaysExecute = true
				}
			}

		} else {
			// --- INICIO DE DEPURACIÓN INTELIGENTE DE ERRORES ---
			finalInput := input
			if shouldColorOutput(input) {
				firstSpace := strings.Index(input, " ")
				if firstSpace == -1 {
					finalInput = input + " --color=always"
				} else {
					cmdName := input[:firstSpace]
					args := input[firstSpace:]
					finalInput = cmdName + " --color=always" + args
				}
			}

			cmd := exec.Command("bash", "-c", finalInput)
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
			cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

			fmt.Println()
			err := cmd.Run()

			// Guardar en historial semántico
			if err == nil {
				// Solo guardar si el comando fue exitoso
				go addCommandToSemanticHistory(client, selectedModel, finalInput)
			} else {
				// El comando falló, analizar el error
				errorOutput := stderrBuf.String()
				fmt.Println()
				fmt.Println(cSystem("--- Análisis de Error de Shell ---"))
				handleDebugCommand(client, selectedModel, errorOutput)
			}

			fmt.Println()
			// --- FIN DE DEPURACIÓN INTELIGENTE DE ERRORES! ---
		}
	}

	saveChatHistory()
	fmt.Println(cSystem("\n¡Adiós!"))
}

// formatCommandForDisplay hace que los comandos largos sean más legibles
// insertando saltos de línea y sangría para pipes y encadenamientos.
func formatCommandForDisplay(cmd string) string {
	if cmd == "" {
		return ""
	}

	// 1. Reemplazar pipes y encadenamientos con saltos de línea + tabulación

	// Reemplazar pipes: '|' por ' |\n    ' (pipe, salto de línea, 4 espacios)
	formatted := strings.ReplaceAll(cmd, "|", " |\n    ")

		// Reemplazar encadenamiento: ';' por ';\n' (punto y coma, salto de línea)
		formatted = strings.ReplaceAll(formatted, ";", ";\n")

			// Reemplazar '&&' y '||' también.
			formatted = strings.ReplaceAll(formatted, "&&", " &&\n    ")
				formatted = strings.ReplaceAll(formatted, "||", " ||\n    ")

					return strings.TrimSpace(formatted)
}


// sanitizeIACommand limpia y prepara el comando sugerido por la IA para su ejecución.
func sanitizeIACommand(rawCmd string) string {
	cmd := strings.TrimSpace(rawCmd)

	// Caso 1: Bloque de código con 3 backticks (```bash ... ```)
	if strings.HasPrefix(cmd, "```") && strings.HasSuffix(cmd, "```") {
		cmd = strings.TrimPrefix(cmd, "```")
		cmd = strings.TrimSuffix(cmd, "```")

		// Si empieza con un lenguaje (ej. bash\n), eliminarlo
		if strings.HasPrefix(cmd, "bash\n") {
			cmd = strings.TrimPrefix(cmd, "bash\n")
		} else if strings.HasPrefix(cmd, "sh\n") {
			cmd = strings.TrimPrefix(cmd, "sh\n")
		} else if strings.HasPrefix(cmd, "shell\n") {
			cmd = strings.TrimPrefix(cmd, "shell\n")
		}

		return strings.TrimSpace(cmd)
	}

	// Caso 2: Comando con un solo backtick (`...`)
	if strings.HasPrefix(cmd, "`") && strings.HasSuffix(cmd, "`") {
		cmd = strings.TrimPrefix(cmd, "`")
		cmd = strings.TrimSuffix(cmd, "`")
		return strings.TrimSpace(cmd)
	}

	// Caso 3: Comando simple o multi-línea sin backticks.
	return cmd
}

// handleWeatherCommand
func handleWeatherCommand(client *api.Client, modelName string, location string) {
	fmt.Println(cIA("IA> Consultando el tiempo...") + cSystem(" (Usando wttr.in)"))
	endpoint := fmt.Sprintf("http://wttr.in/%s?format=j1", url.QueryEscape(location))
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Println(cError(fmt.Sprintf("\nError al crear la petición web: %v", err)))
		return
	}
	req.Header.Set("User-Agent", "terminal-ia-go-client")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println(cError(fmt.Sprintf("\nError al llamar a la API de wttr.in: %v", err)))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Println(cError(fmt.Sprintf("\nError de la API de wttr.in (código %d). ¿Localización correcta?", resp.StatusCode)))
		return
	}
	var wttrResp WttrResponse
	if err := json.NewDecoder(resp.Body).Decode(&wttrResp); err != nil {
		fmt.Println(cError(fmt.Sprintf("\nError al decodificar la respuesta de wttr.in: %v", err)))
		return
	}
	if len(wttrResp.CurrentCondition) == 0 {
		fmt.Println(cError("\nLo siento, wttr.in no pudo encontrar esa localización."))
		fmt.Println()
		return
	}
	current := wttrResp.CurrentCondition[0]
	desc := ""
	if len(current.WeatherDesc) > 0 {
		desc = current.WeatherDesc[0].Value
	}
	locName := location
	if len(wttrResp.NearestArea) > 0 && len(wttrResp.NearestArea[0].AreaName) > 0 {
		locName = wttrResp.NearestArea[0].AreaName[0].Value
		if len(wttrResp.NearestArea[0].Country) > 0 {
			locName += ", " + wttrResp.NearestArea[0].Country[0].Value
		}
	}
	contextSnippet := fmt.Sprintf(
		"Contexto del tiempo para %s:\nTemperatura: %s°C\nSensación térmica: %s°C\nDescripción: %s\n",
		locName,
		current.Temp_C,
		current.FeelsLikeC,
		desc,
	)
	systemPrompt := "Eres un asistente de IA. Responde a la 'Pregunta del Usuario' en español, de forma concisa y amigable, basándote únicamente en el 'Contexto del tiempo' proporcionado."
	fullPrompt := fmt.Sprintf("%s\n\n%s\n\nPregunta del Usuario: ¿Qué tiempo hace en %s?", systemPrompt, contextSnippet, location)
	fmt.Println(cIA("IA> Generando respuesta..."))
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		cancel()
	}()
	defer signal.Stop(sigChan)
	stream := true
	reqOllama := &api.GenerateRequest{
		Model:  modelName,
		Prompt: fullPrompt,
		Stream: &stream,
	}
	firstChunk := true
	streamHandler := func(r api.GenerateResponse) error {
		if firstChunk {
			fmt.Print("\r" + cIA("IA: ") + "    \r")
			firstChunk = false
		}
		fmt.Print(r.Response)
		return nil
	}
	err = client.Generate(ctx, reqOllama, streamHandler)
	if err != nil {
		if err == context.Canceled {
			fmt.Print(cError("\n[Stream cancelado]"))
		} else {
			fmt.Println(cError(fmt.Sprintf("\nError al generar respuesta de chat: %v", err)))
		}
	}
	fmt.Println()
}

// handleChatCommand
func handleChatCommand(client *api.Client, modelName string, userPrompt string) {
	if len(chatHistory) == 0 {
		chatHistory = append(chatHistory, api.Message{
			Role:    "system",
			Content: "Eres un asistente de IA para terminal. Sé directo, conciso y técnico. Céntrate en la solicitud. Evita saludos largos o florituras innecesarias.",
		})
	}
	chatHistory = append(chatHistory, api.Message{
		Role:    "user",
		Content: userPrompt,
	})
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		cancel()
	}()
	defer signal.Stop(sigChan)
	fmt.Println(cIA("IA> Pensando...") + cSystem(" (Presiona Ctrl+C para cancelar)"))
	stream := true
	req := &api.ChatRequest{
		Model:    modelName,
		Messages: chatHistory,
		Stream:   &stream,
	}
	firstChunk := true
	var fullResponse strings.Builder
	streamHandler := func(r api.ChatResponse) error {
		if firstChunk {
			fmt.Print("\r" + cIA("IA: ") + "    \r")
			firstChunk = false
		}
		fmt.Print(r.Message.Content)
		fullResponse.WriteString(r.Message.Content)
		return nil
	}
	err := client.Chat(ctx, req, streamHandler)
	if err != nil {
		if err == context.Canceled {
			fmt.Print(cError("\n[Stream cancelado]"))
			if len(chatHistory) > 0 {
				chatHistory = chatHistory[:len(chatHistory)-1]
			}
		} else {
			fmt.Println(cError(fmt.Sprintf("\nError al generar respuesta de chat: %v", err)))
			if len(chatHistory) > 0 {
				chatHistory = chatHistory[:len(chatHistory)-1]
			}
		}
	} else {
		chatHistory = append(chatHistory, api.Message{
			Role:    "assistant",
			Content: fullResponse.String(),
		})
	}
	fmt.Println()
}

// handleTranslateCommand
func handleTranslateCommand(client *api.Client, modelName string, userPrompt string) {
	parts := strings.SplitN(userPrompt, " ", 2)
	if len(parts) < 2 {
		fmt.Println(cError("Error de formato. Uso: /traducir <idioma> <texto>"))
		fmt.Println(cSystem("Ejemplo: /traducir en Hello world"))
		fmt.Println()
		return
	}
	targetLang := parts[0]
	textToTranslate := parts[1]
	systemPrompt := fmt.Sprintf("Eres un traductor experto. Traduce el texto del usuario al idioma '%s'. Responde ÚNICAMENTE con la traducción, sin explicaciones ni frases introductorias.", targetLang)
	fullPrompt := textToTranslate
	fmt.Println(cIA("IA> Traduciendo..."))
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		cancel()
	}()
	defer signal.Stop(sigChan)
	stream := true
	req := &api.GenerateRequest{
		Model:  modelName,
		System: systemPrompt,
		Prompt: fullPrompt,
		Stream: &stream,
	}
	firstChunk := true
	streamHandler := func(r api.GenerateResponse) error {
		if firstChunk {
			fmt.Print("\r" + cIA("IA: ") + "    \r")
			firstChunk = false
		}
		fmt.Print(r.Response)
		return nil
	}
	err := client.Generate(ctx, req, streamHandler)
	if err != nil {
		if err == context.Canceled {
			fmt.Print(cError("\n[Stream cancelado]"))
		} else {
			fmt.Println(cError(fmt.Sprintf("\nError al generar respuesta de traducción: %v", err)))
		}
	}
	fmt.Println()
}

// handleIACommandAuto
func handleIACommandAuto(client *api.Client, modelName string, userPrompt string) {
	// 1. Obtener contexto de archivos
	dirSnippet := getDirectorySnippet()
	contextLine := ""
	if dirSnippet != "" {
		contextLine = fmt.Sprintf("Contexto de archivos en CWD: %s.", dirSnippet)
	}

	// 2. Definir System Prompt
	systemPrompt := fmt.Sprintf(`Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	%s
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `, contextLine)

	// 3. Crear Full Prompt
	fullPrompt := systemPrompt + userPrompt
	fmt.Println(cIA("IA> Procesando (auto)..."))

	req := &api.GenerateRequest{
		Model:  modelName,
		Prompt: fullPrompt,
		Stream: new(bool),
	}
	ctx := context.Background()
	var resp api.GenerateResponse
	responseHandler := func(r api.GenerateResponse) error {
		resp = r
		return nil
	}
	if err := client.Generate(ctx, req, responseHandler); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al contactar con Ollama: %v", err)))
		return
	}
	comandoSugerido := sanitizeIACommand(resp.Response)
	fmt.Println()
	fmt.Println(cSystem("ejecutando (auto):"))
	fmt.Println(comandoSugerido)
	fmt.Println()
	cmd := exec.Command("bash", "-c", comandoSugerido)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
	}
	fmt.Println()
}

// handleIACommandConfirm (Actualizado con formato de display)
func handleIACommandConfirm(client *api.Client, state *liner.State, modelName string, userPrompt string) bool {
	// 1. Obtener contexto de archivos
	dirSnippet := getDirectorySnippet()
	contextLine := ""
	if dirSnippet != "" {
		contextLine = fmt.Sprintf("Contexto de archivos en CWD: %s.", dirSnippet)
	}

	// 2. Definir System Prompt
	systemPrompt := fmt.Sprintf(`Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	%s
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `, contextLine)

	// 3. Crear Full Prompt
	fullPrompt := systemPrompt + userPrompt
	fmt.Println(cIA("IA> Procesando..."))
	req := &api.GenerateRequest{
		Model:  modelName,
		Prompt: fullPrompt,
		Stream: new(bool),
	}
	ctx := context.Background()
	var resp api.GenerateResponse
	responseHandler := func(r api.GenerateResponse) error {
		resp = r
		return nil
	}
	if err := client.Generate(ctx, req, responseHandler); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al contactar con Ollama: %v", err)))
		return false
	}
	comandoSugerido := sanitizeIACommand(resp.Response)

	// --- Visualización Formateada ---
	formattedCommand := formatCommandForDisplay(comandoSugerido)
		// --- Fin Visualización ---

		fmt.Println(cSystem("---"))
		fmt.Println(cIA("IA> Comando sugerido:"))
		fmt.Printf("\n%s\n\n", formattedCommand) // Mostrar versión legible
		fmt.Println(cSystem("---"))

		prompt := "IA> ¿Ejecutar? [s/N/x (Siempre)]: "
		confirmacion, err := state.Prompt(prompt)
		if err != nil {
			if err == io.EOF || err == liner.ErrPromptAborted {
				fmt.Println(cSystem("\nCancelado."))
				return false
			}
			fmt.Println(cError(fmt.Sprintf("Error al leer la confirmación: %v", err)))
			return false
		}
		state.AppendHistory(confirmacion)
		confirmacion = strings.TrimSpace(strings.ToLower(confirmacion))
		switch confirmacion {
			case "s":
				fmt.Println(cSystem("IA> Ejecutando..."))
				fmt.Println()
				fmt.Println(cSystem("ejecutando:"))
				fmt.Println(comandoSugerido) // Ejecutar versión sin formatear
				fmt.Println()
				cmd := exec.Command("bash", "-c", comandoSugerido)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
				}
				fmt.Println()
				return false
			case "x":
				fmt.Println(cSystem("IA> Ejecutando y activando modo 'auto'..."))
				fmt.Println()
				fmt.Println(cSystem("ejecutando:"))
				fmt.Println(comandoSugerido) // Ejecutar versión sin formatear
				fmt.Println()
				cmd := exec.Command("bash", "-c", comandoSugerido)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
				}
				fmt.Println()
				fmt.Println(cSystem("IA> Modo auto-ejecución activado. Escribe '/ask' para desactivarlo."))
				return true
			default:
				fmt.Println(cSystem("IA> Cancelado."))
				fmt.Println()
				return false
		}
}

// getDirectorySnippet escanea el directorio actual y devuelve un string con los archivos/dirs relevantes.
func getDirectorySnippet() string {
	// 1. Obtener archivos
	files, err := os.ReadDir(".")
	if err != nil {
		return ""
	}

	var snippet strings.Builder
	fileCount := 0

	// Lista de nombres o extensiones a ignorar para mantener el contexto limpio
	ignoreList := map[string]bool{
		".git": true,
		"vendor": true,
		"node_modules": true,
		"terminal-ia": true, // El binario compilado
	}

	// 2. Formatear los 10 elementos más relevantes
	for _, file := range files {
		name := file.Name()

		// 2.1. Ignorar elementos
		if strings.HasPrefix(name, ".") && name != "." && name != ".." {
			// Ignorar archivos y directorios ocultos (excepto los que queremos ver)
			if _, ok := ignoreList[name]; !ok {
				continue
			}
		} else if _, ok := ignoreList[name]; ok {
			continue
		}

		if fileCount >= 10 { // Limitar a 10 items
			break
		}

		// 2.2. Añadir al snippet
		if file.IsDir() {
			snippet.WriteString(name)
			snippet.WriteString("/")
		} else {
			snippet.WriteString(name)
		}
		snippet.WriteString(", ")
		fileCount++
	}

	if snippet.Len() > 0 {
		// Eliminar la última coma y espacio ", "
		return strings.TrimSuffix(snippet.String(), ", ")
	}
	return ""
}

// handleConfigCommand gestiona el menú de configuración interactivo.
func handleConfigCommand(client *api.Client, state *liner.State, currentModel string, currentAutoState bool) (string, bool) {
	fmt.Println()

	// Bucle principal del menú
	for {
		// --- Mostrar ambas versiones en la cabecera ---
		versionInfo := fmt.Sprintf("Local: %s", currentVersion)
		if githubLatestVersion != strings.TrimPrefix(strings.ToLower(currentVersion), "v") {
			versionInfo += cError(fmt.Sprintf(" | GitHub: %s (¡ACTUALIZAR!)", githubLatestVersion))
		} else {
			versionInfo += cIA(" | GitHub: Al día")
		}

		fmt.Println(cSystem(fmt.Sprintf("--- Configuración de Terminal IA (%s) ---", versionInfo)))

		// 1. Mostrar Estado Actual
		autoState := cError("DESACTIVADO (Pedir Confirmación)")
		if currentAutoState {
			autoState = cIA("ACTIVADO (Auto-ejecución)")
		}

		fmt.Println(cPrompt(" [1] Modelo Actual: ") + cModel(currentModel))
		fmt.Println(cPrompt(" [2] Modo Ejecución: ") + autoState)

		// --- Mostrar ambas versiones en Opción 3 ---
		fmt.Println(cPrompt(" [3] Versión:        ") + versionInfo)

		fmt.Println(cPrompt(" [4] Limpiar Historial Semántico"))
		fmt.Println(cPrompt(" [5] Limpiar Historial de Chat"))
		fmt.Println(cPrompt(" [Q] Salir del Menú de Configuración"))
		fmt.Println(cSystem("------------------------------------------------"))

		prompt := "Selecciona una opción [1-5, Q]: "
		input, err := state.Prompt(prompt)
		if err != nil || strings.ToLower(input) == "q" || err == liner.ErrPromptAborted {
			fmt.Println(cSystem("\nSaliendo del menú de configuración."))
			return currentModel, currentAutoState // Salir sin cambios
		}

		state.AppendHistory(input)
		input = strings.TrimSpace(strings.ToLower(input))
		fmt.Println()

		switch input {
			case "1":
				// Cambiar Modelo (Reutilizar chooseModel)
				newModel := chooseModel(client, state)

				// Recalentar y actualizar la UI
				fmt.Println(cSystem("Recargando modelo..."))
				warmUpModel(client, newModel)
				clearScreen()
				printLogo(newModel)
				printHeader()
				fmt.Println(cSystem("\n  Consejo: Escribe /help para ver todos los comandos."))

				return newModel, currentAutoState

			case "2":
				// Alternar Modo Auto/Ask
				newAutoState := !currentAutoState
				if newAutoState {
					fmt.Println(cIA("IA> Modo auto-ejecución ACTIVADO."))
				} else {
					fmt.Println(cSystem("IA> Modo auto-ejecución DESACTIVADO. Se pedirá confirmación."))
				}
				fmt.Println()
				return currentModel, newAutoState

			case "3":
				// Mostrar Versión (Confirmar la información ya mostrada)
				fmt.Println(cSystem(fmt.Sprintf("Terminal IA versión local: %s.", currentVersion)))
				if githubLatestVersion != strings.TrimPrefix(strings.ToLower(currentVersion), "v") {
					fmt.Println(cError(fmt.Sprintf("¡ADVERTENCIA! La versión de GitHub (%s) es más reciente.", githubLatestVersion)))
				}
				fmt.Println()

			case "4":
				// Limpiar Historial Semántico
				// (Limpiar el archivo y el slice en memoria)
				semanticHistoryLock.Lock()
				semanticHistory = make([]SemanticHistoryEntry, 0)
				semanticHistoryLock.Unlock()

				// Sobreescribir el archivo con un array vacío
				os.WriteFile(semanticHistoryPath, []byte("[]"), 0644)

				fmt.Println(cIA("IA> Historial Semántico limpiado."))
				fmt.Println()

			case "5":
				// Limpiar Historial de Chat (Reutilizar lógica de /reset)
				chatHistory = nil
				fmt.Println(cIA("IA> Historial de Chat limpiado."))
				fmt.Println()

			default:
				fmt.Println(cError("Opción inválida. Inténtalo de nuevo."))
				fmt.Println()
		}
	}
}

// handleDebugCommand
func handleDebugCommand(client *api.Client, modelName string, errorOutput string) {
	if len(errorOutput) > 2048 {
		errorOutput = errorOutput[:2048] + "\n... (Error truncado)"
	}
	fullPrompt := fmt.Sprintf("%s\n\nError de Stderr:\n```\n%s\n```", debugSystemPrompt, errorOutput)
	fmt.Println(cIA("IA> Analizando error...") + cSystem(" (Presiona Ctrl+C para cancelar)"))
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		cancel()
	}()
	defer signal.Stop(sigChan)
	stream := true
	req := &api.GenerateRequest{
		Model:  modelName,
		Prompt: fullPrompt,
		Stream: &stream,
	}
	firstChunk := true
	streamHandler := func(r api.GenerateResponse) error {
		if firstChunk {
			fmt.Print("\r" + cIA("IA: ") + "    \r")
			firstChunk = false
		}
		fmt.Print(r.Response)
		return nil
	}
	err := client.Generate(ctx, req, streamHandler)
	if err != nil && err != context.Canceled {
		fmt.Println(cError(fmt.Sprintf("\nError al generar análisis: %v", err)))
	} else if err == context.Canceled {
		fmt.Print(cError("\n[Análisis cancelado]"))
	}
	fmt.Println()
}

// shouldColorOutput
func shouldColorOutput(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	colorCommands := []string{
		"ls", "grep", "diff", "git", "kubectl", "docker", "tree",
	}
	for _, c := range colorCommands {
		if strings.HasPrefix(cmd, c+" ") || cmd == c {
			return true
		}
	}
	return false
}


// --- Funciones Matemáticas para Similitud de Coseno ---

func dotProduct(a, b []float64) float64 {
	var sum float64
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func magnitude(v []float64) float64 {
	var sumSq float64
	for _, val := range v {
		sumSq += val * val
	}
	return math.Sqrt(sumSq)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	magA := magnitude(a)
	magB := magnitude(b)
	if magA == 0 || magB == 0 {
		return 0.0
	}
	return dotProduct(a, b) / (magA * magB)
}

// getEmbedding llama a la API de Ollama para un texto dado usando el modelo DEDICADO.
func getEmbedding(client *api.Client, text string, model string) ([]float64, error) {
	req := &api.EmbeddingRequest{
		Model:  embeddingModelName, // <-- ¡USAR MODELO DEDICADO!
		Prompt: text,
	}
	ctx := context.Background()
	resp, err := client.Embeddings(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}

// --- Funciones de Persistencia del Historial de Chat ---

// loadChatHistory carga el historial de chat desde el archivo JSON.
func loadChatHistory() {
	chatHistoryLock.Lock()
	defer chatHistoryLock.Unlock()

	if _, err := os.Stat(chatHistoryPath); os.IsNotExist(err) {
		chatHistory = make([]api.Message, 0)
		return // Archivo no existe, empezamos un nuevo historial
	}

	data, err := os.ReadFile(chatHistoryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al leer historial de chat: %v", err)))
		chatHistory = make([]api.Message, 0)
		return
	}

	if len(data) == 0 {
		chatHistory = make([]api.Message, 0)
		return
	}

	if err := json.Unmarshal(data, &chatHistory); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al parsear historial de chat (se reiniciará): %v", err)))
		chatHistory = make([]api.Message, 0)
	}
}

// saveChatHistory guarda el historial de chat en el archivo JSON.
func saveChatHistory() {
	chatHistoryLock.Lock()
	defer chatHistoryLock.Unlock()

	// Solo guardar si hay algo en el historial que valga la pena (ej. más que solo el system prompt inicial)
	if len(chatHistory) <= 1 {
		os.Remove(chatHistoryPath) // Si está vacío o solo tiene el system prompt, lo borramos
		return
	}

	data, err := json.MarshalIndent(chatHistory, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al serializar historial de chat: %v", err)))
		return
	}
	if err := os.WriteFile(chatHistoryPath, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al guardar historial de chat: %v", err)))
	}
}

// loadSemanticHistory carga los embeddings desde el archivo JSON
func loadSemanticHistory() {
	semanticHistoryLock.Lock()
	defer semanticHistoryLock.Unlock()

	if _, err := os.Stat(semanticHistoryPath); os.IsNotExist(err) {
		semanticHistory = make([]SemanticHistoryEntry, 0)
		return
	}

	data, err := os.ReadFile(semanticHistoryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al leer historial semántico: %v", err)))
		semanticHistory = make([]SemanticHistoryEntry, 0)
		return
	}

	if err := json.Unmarshal(data, &semanticHistory); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al parsear historial semántico (se reiniciará): %v", err)))
		semanticHistory = make([]SemanticHistoryEntry, 0)
	}
}

// saveSemanticHistory guarda el historial actual en el archivo JSON
func saveSemanticHistory() {
	semanticHistoryLock.Lock()
	defer semanticHistoryLock.Unlock()

	data, err := json.Marshal(semanticHistory)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al serializar historial semántico: %v", err)))
		return
	}
	if err := os.WriteFile(semanticHistoryPath, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al guardar historial semántico: %v", err)))
	}
}

// addCommandToSemanticHistory
func addCommandToSemanticHistory(client *api.Client, model string, command string) {
	// No guardar comandos vacíos, de historial, o el propio 'buscar'
	if command == "" || strings.HasPrefix(command, "/") || strings.HasPrefix(command, "cd ") {
		return
	}

	// Evitar duplicados exactos
	semanticHistoryLock.Lock()
	for _, entry := range semanticHistory {
		if entry.Command == command {
			semanticHistoryLock.Unlock()
			return
		}
	}
	semanticHistoryLock.Unlock() // Desbloquear antes de la llamada de red

	// 1. Generar el Embedding
	embedding, err := getEmbedding(client, command, model)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("\n[Error de Embedding: %v]", err)))
		return
	}

	// 2. Crear la entrada
	entry := SemanticHistoryEntry{
		Command:   command,
		Embedding: embedding,
	}

	// 3. Añadir al historial y guardar (con Lock)
	semanticHistoryLock.Lock()
	semanticHistory = append(semanticHistory, entry)
	semanticHistoryLock.Unlock()

	saveSemanticHistory() // Guardar el archivo
}

// handleSearchCommand (Muestra y permite seleccionar el Top 3)
func handleSearchCommand(client *api.Client, state *liner.State, model string, query string) bool {
	fmt.Println(cIA("IA> Buscando en historial semántico...") + cSystem(" (Presiona Ctrl+C para cancelar)"))

	if len(semanticHistory) == 0 {
		fmt.Println(cSystem("IA> No hay historial semántico. Ejecuta algunos comandos primero."))
		fmt.Println()
		return false
	}

	// 1. Vectorizar la consulta
	queryEmbedding, err := getEmbedding(client, query, model)
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al generar embedding para la búsqueda: %v", err)))
		return false
	}

	// Estructura para almacenar los resultados del Top 3
	type result struct {
		Command string
		Score   float64
	}
	// Inicializar Top 3 con un score muy bajo
	topResults := make([]result, 3)
	for i := range topResults {
		topResults[i] = result{Score: -1.0}
	}

	// 2. Buscar por similitud y gestionar el Top 3
	semanticHistoryLock.Lock()
	for _, entry := range semanticHistory {
		score := cosineSimilarity(queryEmbedding, entry.Embedding)

		// Lógica simple de inserción ordenada para mantener el Top 3
		for i := 0; i < 3; i++ {
			if score > topResults[i].Score {
				// Mover los elementos inferiores
				for j := 2; j > i; j-- {
					topResults[j] = topResults[j-1]
				}
				// Insertar el nuevo resultado
				topResults[i] = result{Command: entry.Command, Score: score}
				break
			}
		}
	}
	semanticHistoryLock.Unlock()

	// 3. Filtrar resultados no válidos (score -1.0)
	var validResults []result
	for _, res := range topResults {
		if res.Score > 0.1 { // Un umbral mínimo para evitar comandos irrelevantes
			validResults = append(validResults, res)
		}
	}

	if len(validResults) == 0 {
		fmt.Println(cSystem("IA> No se encontraron resultados similares (similitud muy baja)."))
		fmt.Println()
		return false
	}

	// 4. Mostrar el Top N (validResults) y pedir selección
	fmt.Println(cSystem("---"))
	fmt.Println(cIA("IA> Comandos encontrados:"))

	for i, res := range validResults {
		// --- APLICAR FORMATO DE DISPLAY AQUÍ ---
		formattedCommand := formatCommandForDisplay(res.Command)
			fmt.Printf(cPrompt("  [%d]: ")+"%s %s\n", i+1, formattedCommand, cSystem(fmt.Sprintf("(Similitud: %.2f%%)", res.Score*100)))
	}
	fmt.Println(cSystem("---"))

	// 5. Bucle de selección
	var selectedCommand string
	var finalConfirmation string

	for {
		prompt := "IA> ¿Ejecutar [1-" + strconv.Itoa(len(validResults)) + "/N/X (Siempre)]?: "
		confirmacion, err := state.Prompt(prompt)

		if err != nil || confirmacion == "n" || confirmacion == "N" || err == io.EOF || err == liner.ErrPromptAborted {
			fmt.Println(cSystem("\nCancelado."))
			return false
		}

		state.AppendHistory(confirmacion)
		finalConfirmation = confirmacion
		confirmacion = strings.TrimSpace(confirmacion)

		// 5.1 Caso "X" (Siempre)
		if strings.ToLower(confirmacion) == "x" {
			// Usamos la opción 1 como el comando a ejecutar si el usuario selecciona X directamente.
			selectedCommand = validResults[0].Command

			// Caemos en el switch de ejecución de abajo.
			break
		}

		// 5.2 Caso Numérico
		choice, parseErr := strconv.Atoi(confirmacion)
		if parseErr == nil && choice >= 1 && choice <= len(validResults) {
			selectedCommand = validResults[choice-1].Command
			break
		}

		fmt.Println(cError("Selección inválida. Introduce el número de la opción, 'N' o 'X'."))
	}

	// 6. Ejecutar el comando seleccionado
	fmt.Println(cSystem("IA> Ejecutando..."))

	// Aquí usamos el mismo switch de ejecución de handleIACommandConfirm
	setAuto := false
	if strings.ToLower(strings.TrimSpace(finalConfirmation)) == "x" {
		setAuto = true
	}

	fmt.Println()
	fmt.Println(cSystem("ejecutando:"))
	fmt.Println(selectedCommand) // Usamos el comando sin formato

	cmd := exec.Command("bash", "-c", selectedCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
	}
	fmt.Println()

	if setAuto {
		fmt.Println(cSystem("IA> Modo auto-ejecución activado. Escribe '/ask' para desactivarlo."))
	}

	return setAuto
}
