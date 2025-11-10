package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math" // ¡NUEVO!
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync" // ¡NUEVO!
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
	currentVersion       = "v22.0" // ¡ACTUALIZADO!
	repoOwner            = "danitxu79"
	repoName             = "terminal-ia"
	historyFileName      = ".terminal_ia_history"
	embeddingHistoryFile = ".terminal_ia_embeddings.json" // ¡NUEVO!
	debugSystemPrompt    = "Eres un experto en depuración de comandos de Linux. Analiza el siguiente error de terminal (stderr), explica brevemente por qué ocurrió y proporciona una solución concisa que el usuario pueda copiar/pegar."
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

	// --- ¡NUEVO! Historial de Chat y Semántico ---
	chatHistory []api.Message

	semanticHistory     []SemanticHistoryEntry
	semanticHistoryPath string
	semanticHistoryLock sync.Mutex // Mutex para proteger el acceso al historial
)

// --- ¡NUEVO! Struct para Historial Semántico ---
type SemanticHistoryEntry struct {
	Command   string    `json:"command"`
	Embedding []float64 `json:"embedding"`
}

// Structs para APIs (Sin cambios)
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

// clearScreen (Sin cambios)
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// loadLogos (Sin cambios)
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

// createColorMap (Sin cambios)
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

// getLogoKey (Sin cambios)
func getLogoKey(modelName string) string {
	lowerName := strings.ToLower(modelName)
	for key := range logoMap {
		if strings.Contains(lowerName, key) {
			return key
		}
	}
	return "default"
}

// printLogo (Sin cambios)
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

// printHeader (Sin cambios)
func printHeader() {
	fmt.Println()
	fmt.Println(styleHeader.Render(fmt.Sprintf("  Terminal Aumentada por IA (%s)", currentVersion)))
	fmt.Println(styleHeader.Render("  Creado por: Daniel Serrano Armenta <dani.eus79@gmail.com>"))
	fmt.Println(styleHeader.Render("  Copyright (c) 2025 Daniel Serrano Armenta. Ver LICENSE para más detalles."))
	fmt.Println(styleHeader.Render(fmt.Sprintf("  Github: https://github.com/%s/%s", repoOwner, repoName)))
}

// --- printHelp (¡ACTUALIZADO!) ---
func printHelp() {
	fmt.Println()
	fmt.Println(cSystem("--- Ayuda: Comandos Disponibles ---"))
	fmt.Println(cPrompt("  /<petición> ") + cIA("- Pide un comando de shell (ej. /listar archivos .go)"))
	fmt.Println(cPrompt("  /buscar <intención> ") + cIA("- Busca en tu historial por significado (ej. /buscar reiniciar servidor)"))
	fmt.Println(cPrompt("  /chat <pregunta> ") + cIA("- Inicia una conversación de chat (ej. /chat ¿qué es Docker?)"))
	fmt.Println(cPrompt("  /reset       ") + cIA("- Limpia el historial de la conversación de /chat."))
	fmt.Println(cPrompt("  /tiempo <lugar>  ") + cIA("- Consulta el tiempo (sin API key) (ej. /tiempo Madrid)"))
	fmt.Println(cPrompt("  /traducir <idioma> <texto> ") + cIA("- Traduce un texto (ej. /traducir fr hola)"))
	fmt.Println(cPrompt("  /model       ") + cIA("- Vuelve a mostrar el selector de modelos."))
	fmt.Println(cPrompt("  /ask         ") + cIA("- Desactiva el modo 'auto' y vuelve a pedir confirmación."))
	fmt.Println(cPrompt("  /help        ") + cIA("- Muestra este menú de ayuda."))
	fmt.Println(cPrompt("  cd <dir>     ") + cIA("- Cambia el directorio actual (comando interno)."))
	fmt.Println(cPrompt("  exit / quit  ") + cIA("- Cierra la terminal de IA (también Ctrl+D)."))
	fmt.Println(cSystem("------------------------------------"))
	fmt.Println()
}

// warmUpModel (¡ACTUALIZADO CON KEEP-ALIVE Y EMBEDDINGS!)
func warmUpModel(client *api.Client, modelName string) {
	ctx := context.Background()

	// Omitir KeepAlive por ahora para evitar problemas de conversión
	// O usar nil si no es obligatorio

	// --- 2. Pre-calentar el endpoint 'generate' (/ , /chat, /traducir, /debug) ---
	reqGen := &api.GenerateRequest{
		Model:     modelName,
		Prompt:    "hola",
		Stream:    new(bool),
		// KeepAlive: nil, // Comentar o dejar en nil temporalmente
	}
	genHandler := func(r api.GenerateResponse) error { return nil }

	if err := client.Generate(ctx, reqGen, genHandler); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Advertencia: Fallo al 'calentar' (generate): %v", err)))
	}

	// --- 3. Pre-calentar el endpoint 'embeddings' (/buscar) ---
	reqEmb := &api.EmbeddingRequest{
		Model:     modelName,
		Prompt:    "hola",
		// KeepAlive: nil,
	}

	if _, err := client.Embeddings(ctx, reqEmb); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Advertencia: Fallo al 'calentar' (embeddings): %v", err)))
	}
}

// chooseModel (Sin cambios)
func chooseModel(client *api.Client, state *liner.State) string {
	fmt.Println(cSystem("Consultando modelos de Ollama disponibles..."))

	ctx := context.Background()
	resp, err := client.List(ctx)
	if err != nil {
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo listar los modelos de Ollama: %v", err)))
	}
	if len(resp.Models) == 0 {
		log.Fatal(cError("Error fatal: No tienes ningún modelo de Ollama descargado. (Usa 'ollama pull ...')"))
	}

	fmt.Println(cSystem("--- Elige un modelo de IA ---"))
	fmt.Println(cSystem("------------------------------"))
	for i, model := range resp.Models {
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
		if err != nil || choice < 1 || choice > len(resp.Models) {
			fmt.Println(cError("Selección inválida. Introduce un número de la lista."))
		} else {
			state.AppendHistory(input)
			break
		}
	}
	return resp.Models[choice-1].Name
}

// saveHistory (Sin cambios)
func saveHistory(state *liner.State) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Error al encontrar el home dir para guardar historial: %v", err)))
		return
	}
	historyPath := filepath.Join(home, historyFileName)

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

// checkVersion (Sin cambios)
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
		remoteVersion := strings.TrimPrefix(strings.ToLower(release.TagName), "v")
		localVersion := strings.TrimPrefix(strings.ToLower(currentVersion), "v")
		if remoteVersion != localVersion {
			msg := fmt.Sprintf("\n%s\n", cSystem(fmt.Sprintf(
				"  ¡Nueva versión %s disponible! (Estás en %s)",
									 release.TagName, currentVersion,
			)))
			updateMessageChannel <- msg
		}
	}
}

// --- main (¡ACTUALIZADO!) ---
func main() {
	loadLogos()
	createColorMap()
	clearScreen()

	go checkVersion()

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal(cError(fmt.Sprintf("Error fatal: No se pudo crear el cliente de Ollama: %v", err)))
	}

	state := liner.NewLiner()
	defer state.Close()
	state.SetCtrlCAborts(true)

	// --- LÓGICA DE AUTO-COMPLETADO (¡ACTUALIZADO!) ---
	state.SetCompleter(func(line string) (c []string) {
		commands := []string{
			"/help",
			"/chat ",
			"/buscar ", // <-- ¡Añadido!
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
			strings.HasPrefix(line, "/buscar ") || // <-- ¡Añadido!
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
		// Cargar historial de liner
		historyPath := filepath.Join(home, historyFileName)
		if f, err := os.Open(historyPath); err == nil {
			state.ReadHistory(f)
			f.Close()
		}
		// --- ¡NUEVO! Cargar historial semántico ---
		semanticHistoryPath = filepath.Join(home, embeddingHistoryFile)
		loadSemanticHistory()
		fmt.Printf(cSystem("Cargados %d comandos del historial semántico.\n"), len(semanticHistory))
		// --- Fin ---
	}
	defer saveHistory(state)
	// Nota: El historial semántico se guarda en cada adición, no al salir.

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
			alwaysExecute = false
			fmt.Println(cSystem("IA> Modo auto-ejecución desactivado. Se pedirá confirmación."))
			fmt.Println()
			continue

		} else if input == "/help" {
			printHelp()
			continue

		} else if input == "/reset" {
			chatHistory = nil
			fmt.Println(cSystem("IA> Historial de chat limpiado."))
			fmt.Println()
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

			// --- ¡NUEVO! Bloque /buscar ---
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
			// --- Fin ---

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
			// --- INICIO DE DEPURACIÓN INTELIGENTE DE ERRORES (¡ACTUALIZADO!) ---
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

			// --- ¡NUEVO! Guardar en historial semántico ---
			if err == nil {
				// Solo guardar si el comando fue exitoso
				// Se ejecuta en gorutina para no bloquear el prompt
				go addCommandToSemanticHistory(client, selectedModel, finalInput)
			} else {
				// El comando falló, analizar el error
				errorOutput := stderrBuf.String()
				fmt.Println()
				fmt.Println(cSystem("--- Análisis de Error de Shell ---"))
				handleDebugCommand(client, selectedModel, errorOutput)
			}
			// --- Fin ---

			fmt.Println()
			// --- FIN DE DEPURACIÓN INTELIGENTE DE ERRORES! ---
		}
	}

	fmt.Println(cSystem("\n¡Adiós!"))
}

// sanitizeIACommand (Sin cambios)
func sanitizeIACommand(rawCmd string) string {
	cmd := strings.TrimSpace(rawCmd)
	if strings.HasPrefix(cmd, "`") && strings.HasSuffix(cmd, "`") {
		cmd = strings.TrimPrefix(cmd, "`")
		cmd = strings.TrimSuffix(cmd, "`")
		return strings.TrimSpace(cmd)
	}
	if strings.HasPrefix(cmd, "```") && strings.HasSuffix(cmd, "```") {
		cmd = strings.TrimPrefix(cmd, "```")
		cmd = strings.TrimSuffix(cmd, "```")
		if strings.HasPrefix(cmd, "bash\n") {
			cmd = strings.TrimPrefix(cmd, "bash\n")
		} else if strings.HasPrefix(cmd, "sh\n") {
			cmd = strings.TrimPrefix(cmd, "sh\n")
		}
		return strings.TrimSpace(cmd)
	}
	return cmd
}

// handleWeatherCommand (Sin cambios)
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

// handleChatCommand (Sin cambios)
func handleChatCommand(client *api.Client, modelName string, userPrompt string) {
	if len(chatHistory) == 0 {
		chatHistory = append(chatHistory, api.Message{
			Role:    "system",
			Content: "Eres un asistente servicial, amigable y conversacional. Responde a las preguntas del usuario.",
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

// handleTranslateCommand (Sin cambios)
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

// handleIACommandAuto (Sin cambios)
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

// handleIACommandConfirm (Sin cambios)
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
	fmt.Println(cSystem("---"))
	fmt.Println(cIA("IA> Comando sugerido:"))
	fmt.Printf("\n%s\n\n", comandoSugerido)
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
			fmt.Println(comandoSugerido)
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
			fmt.Println(comandoSugerido)
			fmt.Println()
			cmd := exec.Command("bash", "-c", comandoSugerido)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
			}
			fmt.Println()
			fmt.Println(cSystem("IA> Modo auto-ejecución activado. Escribe '/ask' para desactivarlo."))
			fmt.Println()
			return true
		default:
			fmt.Println(cSystem("IA> Cancelado."))
			fmt.Println()
			return false
	}
}

// handleDebugCommand (Sin cambios)
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

// shouldColorOutput (Sin cambios)
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

// getEmbedding llama a la API de Ollama para un texto dado
func getEmbedding(client *api.Client, text string, model string) ([]float64, error) {
	// ¡CORREGIDO! Usa EmbeddingRequest (singular)
	req := &api.EmbeddingRequest{
		Model:  model, // Usar el modelo actual (o uno dedicado si se prefiere)
		Prompt: text,
	}
	ctx := context.Background()
	resp, err := client.Embeddings(ctx, req)
	if err != nil {
		return nil, err
	}
	// ¡CORREGIDO! Devuelve directamente resp.Embedding (que ya es []float64)
	return resp.Embedding, nil
}

// loadSemanticHistory carga los embeddings desde el archivo JSON
func loadSemanticHistory() {
	semanticHistoryLock.Lock()
	defer semanticHistoryLock.Unlock()

	if _, err := os.Stat(semanticHistoryPath); os.IsNotExist(err) {
		semanticHistory = make([]SemanticHistoryEntry, 0)
		return // Archivo no existe, empezar de cero
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

// addCommandToSemanticHistory (El proceso de "Memoria")
// Se ejecuta en una gorutina para no bloquear el prompt
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
			return // Ya existe
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

// handleSearchCommand (El proceso de "Recuperación")
// Devuelve un bool 'setAuto' (igual que handleIACommandConfirm)
func handleSearchCommand(client *api.Client, state *liner.State, model string, query string) bool {
	fmt.Println(cIA("IA> Buscando en historial semántico..."))

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

	// 2. Buscar por similitud
	var bestScore float64 = -1.0
	var bestCommand string = ""

	semanticHistoryLock.Lock()
	for _, entry := range semanticHistory {
		score := cosineSimilarity(queryEmbedding, entry.Embedding)
		if score > bestScore {
			bestScore = score
			bestCommand = entry.Command
		}
	}
	semanticHistoryLock.Unlock()

	if bestCommand == "" {
		fmt.Println(cSystem("IA> No se encontraron resultados similares."))
		fmt.Println()
		return false
	}

	// 3. Mostrar y pedir confirmación (lógica copiada de handleIACommandConfirm)
	fmt.Println(cSystem("---"))
	fmt.Println(cIA(fmt.Sprintf("IA> Comando encontrado (Similitud: %.2f%%):", bestScore*100)))
	fmt.Printf("\n%s\n\n", bestCommand)
	fmt.Println(cSystem("---"))

	prompt := "IA> ¿Ejecutar? [s/N/X (Siempre)]: "
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
			fmt.Println(bestCommand)
			fmt.Println()
			cmd := exec.Command("bash", "-c", bestCommand)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
				// (Podríamos llamar a handleDebugCommand aquí también)
			}
			fmt.Println()
			return false
		case "x":
			fmt.Println(cSystem("IA> Ejecutando y activando modo 'auto'..."))
			fmt.Println()
			fmt.Println(cSystem("ejecutando:"))
			fmt.Println(bestCommand)
			fmt.Println()
			cmd := exec.Command("bash", "-c", bestCommand)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, cError("IA> El comando falló."))
			}
			fmt.Println()
			fmt.Println(cSystem("IA> Modo auto-ejecución activado. Escribe '/ask' para desactivarlo."))
			fmt.Println()
			return true // Devuelve true para activar el modo 'auto'
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
