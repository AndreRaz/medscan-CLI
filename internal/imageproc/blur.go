package imageproc

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
)

// BlurScore calcula la nitidez de una imagen usando la Varianza del Laplaciano.
// Score alto = imagen nítida. Score bajo = imagen borrosa.
// Se aplica sobre la imagen ORIGINAL para reflejar la calidad real de la captura (ADR-003).
func BlurScore(imagePath string) (float64, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return 0, fmt.Errorf("abrir imagen para blur check: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return 0, fmt.Errorf("decodificar imagen para blur check: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// Convertir a escala de grises manualmente
	gray := make([][]float64, height)
	for y := 0; y < height; y++ {
		gray[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b, _ := c.RGBA()
			// Fórmula luminancia estándar
			lum := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
			gray[y][x] = lum
		}
	}

	// Máscara rápida: convertir a color.Gray para usar en convolución
	grayImg := image.NewGray(bounds)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			grayImg.SetGray(bounds.Min.X+x, bounds.Min.Y+y, color.Gray{Y: uint8(gray[y][x])})
		}
	}

	// Kernel Laplaciano 3x3: [0,1,0],[1,-4,1],[0,1,0]
	kernel := [3][3]float64{
		{0, 1, 0},
		{1, -4, 1},
		{0, 1, 0},
	}

	// Aplicar convolución manual y calcular varianza
	var values []float64
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			var conv float64
			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					pixVal := float64(grayImg.GrayAt(bounds.Min.X+x+kx, bounds.Min.Y+y+ky).Y)
					conv += pixVal * kernel[ky+1][kx+1]
				}
			}
			values = append(values, conv)
		}
	}

	if len(values) == 0 {
		return 0, nil
	}

	// Calcular media
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calcular varianza
	var varianceSum float64
	for _, v := range values {
		diff := v - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(values))

	return math.Round(variance*100) / 100, nil
}
