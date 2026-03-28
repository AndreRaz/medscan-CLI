package cmd

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
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
	// Fecha simple sin imports extra de time en este scope
	return "ver 1.0"
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

	// Decodificar base64 de la imagen para validar que es correcto
	_ = base64.StdEncoding.EncodeToString([]byte("test"))

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
