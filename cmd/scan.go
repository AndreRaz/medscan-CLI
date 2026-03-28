package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"medscan/internal/pipeline"
	"medscan/internal/transcriber"
)

// Flags del comando scan
var (
	debugBlur     bool
	blurThreshold float64
)

var scanCmd = &cobra.Command{
	Use:   "scan [carpeta]",
	Short: "Digitaliza todos los documentos médicos en una carpeta",
	Long: `Recorre una carpeta con fotos o escaneos de documentos médicos y ejecuta
el pipeline completo para cada archivo:

  1. Validación de formato y tamaño
  2. Deduplicación por SHA-256
  3. Detección de borrosidad (Varianza del Laplaciano)
  4. Pre-procesamiento local (escala de grises, contraste, resize)
  5. Transcripción con LLM (Gemini o Anthropic)
  6. Persistencia en SQLite

Al final imprime un resumen: procesados / rechazados / errores / duplicados.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		folder := args[0]

		// Validar que la carpeta exista
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			return fmt.Errorf("la carpeta no existe: %s", folder)
		}

		fmt.Printf("Escaneando carpeta: %s\n", folder)

		// Obtener umbral de blur (flag > env var > default)
		threshold := getBlurThreshold(blurThreshold)
		if threshold > 0 {
			fmt.Printf("Umbral de borrosidad: %.1f\n", threshold)
		} else {
			fmt.Println("Detección de borrosidad desactivada (MEDISCAN_BLUR_THRESHOLD=0)")
		}

		t := transcriber.New()

		cfg := pipeline.Config{
			Folder:        folder,
			BlurThreshold: threshold,
			DebugBlur:     debugBlur,
			Db:            db, // global en cmd/root.go
			Transcriber:   t,
		}

		events := make(chan pipeline.ScanEvent)
		go func() {
			_, err := pipeline.RunScanner(cfg, events)
			if err != nil {
				fmt.Printf("Error running scanner: %s\n", err.Error())
			}
		}()

		var (
			countProcessed  int
			countDuplicates int
			countBlurRej    int
			countAPIError   int
			countFormatRej  int
			totalFiles      int
		)

		for evt := range events {
			if totalFiles == 0 && evt.TotalFiles > 0 {
				totalFiles = evt.TotalFiles
			}

			if evt.CurrentFile == 0 && evt.Status == "Rechazado (Formato/Tamaño)" {
				fmt.Printf("  ⛔ %s → %s\n", evt.FileName, evt.ErrorMessage)
				countFormatRej++
				continue
			}

			fmt.Printf("[%d/%d] %s\n", evt.CurrentFile, evt.TotalFiles, evt.FileName)

			if debugBlur && evt.BlurScore > 0 {
				fmt.Printf("  Blur score: %.2f\n", evt.BlurScore)
			} else if !debugBlur && threshold > 0 && evt.BlurScore > 0 {
				fmt.Printf("  [blur:%.1f]", evt.BlurScore)
			}

			switch evt.Status {
			case "Duplicado (Omitido)":
				fmt.Printf("  ✓ Duplicado (SHA-256 ya procesado), omitiendo\n")
				countDuplicates++
			case "Rechazado (Borrosa)":
				fmt.Printf("  Imagen borrosa (score: %.1f < %.1f). Tómela de nuevo.\n", evt.BlurScore, threshold)
				countBlurRej++
			case "Error (Pre-procesamiento)":
				fmt.Printf("  Error en pre-procesamiento: %v\n", evt.ErrorMessage)
				countAPIError++
			case "Error (API Transcripción)":
				fmt.Printf("\n  Error en transcripción: %v\n", evt.ErrorMessage)
				countAPIError++
			case "Error (Base de Datos)":
				fmt.Printf("\n  Error guardando en DB: %v\n", evt.ErrorMessage)
				countAPIError++
			default:
				// Asumimos Procesado
				fmt.Printf("  ✓ %s\n", evt.Status)
				countProcessed++
			}
		}

		// --- Resumen final ---
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Resumen del scan\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("  Procesados:           %d\n", countProcessed)
		fmt.Printf("  Duplicados (omitidos): %d\n", countDuplicates)
		fmt.Printf("  Rechazados por blur:   %d\n", countBlurRej)
		fmt.Printf("  ⛔ Formato/tamaño:        %d\n", countFormatRej)
		fmt.Printf("  Errores de API/DB:     %d\n", countAPIError)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("  Total revisados: %d\n", totalFiles)

		return nil
	},
}

func init() {
	scanCmd.Flags().BoolVar(&debugBlur, "debug-blur", false, "Imprime el blur score sin rechazar imágenes")
	scanCmd.Flags().Float64Var(&blurThreshold, "blur-threshold", 0, "Umbral de blur (0 = usar variable de entorno)")
}

// getBlurThreshold devuelve el umbral efectivo de blur.
// Prioridad: flag > env var > default (100.0).
func getBlurThreshold(flagValue float64) float64 {
	if flagValue > 0 {
		return flagValue
	}
	envVal := os.Getenv("MEDISCAN_BLUR_THRESHOLD")
	if envVal != "" {
		v, err := strconv.ParseFloat(envVal, 64)
		if err == nil {
			return v
		}
	}
	return 100.0
}

// logDebug imprime un mensaje de debug si MEDISCAN_LOG_LEVEL=debug.
func logDebug(format string, args ...interface{}) {
	if os.Getenv("MEDISCAN_LOG_LEVEL") == "debug" {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}
