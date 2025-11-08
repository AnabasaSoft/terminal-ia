package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ollama/ollama/api"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

var (
	logoMap    map[string][]string
	colorMap   map[string][]string
	styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5733"))
)

// --- clearScreen (Sin cambios) ---
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// --- loadLogos (Sin cambios) ---
func loadLogos() {
	file, err := os.Open("logos.json")
	if err != nil {
		fmt.Println(styleError.Render("Error: No se encontró logos.json. Saltando logos."))
		logoMap = make(map[string][]string)
		return
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("Error fatal: No se pudo leer logos.json:", err)
	}
	if err := json.Unmarshal(bytes, &logoMap); err != nil {
		log.Fatal("Error fatal: No se pudo parsear logos.json:", err)
	}
}

// --- createColorMap (Sin cambios) ---
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

// --- getLogoKey (Sin cambios) ---
func getLogoKey(modelName string) string {
	lowerName := strings.ToLower(modelName)
	for key := range logoMap {
		if strings.Contains(lowerName, key) {
			return key
		}
	}
	return "default"
}

// --- printLogo (Sin cambios) ---
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

// --- FUNCIÓN DE CALENTAMIENTO (MODIFICADA) ---
// ¡Ahora SÓLO calienta el modelo! No borra ni imprime logos.
// Es una función "silenciosa" de trabajo.
func warmUpModel(client *api.Client, modelName string) {
	ctx := context.Background()
	req := &api.GenerateRequest{
		Model:  modelName,
		Prompt: "hola",
		Stream: new(bool),
	}
	responseHandler := func(r api.GenerateResponse) error { return nil }

	// Ejecuta el calentamiento
	if err := client.Generate(ctx, req, responseHandler); err != nil {
		// No usamos log.Fatal, pero sí informamos del error.
		fmt.Fprintf(os.Stderr, "Advertencia: Fallo al 'calentar' el modelo: %v\n", err)
	}
}

// --- chooseModel (Sin cambios) ---
func chooseModel(client *api.Client, scanner *bufio.Scanner) string {
	fmt.Println("Consultando modelos de Ollama disponibles...")
	ctx := context.Background()
	resp, err := client.List(ctx)
	if err != nil {
		log.Fatalf("Error fatal: No se pudo listar los modelos de Ollama: %v", err)
	}
	if len(resp.Models) == 0 {
		log.Fatal("Error fatal: No tienes ningún modelo de Ollama descargado. (Usa 'ollama pull ...')")
	}
	fmt.Println("--- Elige un modelo de IA ---")
	for i, model := range resp.Models {
		fmt.Printf("%d: %s\n", i+1, model.Name)
	}
	fmt.Println("------------------------------")
	var choice int
	for {
		fmt.Print("Introduce el número del modelo: ")
		if !scanner.Scan() {
			log.Fatal("Error al leer la selección.")
		}
		input := scanner.Text()
		choice, err = strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(resp.Models) {
			fmt.Println("Selección inválida. Introduce un número de la lista.")
		} else {
			break
		}
	}
	return resp.Models[choice-1].Name
}

// --- FUNCIÓN PRINCIPAL (MODIFICADA) ---
func main() {
	loadLogos()
	createColorMap()

	// 1. Limpia la pantalla para el menú
	clearScreen()

	client, err := api.ClientFromEnvironment()
	if err != nil {
		log.Fatal("Error fatal: No se pudo crear el cliente de Ollama. ¿Está Ollama corriendo?", err)
	}

	scanner := bufio.NewScanner(os.Stdin)

	// 2. Muestra el menú y el usuario elige
	selectedModel := chooseModel(client, scanner)

	// --- ¡NUEVO FLUJO DE INICIO CON FEEDBACK! ---
	// 3. Borra la terminal (después de elegir)
	clearScreen()

	// 4. Muestra el feedback de carga
	fmt.Printf("Cargando modelo \"%s\" en memoria...\n(Esto puede tardar unos segundos)\n", selectedModel)

	// 5. Ejecuta el calentamiento (la parte lenta)
	warmUpModel(client, selectedModel)

	// 6. Borra la terminal DE NUEVO (para quitar el mensaje de "Cargando...")
	clearScreen()

	// 7. Muestra el logo
	printLogo(selectedModel)
	// --- FIN DEL NUEVO FLUJO ---

	var alwaysExecute bool = false
	var isFirstLoop bool = true

	for {
		if isFirstLoop {
			isFirstLoop = false
			fmt.Println()
		} else {
			fmt.Println("──────────────────────────────────────────────────")
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Print("ia> (error dir) >>> ")
		} else {
			home, err := os.UserHomeDir()
			if err == nil && strings.HasPrefix(cwd, home) {
				cwd = "~" + strings.TrimPrefix(cwd, home)
			}
			if alwaysExecute {
				fmt.Printf("ia (auto) [%s]> %s >>> ", selectedModel, cwd)
			} else {
				fmt.Printf("ia [%s]> %s >>> ", selectedModel, cwd)
			}
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil && err != io.EOF {
				fmt.Println("Error al leer la entrada:", err)
			}
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "cd ") {
			dir := strings.TrimSpace(strings.TrimPrefix(input, "cd "))
			if dir == "" || dir == "~" {
				home, err := os.UserHomeDir()
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error al encontrar el home dir:", err)
					continue
				}
				dir = home
			}
			if err := os.Chdir(dir); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			continue

			// --- ¡NUEVO FLUJO PARA //model TAMBIÉN! ---
		} else if input == "//model" {
			selectedModel = chooseModel(client, scanner)
			clearScreen() // 1. Borra
			fmt.Printf("Cargando modelo \"%s\" en memoria...\n(Esto puede tardar unos segundos)\n", selectedModel) // 2. Feedback
			warmUpModel(client, selectedModel) // 3. Calienta
			clearScreen() // 4. Borra de nuevo
			printLogo(selectedModel) // 5. Muestra logo
			isFirstLoop = true
			continue

		} else if input == "//ask" {
			alwaysExecute = false
			fmt.Println("IA> Modo auto-Llamada desactivado. Se pedirá confirmación.")
			fmt.Println()
			continue
		} else if strings.HasPrefix(input, "//") {
			prompt := strings.TrimPrefix(input, "//")
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Println("IA> Petición de IA vacía. Escribe // seguido de tu consulta.")
				fmt.Println()
				continue
			}
			if alwaysExecute {
				handleIACommandAuto(client, selectedModel, prompt)
			} else {
				if handleIACommandConfirm(client, scanner, selectedModel, prompt) {
					alwaysExecute = true
				}
			}
		} else {
			fmt.Println()
			cmd := exec.Command("bash", "-c", input)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			fmt.Println()
		}
	}
	fmt.Println("\n¡Adiós!")
}

// --- Funciones sin cambios (Sanitize, Auto, Confirm) ---
// (Estas 3 funciones son idénticas al Paso 7.2)

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
func handleIACommandAuto(client *api.Client, modelName string, userPrompt string) {
	systemPrompt := `Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `
	fullPrompt := systemPrompt + userPrompt
	fmt.Println("IA> Procesando (auto)...")
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
		fmt.Fprintln(os.Stderr, "Error al contactar con Ollama:", err)
		return
	}
	comandoSugerido := sanitizeIACommand(resp.Response)
	fmt.Println()
	fmt.Println("ejecutando (auto):")
	fmt.Println(comandoSugerido)
	fmt.Println()
	cmd := exec.Command("bash", "-c", comandoSugerido)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "IA> El comando falló.")
	}
	fmt.Println()
}
func handleIACommandConfirm(client *api.Client, scanner *bufio.Scanner, modelName string, userPrompt string) bool {
	systemPrompt := `Eres un experto en terminal de Linux y shell.
	Traduce la siguiente petición de lenguaje natural a un ÚNICO comando de shell.
	Responde SÓLO con el comando y nada más. No uses markdown, ni explicaciones.
	Petición: `
	fullPrompt := systemPrompt + userPrompt
	fmt.Println("IA> Procesando... (contactando a Ollama)")
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
		fmt.Fprintln(os.Stderr, "Error al contactar con Ollama:", err)
		return false
	}
	comandoSugerido := sanitizeIACommand(resp.Response)
	fmt.Println("---")
	fmt.Println("IA> Comando sugerido:")
	fmt.Printf("\n%s\n\n", comandoSugerido)
	fmt.Println("---")
	fmt.Print("IA> ¿Ejecutar? [s/N/X (Siempre)]: ")
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil && err != io.EOF {
			fmt.Println("Error al leer la confirmación:", err)
		}
		return false
	}
	confirmacion := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(confirmacion) {
		case "s":
			fmt.Println("IA> Ejecutando...")
			fmt.Println()
			fmt.Println("ejecutando:")
			fmt.Println(comandoSugerido)
			fmt.Println()
			cmd := exec.Command("bash", "-c", comandoSugerido)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "IA> El comando falló.")
			}
			fmt.Println()
			return false
		case "x":
			fmt.Println("IA> Ejecutando y activando modo 'auto'...")
			fmt.Println()
			fmt.Println("ejecutando:")
			fmt.Println(comandoSugerido)
			fmt.Println()
			cmd := exec.Command("bash", "-c", comandoSugerido)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "IA> El comando falló.")
			}
			fmt.Println()
			fmt.Println("IA> Modo auto-ejecución activado. Escribe '//ask' para desactivarlo.")
			fmt.Println()
			return true
		default:
			fmt.Println("IA> Cancelado.")
			fmt.Println()
			return false
	}
}
