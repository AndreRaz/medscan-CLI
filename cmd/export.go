package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"medscan/internal/parser"
)

var exportCmd = &cobra.Command{
	Use:   "export [curp]",
	Short: "Exporta el expediente de un paciente a JSON",
	Long: `Exporta el expediente completo de un paciente en formato JSON.

Ejemplos:
  medscan export GACM850101HMCRLS09
  medscan export GACM850101HMCRLS09 --output expediente.json
  medscan export --id 2 --output filomena.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")
		exportID, _ := cmd.Flags().GetInt64("id")

		var (
			exp          *parser.Expediente
			visitas      []parser.Visita
			tratamientos [][]parser.Tratamiento
			err          error
		)

		if exportID > 0 {
			exp, visitas, tratamientos, err = db.GetExpedienteByID(exportID)
		} else if len(args) == 1 {
			curp := strings.ToUpper(args[0])
			exp, visitas, tratamientos, err = db.GetExpediente(curp)
		} else {
			return fmt.Errorf("proporciona un CURP o usa --id <número> (ver IDs con 'medscan db visitas')")
		}
		if err != nil {
			return fmt.Errorf("error consultando expediente: %w", err)
		}

		// Construir el objeto completo de exportación
		type VisitaCompleta struct {
			parser.Visita
			Tratamiento []parser.Tratamiento `json:"tratamiento"`
		}
		type ExportData struct {
			Paciente parser.Paciente  `json:"paciente"`
			Visitas  []VisitaCompleta `json:"visitas"`
		}

		var visitasCompletas []VisitaCompleta
		for i, v := range visitas {
			vc := VisitaCompleta{Visita: v}
			if i < len(tratamientos) {
				vc.Tratamiento = tratamientos[i]
			}
			visitasCompletas = append(visitasCompletas, vc)
		}

		exportData := ExportData{
			Paciente: exp.Paciente,
			Visitas:  visitasCompletas,
		}

		jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			return fmt.Errorf("error serializando JSON: %w", err)
		}

		if outputFile != "" {
			if err := os.WriteFile(outputFile, jsonBytes, 0644); err != nil {
				return fmt.Errorf("error escribiendo archivo: %w", err)
			}
			fmt.Printf("Expediente exportado a: %s\n", outputFile)
		} else {
			fmt.Println(string(jsonBytes))
		}

		return nil
	},
}

func init() {
	exportCmd.Flags().StringP("output", "o", "", "Archivo de salida (default: stdout)")
	exportCmd.Flags().Int64("id", 0, "ID numérico del paciente (para pacientes sin CURP)")
}
