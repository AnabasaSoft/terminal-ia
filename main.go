package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
	currentVersion    = "v21.0" // ¡ACTUALIZADO!
	repoOwner         = "danitxu79"
	repoName          = "terminal-ia"
	historyFileName   = ".terminal_ia_history"
	debugSystemPrompt = "Eres un experto en depuración de comandos de Linux. Analiza el siguiente error de terminal (stderr), explica brevemente por qué ocurrió y proporciona una solución concisa que el usuario pueda copiar/pegar."
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

	// --- ¡NUEVO! Historial de Chat ---
	chatHistory []api.Message
)

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

// warmUpModel (Sin cambios)
func warmUpModel(client *api.Client, modelName string) {
	ctx := context.Background()
	req := &api.GenerateRequest{
		Model:  modelName,
		Prompt: "hola",
		Stream: new(bool),
	}
	responseHandler := func(r api.GenerateResponse) error { return nil }
	if err := client.Generate(ctx, req, responseHandler); err != nil {
		fmt.Fprintln(os.Stderr, cError(fmt.Sprintf("Advertencia: Fallo al 'calentar' el modelo: %v", err)))
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
		// Normalizar versiones quitando la "v" inicial
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

	// --- LÓGICA DE AUTO-COMPLETADO --- (Sin cambios)
	state.SetCompleter(func(line string) (c []string) {
		// --- 1. Definir comandos ---
		// Comandos internos de la IA y comandos de shell comunes
		commands := []string{
			// Comandos IA
			"/help",
			"/chat ",
			"/reset", // <-- ¡Añadido!
			"/tiempo ",
			"/traducir ",
			"/model",
			"/ask",
			"exit",
			"quit",
			// Comandos Shell
			"cd ",
			"ls ",
			"cat ",
			"rm ",
			"mv ",
			"cp ",
			"mkdir ",
			"rmdir ",
			"grep ",
			"find ",
			"chmod ",
			"chown ",
			"touch ",
			"nano ",
			"vim ",
			"less ",
			"go ",
			"git ",
			"docker ",
		}

		// --- 2. Sugerencias de comandos ---
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, line) {
				c = append(c, cmd)
			}
		}

		// --- 3. Autocompletar rutas/archivos ---
		// NO autocompletar rutas para estos comandos específicos
		if strings.HasPrefix(line, "/chat ") ||
			strings.HasPrefix(line, "/tiempo ") ||
			strings.HasPrefix(line, "/traducir ") {
				return c // Devuelve solo las sugerencias de comandos (si las hay)
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
		historyPath := filepath.Join(home, historyFileName)
		if f, err := os.Open(historyPath); err == nil {
			state.ReadHistory(f)
			f.Close()
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

			// --- ¡NUEVO! Comando /reset ---
		} else if input == "/reset" {
			chatHistory = nil // Limpia el historial
			fmt.Println(cSystem("IA> Historial de chat limpiado."))
			fmt.Println()
			continue
			// --- Fin ---

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
			handleChatCommand(client, selectedModel, prompt) // ¡Llama a la nueva función!

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

			if err != nil {
				errorOutput := stderrBuf.String()
				fmt.Println()
				fmt.Println(cSystem("--- Análisis de Error de Shell ---"))
				handleDebugCommand(client, selectedModel, errorOutput)
			}
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

// --- handleChatCommand (¡REESCRITO CON CONTEXTO!) ---
func handleChatCommand(client *api.Client, modelName string, userPrompt string) {
	// 1. Añadir system prompt si es una nueva conversación
	if len(chatHistory) == 0 {
		chatHistory = append(chatHistory, api.Message{
			Role:    "system",
			Content: "Eres un asistente servicial, amigable y conversacional. Responde a las preguntas del usuario.",
		})
	}

	// 2. Añadir el mensaje actual del usuario al historial
	chatHistory = append(chatHistory, api.Message{
		Role:    "user",
		Content: userPrompt,
	})

	// 3. Configurar contexto y cancelación (igual que antes)
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	go func() {
		<-sigChan
		cancel()
	}()

	defer signal.Stop(sigChan)

	fmt.Println(cIA("IA> Pensando...") + cSystem(" (Presiona Ctrl+C para cancelar)"))

	// 4. Crear la petición de CHAT (en lugar de Generate)
	stream := true
	req := &api.ChatRequest{
		Model:    modelName,
		Messages: chatHistory, // ¡Enviar el historial completo!
		Stream:   &stream,
	}

	firstChunk := true
	var fullResponse strings.Builder // Para acumular la respuesta

	// 5. Definir el stream handler (¡recibe api.ChatResponse!)
	streamHandler := func(r api.ChatResponse) error {
		if firstChunk {
			fmt.Print("\r" + cIA("IA: ") + "    \r")
			firstChunk = false
		}
		// Imprimir el trozo de contenido
		fmt.Print(r.Message.Content)
		// Acumular el trozo de contenido
		fullResponse.WriteString(r.Message.Content)
		return nil
	}

	// 6. Llamar a client.Chat
	err := client.Chat(ctx, req, streamHandler)

	// 7. Manejar el final y guardar la respuesta en el historial
	if err != nil {
		if err == context.Canceled {
			fmt.Print(cError("\n[Stream cancelado]"))
			// ¡Importante! Si se cancela, quitamos el último mensaje del usuario
			// para que no se "pegue" a la siguiente petición.
			if len(chatHistory) > 0 {
				chatHistory = chatHistory[:len(chatHistory)-1]
			}
		} else {
			fmt.Println(cError(fmt.Sprintf("\nError al generar respuesta de chat: %v", err)))
			// También quitar en caso de error
			if len(chatHistory) > 0 {
				chatHistory = chatHistory[:len(chatHistory)-1]
			}
		}
	} else {
		// Si todo fue bien, añadir la respuesta completa del asistente al historial
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
	systemPrompt := `Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `
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
	systemPrompt := `Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `
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
