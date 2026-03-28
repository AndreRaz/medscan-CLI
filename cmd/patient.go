package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var patientCmd = &cobra.Command{
	Use:   "patient",
	Short: "Operaciones sobre pacientes",
}

var patientListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista todos los pacientes en la base de datos",
	RunE: func(cmd *cobra.Command, args []string) error {
		patients, err := db.ListPatients()
		if err != nil {
			return fmt.Errorf("error listando pacientes: %w", err)
		}

		if len(patients) == 0 {
			fmt.Println("No hay pacientes en la base de datos.")
			fmt.Println("Usa 'medscan scan <carpeta>' para digitalizar expedientes.")
			return nil
		}

		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("👤 Pacientes registrados (%d)\n", len(patients))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("%-4s  %-35s  %-22s  %s\n", "ID", "Nombre", "CURP", "Fecha nac.")
		fmt.Printf("%-4s  %-35s  %-22s  %s\n", "────", strings.Repeat("─", 35), strings.Repeat("─", 22), "──────────")

		for _, p := range patients {
			curp := p.CURP
			if curp == "" {
				curp = "(sin CURP)"
			}
			fmt.Printf("%-4d  %-35s  %-22s  %s\n", p.ID, truncate(p.Nombre, 35), truncate(curp, 22), p.FechaNacimiento)
		}

		fmt.Printf("\nUsa: medscan query <CURP>  para ver el expediente completo de un paciente.\n")
		return nil
	},
}

func init() {
	patientCmd.AddCommand(patientListCmd)
}

// truncate trunca un string a maxLen caracteres, añadiendo "..." si se trunca.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
