//go:build ignore

// Genera una imagen de prueba ficticia de expediente médico.
// Uso: go run testdata/gen_testimage.go
package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func main() {
	// Crear imagen blanca 800x1000
	img := imaging.New(800, 1000, color.White)
	rgba := image.NewRGBA(img.Bounds())
	for y := 0; y < 1000; y++ {
		for x := 0; x < 800; x++ {
			rgba.Set(x, y, color.White)
		}
	}

	// Escribir texto de expediente ficticio
	lines := []string{
		"CLINICA SALUD PLUS - RFC: CSP850101XXX",
		"",
		"EXPEDIENTE CLINICO",
		"------------------------------",
		"Paciente: Juan Perez Garcia",
		"CURP: PEGJ850101HMCRRS09",
		"NSS: 12345678901",
		"Fecha nac.: 1985-01-01",
		"Tel.: 555-123-4567",
		"Domicilio: Av. Reforma 123, CDMX",
		"",
		"Doctor: Dra. Maria Lopez Ruiz",
		"Cedula: 1234567",
		"Especialidad: Medicina General",
		"",
		"Fecha: 2026-03-28",
		"",
		"Diagnostico: Infeccion respiratoria aguda",
		"Sintomas: Tos, fiebre 38.5C, dolor de garganta",
		"Notas: Reposo 3 dias, abundantes liquidos",
		"",
		"TRATAMIENTO:",
		"1. Amoxicilina 500mg c/8h por 7 dias",
		"2. Paracetamol 500mg c/6h si hay fiebre",
		"3. Loratadina 10mg cada 24h por 5 dias",
	}

	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
	}

	y := 40
	for _, line := range lines {
		d.Dot = fixed.P(40, y)
		d.DrawString(line)
		y += 20
	}

	f, err := os.Create("testdata/expediente_ficticio.jpg")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	jpeg.Encode(f, rgba, &jpeg.Options{Quality: 90})
	println("Imagen creada: testdata/expediente_ficticio.jpg")
}
