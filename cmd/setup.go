package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Asistente de configuración inicial de medscan",
	Long: `Configura medscan de forma interactiva.

El asistente te pedirá:
  · Proveedor de LLM (Gemini gratuito o Anthropic)
  · API key del proveedor elegido
  · Ruta de la base de datos
  · Umbral de detección de imágenes borrosas

Al finalizar, crea o actualiza el archivo .env en el directorio actual.`,
	// setup no necesita DB abierta, sobreescribimos PersistentPreRunE
	PersistentPreRunE:  func(cmd *cobra.Command, args []string) error { return nil },
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		clearScreen()
		printBanner()

		fmt.Println("Bienvenido al asistente de configuración de medscan.")
		fmt.Println("Responde las preguntas para generar tu archivo .env")
		fmt.Println()

		// ── 1. Proveedor ────────────────────────────────────────────────────
		fmt.Println("┌─────────────────────────────────────────────────────────────┐")
		fmt.Println("│  Paso 1/4 — Proveedor de LLM                               │")
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("  [1] Google Gemini 2.5 Flash  (GRATIS, no requiere tarjeta)")
		fmt.Println("      → Obtén tu key en https://aistudio.google.com")
		fmt.Println()
		fmt.Println("  [2] Anthropic Claude          (Mayor precisión, requiere pago)")
		fmt.Println("      → Obtén tu key en https://console.anthropic.com")
		fmt.Println()
		provider := promptSelect(reader, "Elige una opción [1/2]", []string{"1", "2"}, "1")

		providerName := "gemini"
		if provider == "2" {
			providerName = "anthropic"
		}

		// ── 2. API Key ──────────────────────────────────────────────────────
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────┐")
		fmt.Println("│  Paso 2/4 — API Key                                        │")
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
		fmt.Println()

		var apiKeyEnvVar, apiKey string
		if providerName == "gemini" {
			fmt.Println("  Obtener key gratuita:")
			fmt.Println("  1. Ir a https://aistudio.google.com")
			fmt.Println("  2. Clic en \"Get API key\" → crear proyecto → copiar key")
			fmt.Println()
			apiKeyEnvVar = "GEMINI_API_KEY"
			apiKey = promptRequired(reader, "Pega tu GEMINI_API_KEY")
		} else {
			fmt.Println("  Obtener key:")
			fmt.Println("  1. Ir a https://console.anthropic.com")
			fmt.Println("  2. Settings → API Keys → Create Key")
			fmt.Println()
			apiKeyEnvVar = "ANTHROPIC_API_KEY"
			apiKey = promptRequired(reader, "Pega tu ANTHROPIC_API_KEY")
		}

		// Validar key
		fmt.Println()
		fmt.Print("  Validando API key... ")
		if err := validateAPIKey(providerName, apiKey); err != nil {
			fmt.Println("")
			fmt.Printf("  La key no pudo verificarse: %v\n", err)
			fmt.Println("  Puedes continuar de todas formas, pero verifica la key antes de usar medscan.")
			if !promptYesNo(reader, "¿Continuar de todas formas?", true) {
				fmt.Println("Configuración cancelada.")
				return nil
			}
		} else {
			fmt.Println("Key válida")
		}

		// ── 3. Rutas ────────────────────────────────────────────────────────
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────┐")
		fmt.Println("│  Paso 3/4 — Configuración de rutas                         │")
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
		fmt.Println()

		dbPath := promptWithDefault(reader, "Ruta del archivo de base de datos", "./mediscan.db")

		// Crear directorio de la DB si no existe
		dbDir := filepath.Dir(dbPath)
		if dbDir != "." && dbDir != "" {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				fmt.Printf("  No se pudo crear el directorio %s: %v\n", dbDir, err)
			}
		}

		// ── 4. Parámetros avanzados ─────────────────────────────────────────
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────┐")
		fmt.Println("│  Paso 4/4 — Parámetros de imagen (opcional)                │")
		fmt.Println("└─────────────────────────────────────────────────────────────┘")
		fmt.Println()
		fmt.Println("  El umbral de blur rechaza imágenes borrosas antes de enviarlas a la API.")
		fmt.Println("  Referencia: foto en celular con buena luz ≈ 300–800, muy borrosa < 80")
		fmt.Println()

		blurThresholdStr := promptWithDefault(reader, "Umbral de borrosidad (0 = desactivado)", "100.0")
		maxWidthStr := promptWithDefault(reader, "Ancho máximo en píxeles (reduce tokens)", "1200")

		// ── Escribir .env ───────────────────────────────────────────────────
		envPath := ".env"
		if _, err := os.Stat(envPath); err == nil {
			fmt.Println()
			if !promptYesNo(reader, fmt.Sprintf("Ya existe %s. ¿Sobreescribir?", envPath), false) {
				fmt.Println("Configuración cancelada. Tu .env anterior no fue modificado.")
				return nil
			}
		}

		model := "gemini-2.5-flash"
		if providerName == "anthropic" {
			model = "claude-opus-4-5"
		}
		modelEnvVar := "MEDISCAN_GEMINI_MODEL"
		if providerName == "anthropic" {
			modelEnvVar = "MEDISCAN_ANTHROPIC_MODEL"
		}

		envContent := fmt.Sprintf(`# medscan — configuración generada por 'medscan setup'
# Generado: %s

# Proveedor de LLM
MEDISCAN_PROVIDER=%s

# API Key
%s=%s

# Modelo
%s=%s

# Base de datos
MEDISCAN_DB_PATH=%s

# Imagen
MEDISCAN_MAX_WIDTH=%s
MEDISCAN_CONTRAST=1.3
MEDISCAN_BLUR_THRESHOLD=%s

# Logging (debug | info | warn | error)
MEDISCAN_LOG_LEVEL=info
`, nowString(), providerName, apiKeyEnvVar, apiKey, modelEnvVar, model, dbPath, maxWidthStr, blurThresholdStr)

		if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
			return fmt.Errorf("error escribiendo .env: %w", err)
		}

		// ── Resumen final ───────────────────────────────────────────────────
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("medscan configurado correctamente")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Printf("  Proveedor:   %s\n", providerName)
		fmt.Printf("  Modelo:      %s\n", model)
		fmt.Printf("  Base datos:  %s\n", dbPath)
		fmt.Printf("  Blur umbral: %s\n", blurThresholdStr)
		fmt.Println()
		fmt.Println("Próximos pasos:")
		fmt.Println()
		fmt.Println("  Digitalizar una carpeta de documentos:")
		fmt.Println("    medscan scan /ruta/a/tus/imagenes/")
		fmt.Println()
		fmt.Println("  Ver todos los pacientes registrados:")
		fmt.Println("    medscan patient list")
		fmt.Println()
		fmt.Println("  Ver estadísticas de la base de datos:")
		fmt.Println("    medscan db stats")
		fmt.Println()

		// ── Instalar globalmente si se pasó --install ───────────────────────
		if setupInstall {
			if err := installBinary(); err != nil {
				fmt.Printf("\n  Advertencia: no se pudo instalar globalmente: %v\n", err)
			}
		}

		return nil
	},
}

var setupInstall bool

// installCmd es un subcomando dedicado para instalar medscan globalmente.
// No requiere pasar por el wizard de configuración.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Instala medscan globalmente para ejecutarlo desde cualquier terminal",
	Long: `Instala el binario de medscan en ~/.local/bin (sin necesitar permisos de root).

Al finalizar, podrás ejecutar 'medscan' desde cualquier directorio
sin necesidad de escribir './medscan' o buscar el archivo.

Linux / macOS:
  Copia el binario a ~/.local/bin/medscan y añade esa ruta al PATH
  en ~/.bashrc y ~/.zshrc (si existen).

Windows:
  Imprime instrucciones para añadir el directorio al PATH del sistema.`,
	// El subcomando install no necesita la DB
	PersistentPreRunE:  func(cmd *cobra.Command, args []string) error { return nil },
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		return installBinary()
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupInstall, "install", false, "Instala medscan globalmente en ~/.local/bin (Linux/macOS)")
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(installCmd)
}


// ── helpers de entrada interactiva ──────────────────────────────────────────

func prompt(reader *bufio.Reader, question string) string {
	fmt.Printf("  %s: ", question)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptRequired(reader *bufio.Reader, question string) string {
	for {
		val := prompt(reader, question)
		if val != "" {
			return val
		}
		fmt.Println("  Este campo es obligatorio.")
	}
}

func promptWithDefault(reader *bufio.Reader, question, defaultVal string) string {
	fmt.Printf("  %s [%s]: ", question, defaultVal)
	text, _ := reader.ReadString('\n')
	val := strings.TrimSpace(text)
	if val == "" {
		return defaultVal
	}
	return val
}

func promptSelect(reader *bufio.Reader, question string, options []string, defaultVal string) string {
	for {
		val := promptWithDefault(reader, question, defaultVal)
		for _, opt := range options {
			if val == opt {
				return val
			}
		}
		fmt.Printf("  Opción inválida. Elige entre: %s\n", strings.Join(options, ", "))
	}
}

func promptYesNo(reader *bufio.Reader, question string, defaultYes bool) bool {
	def := "s/N"
	if defaultYes {
		def = "S/n"
	}
	fmt.Printf("  %s [%s]: ", question, def)
	text, _ := reader.ReadString('\n')
	val := strings.ToLower(strings.TrimSpace(text))
	if val == "" {
		return defaultYes
	}
	return val == "s" || val == "si" || val == "y" || val == "yes"
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════╗")
	fmt.Println("  ║     medscan — Digitalizador de Expedientes   ║")
	fmt.Println("  ║          Asistente de Configuración          ║")
	fmt.Println("  ╚══════════════════════════════════════════════╝")
	fmt.Println()
}

func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ── validación de API key ───────────────────────────────────────────────────

func validateAPIKey(provider, key string) error {
	switch provider {
	case "gemini":
		return validateGeminiKey(key)
	case "anthropic":
		return validateAnthropicKey(key)
	}
	return nil
}

// validateGeminiKey hace un listado de modelos para verificar la key.
func validateGeminiKey(key string) error {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", key)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a Gemini: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 400 || resp.StatusCode == 403 {
		return fmt.Errorf("API key inválida (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("respuesta inesperada de Gemini (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// validateAnthropicKey hace una llamada mínima para verificar la key.
func validateAnthropicKey(key string) error {
	// Imagen 1x1 PNG en base64 para el test
	onePxPNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	body := map[string]interface{}{
		"model":      "claude-haiku-20240307",
		"max_tokens": 10,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "image", "source": map[string]string{
						"type":       "base64",
						"media_type": "image/png",
						"data":       onePxPNG,
					}},
					{"type": "text", "text": "ok"},
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a Anthropic: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return fmt.Errorf("API key inválida")
	}
	if resp.StatusCode == 403 {
		return fmt.Errorf("sin permisos (HTTP 403)")
	}
	_ = b
	return nil
}

// ── instalación global ──────────────────────────────────────────────────────

// installBinary instala el binario de medscan de forma global en el sistema
// sin necesitar permisos de administrador (root/sudo).
//
// Linux/macOS: copia a ~/.local/bin y añade la ruta al PATH en el shell.
// Windows:     imprime instrucciones claras para el usuario.
func installBinary() error {
	goos := runtime.GOOS

	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("  │  Instalación global de medscan                             │")
	fmt.Println("  └─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	if goos == "windows" {
		return installBinaryWindows()
	}
	return installBinaryUnix()
}

// installBinaryUnix instala medscan en ~/.local/bin (Linux y macOS).
func installBinaryUnix() error {
	// Ruta del ejecutable actual
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo determinar la ruta del ejecutable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("no se pudo resolver el ejecutable: %w", err)
	}

	home, _ := os.UserHomeDir()
	binDir := filepath.Join(home, ".local", "bin")
	dest := filepath.Join(binDir, "medscan")

	// Crear ~/.local/bin si no existe
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", binDir, err)
	}

	// Copiar el binario
	if err := setupCopyFile(exePath, dest); err != nil {
		return fmt.Errorf("error copiando el binario: %w", err)
	}

	// Dar permisos de ejecución
	if err := os.Chmod(dest, 0755); err != nil {
		return fmt.Errorf("error asignando permisos: %w", err)
	}

	fmt.Printf("  Binario instalado en: %s\n\n", dest)

	// Añadir ~/.local/bin al PATH en los shells más comunes
	pathLine := `export PATH="$HOME/.local/bin:$PATH"`
	addedToAny := false

	for _, shellFile := range []string{".bashrc", ".zshrc", ".profile"} {
		shellPath := filepath.Join(home, shellFile)
		if _, err := os.Stat(shellPath); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(shellPath)
		if err != nil {
			continue
		}

		// No duplicar si ya está
		if strings.Contains(string(content), pathLine) {
			fmt.Printf("  PATH ya configurado en ~/%s\n", shellFile)
			addedToAny = true
			continue
		}

		entry := "\n# medscan — instalado por 'medscan setup --install'\n" + pathLine + "\n"
		f, err := os.OpenFile(shellPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("  No se pudo escribir en ~/%s: %v\n", shellFile, err)
			continue
		}
		_, _ = f.WriteString(entry)
		_ = f.Close()
		fmt.Printf("  PATH actualizado en ~/%s\n", shellFile)
		addedToAny = true
	}

	fmt.Println()
	if addedToAny {
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  medscan instalado correctamente")
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Println("  Para que surta efecto en la sesión actual, ejecuta:")
		fmt.Println()
		fmt.Printf("    source ~/.bashrc   (o ~/.zshrc según tu shell)\n")
		fmt.Println()
		fmt.Println("  Después podrás ejecutar simplemente:")
		fmt.Println()
		fmt.Println("    medscan")
		fmt.Println()
	} else {
		fmt.Printf("  El binario está en: %s\n", dest)
		fmt.Println("  Añade manualmente esta línea a tu shell de configuración:")
		fmt.Println()
		fmt.Printf("    %s\n", pathLine)
		fmt.Println()
	}

	return nil
}

// installBinaryWindows imprime instrucciones para agregar medscan al PATH en Windows.
func installBinaryWindows() error {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "C:\\ruta\\a\\medscan.exe"
	}
	exeDir := filepath.Dir(exePath)

	fmt.Println("  En Windows, sigue estos pasos para ejecutar medscan desde cualquier terminal:")
	fmt.Println()
	fmt.Println("  Opción 1 — PowerShell (sin admin, solo para tu usuario):")
	fmt.Println()
	fmt.Printf("    $env:Path += ';%s'\n", exeDir)
	fmt.Println("    [Environment]::SetEnvironmentVariable('Path', $env:Path, 'User')")
	fmt.Println()
	fmt.Println("  Opción 2 — Copiar medscan.exe a una carpeta ya en el PATH:")
	fmt.Println()
	fmt.Println("    C:\\Windows\\System32\\   (requiere admin)")
	fmt.Println("    C:\\Users\\TuUsuario\\AppData\\Local\\Microsoft\\WindowsApps\\")
	fmt.Println()
	fmt.Printf("  Directorio actual del ejecutable: %s\n", exeDir)
	fmt.Println()
	return nil
}

// setupCopyFile copia src → dst, usado para instalar el binario.
func setupCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

