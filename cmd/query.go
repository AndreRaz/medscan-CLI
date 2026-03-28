package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"medscan/internal/parser"
)

var queryNombre string
var queryID int64

var queryCmd = &cobra.Command{
	Use:   "query [curp]",
	Short: "Consulta el expediente completo de un paciente",
	Long: `Muestra el expediente completo de un paciente.

Buscar por CURP:
  medscan query GACM850101HMCRLS09

Buscar por nombre (muestra resultados):
  medscan query --nombre "García"

Buscar por ID (para pacientes sin CURP, ver IDs con 'db visitas'):
  medscan query --id 2`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && queryNombre == "" && queryID == 0 {
			return fmt.Errorf("proporciona un CURP, usa --nombre o --id (ver 'medscan db visitas' para los IDs)")
		}

		// Búsqueda por ID numérico (para pacientes sin CURP)
		if queryID > 0 {
			exp, visitas, tratamientos, err := db.GetExpedienteByID(queryID)
			if err != nil {
				return fmt.Errorf("error consultando expediente: %w", err)
			}
			printExpediente(exp, visitas, tratamientos)
			return nil
		}

		// Búsqueda por nombre
		if queryNombre != "" {
			patients, err := db.SearchByNombre(queryNombre)
			if err != nil {
				return fmt.Errorf("error buscando pacientes: %w", err)
			}
			if len(patients) == 0 {
				fmt.Printf("No se encontraron pacientes con nombre '%s'\n", queryNombre)
				return nil
			}
			fmt.Printf("🔍 %d paciente(s) encontrado(s):\n\n", len(patients))
			for _, p := range patients {
				curpDisplay := p.CURP
				if curpDisplay == "" {
					curpDisplay = "(sin CURP)"
				}
				fmt.Printf("  %-30s  CURP: %s\n", p.Nombre, curpDisplay)
			}
			if len(patients) == 1 && patients[0].CURP != "" {
				fmt.Printf("\nUsa: medscan query %s  — para ver el expediente completo\n", patients[0].CURP)
			}
			return nil
		}

		// Búsqueda por CURP
		curp := strings.ToUpper(args[0])
		exp, visitas, tratamientos, err := db.GetExpediente(curp)
		if err != nil {
			return fmt.Errorf("error consultando expediente: %w", err)
		}
		printExpediente(exp, visitas, tratamientos)
		return nil
	},
}

func init() {
	queryCmd.Flags().StringVar(&queryNombre, "nombre", "", "Buscar paciente por nombre")
	queryCmd.Flags().Int64Var(&queryID, "id", 0, "Buscar paciente por ID numérico (para pacientes sin CURP)")
}

// printExpediente imprime el expediente completo de un paciente.
func printExpediente(exp *parser.Expediente, visitas []parser.Visita, tratamientos [][]parser.Tratamiento) {
	curpDisplay := exp.Paciente.CURP
	if curpDisplay == "" {
		curpDisplay = "(sin CURP)"
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Expediente: %s\n", exp.Paciente.Nombre)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("CURP:           %s\n", curpDisplay)
	fmt.Printf("NSS:            %s\n", exp.Paciente.NSS)
	fmt.Printf("Fecha nac.:     %s\n", exp.Paciente.FechaNacimiento)
	fmt.Printf("Teléfono:       %s\n", exp.Paciente.Telefono)
	fmt.Printf("Domicilio:      %s\n", exp.Paciente.Domicilio)
	fmt.Printf("\n%d visita(s) registrada(s):\n", len(visitas))

	for i, v := range visitas {
		fmt.Printf("\n  ── Visita %d (%s) ──────────\n", i+1, v.Fecha)
		fmt.Printf("  Diagnóstico:  %s\n", v.Diagnostico)
		fmt.Printf("  Síntomas:     %s\n", v.Sintomas)
		fmt.Printf("  Notas:        %s\n", v.Notas)
		if len(tratamientos) > i && len(tratamientos[i]) > 0 {
			fmt.Printf("  Tratamiento:\n")
			for _, t := range tratamientos[i] {
				fmt.Printf("    • %s — %s, %s, %s\n", t.Medicamento, t.Dosis, t.Frecuencia, t.Duracion)
				if t.Indicaciones != "" {
					fmt.Printf("      Indicaciones: %s\n", t.Indicaciones)
				}
			}
		}
		fmt.Printf("  [blur:%.1f, %dms]\n", v.BlurScore, v.ProcesadoEnMs)
	}
}

// formatJSON formatea un valor como JSON indentado para display.
func formatJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
