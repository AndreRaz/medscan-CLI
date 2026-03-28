package imageproc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/disintegration/imaging"
)

// Preprocess aplica el pipeline local de mejora de imagen antes de enviarlo al LLM.
// Pipeline: escala de grises → contraste → resize (si excede maxWidth).
// La imagen resultante se guarda en os.TempDir().
// El caller es responsable de llamar os.Remove(outputPath) con defer.
func Preprocess(inputPath string) (string, error) {
	img, err := imaging.Open(inputPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("abrir imagen para preprocesar: %w", err)
	}

	// 1. Escala de grises
	gray := imaging.Grayscale(img)

	// 2. Contraste (configurable, default 1.3)
	contrastFactor := getContrastFactor()
	contrasted := imaging.AdjustContrast(gray, float64(contrastFactor))

	// 3. Resize si el ancho supera el máximo permitido
	maxWidth := getMaxWidth()
	result := contrasted
	if contrasted.Bounds().Max.X > maxWidth {
		result = imaging.Resize(contrasted, maxWidth, 0, imaging.Lanczos)
	}

	// 4. Guardar en directorio temporal (no contaminar carpeta del usuario — ADR-006)
	tmpPath := filepath.Join(os.TempDir(), "medscan_"+filepath.Base(inputPath))
	if err := imaging.Save(result, tmpPath); err != nil {
		return "", fmt.Errorf("guardar imagen preprocesada: %w", err)
	}

	return tmpPath, nil
}

// getContrastFactor lee MEDISCAN_CONTRAST del entorno, default 1.3.
func getContrastFactor() float32 {
	raw := os.Getenv("MEDISCAN_CONTRAST")
	if raw == "" {
		return 1.3
	}
	v, err := strconv.ParseFloat(raw, 32)
	if err != nil {
		return 1.3
	}
	return float32(v)
}

// getMaxWidth lee MEDISCAN_MAX_WIDTH del entorno, default 1200.
func getMaxWidth() int {
	raw := os.Getenv("MEDISCAN_MAX_WIDTH")
	if raw == "" {
		return 1200
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 1200
	}
	return v
}
