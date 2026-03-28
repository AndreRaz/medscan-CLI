package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"medscan/internal/imageproc"
	"medscan/internal/scanner"
	"medscan/internal/transcriber"
)

// Flags del comando scan
var (
	debugBlur      bool
	blurThreshold  float64
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

		fmt.Printf("📂 Escaneando carpeta: %s\n", folder)

		// Obtener umbral de blur (flag > env var > default)
		threshold := getBlurThreshold(blurThreshold)
		if threshold > 0 {
			fmt.Printf("🔍 Umbral de borrosidad: %.1f\n", threshold)
		} else {
			fmt.Println("⚠  Detección de borrosidad desactivada (MEDISCAN_BLUR_THRESHOLD=0)")
		}

		// Obtener archivos válidos de la carpeta
		files, formatRejected, err := scanner.ScanFolder(folder)
		if err != nil {
			return fmt.Errorf("error al escanear carpeta: %w", err)
		}

		// Registrar archivos con formato/tamaño inválido
		for _, rej := range formatRejected {
			fmt.Printf("  ⛔ %s → %s\n", filepath.Base(rej.Path), rej.Reason)
			if dbErr := db.SaveRejectedFile(rej.Path, "", "formato", 0); dbErr != nil {
				logDebug("No se pudo registrar rechazo: %v", dbErr)
			}
		}

		if len(files) == 0 {
			fmt.Println("No se encontraron imágenes válidas en la carpeta.")
			return nil
		}

		fmt.Printf("📄 %d imagen(es) encontrada(s) para procesar\n\n", len(files))

		t := transcriber.New()

		// Contadores para el resumen final
		var (
			countProcessed  int
			countDuplicates int
			countBlurRej    int
			countAPIError   int
			countFormatRej  = len(formatRejected)
		)

		for i, file := range files {
			startTime := time.Now()
			baseName := filepath.Base(file.Path)

			fmt.Printf("[%d/%d] %s\n", i+1, len(files), baseName)

			// --- Paso 1: Deduplicación por hash ---
			exists, err := db.HashExists(file.Hash)
			if err != nil {
				logDebug("Error verificando hash: %v", err)
			}
			if exists {
				fmt.Printf("  ✓ Duplicado (SHA-256 ya procesado), omitiendo\n")
				countDuplicates++
				continue
			}

			// --- Paso 2: Detección de borrosidad ---
			blurScore, err := imageproc.BlurScore(file.Path)
			if err != nil {
				fmt.Printf("  ⚠  No se pudo calcular blur score: %v\n", err)
			}

			if debugBlur {
				fmt.Printf("  🔍 Blur score: %.2f\n", blurScore)
			}

			if threshold > 0 && blurScore < threshold {
				fmt.Printf("  ⚠  Imagen borrosa (score: %.1f < %.1f). Tómela de nuevo.\n", blurScore, threshold)
				if err := db.SaveRejectedFile(file.Path, file.Hash, "blur", blurScore); err != nil {
					logDebug("Error guardando rechazo blur: %v", err)
				}
				countBlurRej++
				continue
			}

			// Mostrar estado de blur si pasó
			if !debugBlur && threshold > 0 {
				fmt.Printf("  [blur:%.1f]", blurScore)
			}

			// --- Paso 3: Pre-procesamiento local ---
			processedPath, err := imageproc.Preprocess(file.Path)
			if err != nil {
				fmt.Printf("  ❌ Error en pre-procesamiento: %v\n", err)
				if dbErr := db.SaveFailedFile(file.Path, file.Hash, "preprocesamiento: "+err.Error()); dbErr != nil {
					logDebug("Error guardando failed_file: %v", dbErr)
				}
				countAPIError++
				continue
			}
			defer os.Remove(processedPath) // limpiar siempre — ADR-006

			// Obtener ancho resultante para el log
			fmt.Printf(" [procesado]")

			// --- Paso 4: Transcripción con LLM ---
			fmt.Printf(" [enviando a LLM...]")
			exp, err := t.Transcribe(processedPath)
			if err != nil {
				fmt.Printf("\n  ❌ Error en transcripción: %v\n", err)
				if dbErr := db.SaveFailedFile(file.Path, file.Hash, "transcripción: "+err.Error()); dbErr != nil {
					logDebug("Error guardando failed_file: %v", dbErr)
				}
				countAPIError++
				continue
			}

			// Completar metadatos de la visita
			elapsed := time.Since(startTime).Milliseconds()
			exp.Visita.ArchivoOrigen = file.Path
			exp.Visita.BlurScore = blurScore
			exp.Visita.ProcesadoEnMs = elapsed

			// --- Paso 5: Guardar en DB ---
			if err := db.SaveExpediente(exp, file.Hash); err != nil {
				fmt.Printf("\n  ❌ Error guardando en DB: %v\n", err)
				if dbErr := db.SaveFailedFile(file.Path, file.Hash, "db: "+err.Error()); dbErr != nil {
					logDebug("Error guardando failed_file: %v", dbErr)
				}
				countAPIError++
				continue
			}

			fmt.Printf(" ✅ (%dms)\n", elapsed)

			// Mostrar paciente extraído (sin datos sensibles en logs normales)
			if exp.Paciente.Nombre != "" {
				fmt.Printf("  👤 Paciente: %s\n", exp.Paciente.Nombre)
			} else {
				fmt.Printf("  ⚠  No se pudo extraer nombre del paciente [sin_datos]\n")
			}

			countProcessed++
		}

		// --- Resumen final ---
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📊 Resumen del scan\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("  ✅ Procesados:           %d\n", countProcessed)
		fmt.Printf("  🔁 Duplicados (omitidos): %d\n", countDuplicates)
		fmt.Printf("  ⚠  Rechazados por blur:   %d\n", countBlurRej)
		fmt.Printf("  ⛔ Formato/tamaño:        %d\n", countFormatRej)
		fmt.Printf("  ❌ Errores de API/DB:     %d\n", countAPIError)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("  Total revisados: %d\n", len(files)+len(formatRejected))

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
