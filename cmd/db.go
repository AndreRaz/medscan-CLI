package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"medscan/internal/store"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Visualiza el contenido de la base de datos",
	Long: `Muestra el contenido completo de la base de datos local SQLite.

Subcomandos:
  medscan db stats        — estadísticas generales
  medscan db visitas      — historial de todas las visitas
  medscan db rechazados   — archivos rechazados (blur, formato, etc.)`,
}

// ── db stats ───────────────────────────────────────────────────────────────

var dbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Estadísticas generales de la base de datos",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, err := db.GetStats()
		if err != nil {
			return fmt.Errorf("error leyendo estadísticas: %w", err)
		}

		// Tamaño del archivo de DB en disco
		dbPath := store.GetDBPath()
		var sizeStr string
		if info, err := os.Stat(dbPath); err == nil {
			sizeStr = formatBytes(info.Size())
		} else {
			sizeStr = "desconocido"
		}

		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Base de datos: %s  (%s)\n", dbPath, sizeStr)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("  Pacientes registrados:   %d\n", stats.TotalPacientes)
		fmt.Printf("  Visitas totales:          %d\n", stats.TotalVisitas)
		fmt.Printf("  Tratamientos registrados: %d\n", stats.TotalTratamientos)
		fmt.Printf("  Archivos rechazados:      %d\n", stats.TotalRechazados)
		fmt.Printf("  Archivos con error:       %d\n", stats.TotalFallidos)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		if stats.TotalPacientes == 0 {
			fmt.Println("La base de datos está vacía. Usa 'medscan scan <carpeta>' para digitalizar documentos.")
		}
		return nil
	},
}

// ── db visitas ─────────────────────────────────────────────────────────────

var dbVisitasLimit int

var dbVisitasCmd = &cobra.Command{
	Use:   "visitas",
	Short: "Muestra el historial de visitas registradas",
	RunE: func(cmd *cobra.Command, args []string) error {
		visitas, err := db.ListVisits(dbVisitasLimit)
		if err != nil {
			return fmt.Errorf("error leyendo visitas: %w", err)
		}

		if len(visitas) == 0 {
			fmt.Println("No hay visitas registradas aún.")
			return nil
		}

		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Historial de visitas (últimas %d)\n", len(visitas))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("%-4s  %-25s  %-22s  %-10s  %-30s  %s\n",
			"ID", "Paciente", "CURP", "Fecha", "Diagnóstico", "")
		fmt.Printf("%-4s  %-25s  %-22s  %-10s  %-30s  %s\n",
			strings.Repeat("─", 4),
			strings.Repeat("─", 25),
			strings.Repeat("─", 22),
			strings.Repeat("─", 10),
			strings.Repeat("─", 30),
			"──")

		for _, v := range visitas {
			curp := v.PacienteCURP
			if curp == "" {
				curp = "(sin CURP)"
			}
			trat := fmt.Sprintf("%d", v.NumTratamientos)
			fmt.Printf("%-4d  %-25s  %-22s  %-10s  %-30s  %s\n",
				v.ID,
				truncate(v.PacienteNombre, 25),
				truncate(curp, 22),
				v.Fecha,
				truncate(v.Diagnostico, 30),
				trat,
			)
		}

		fmt.Println()
		fmt.Println("Columnas: ID | Paciente | CURP | Fecha | Diagnóstico | Nº tratamientos")
		fmt.Println("Usa: medscan query <CURP>  para ver el expediente completo.")
		return nil
	},
}

// ── db rechazados ──────────────────────────────────────────────────────────

var dbRechazadosLimit int

var dbRechazadosCmd = &cobra.Command{
	Use:   "rechazados",
	Short: "Muestra los archivos rechazados durante el scan",
	Long: `Lista los archivos que fueron rechazados durante el scan por:
  · blur   — imagen demasiado borrosa (score < umbral)
  · formato — extensión no soportada
  · tamaño  — archivo mayor a 20MB`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rechazados, err := db.ListRejected(dbRechazadosLimit)
		if err != nil {
			return fmt.Errorf("error leyendo archivos rechazados: %w", err)
		}

		if len(rechazados) == 0 {
			fmt.Println("No hay archivos rechazados.")
			return nil
		}

		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Archivos rechazados (%d)\n", len(rechazados))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("%-4s  %-35s  %-10s  %-8s  %s\n", "ID", "Archivo", "Motivo", "Blur", "Fecha")
		fmt.Printf("%-4s  %-35s  %-10s  %-8s  %s\n",
			strings.Repeat("─", 4),
			strings.Repeat("─", 35),
			strings.Repeat("─", 10),
			strings.Repeat("─", 8),
			strings.Repeat("─", 19))

		for _, r := range rechazados {
			blurStr := ""
			if r.BlurScore > 0 {
				blurStr = fmt.Sprintf("%.1f", r.BlurScore)
			} else {
				blurStr = "—"
			}
			fmt.Printf("%-4d  %-35s  %-10s  %-8s  %s\n",
				r.ID,
				truncate(filepath.Base(r.FilePath), 35),
				r.Motivo,
				blurStr,
				r.ScannedAt,
			)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	dbVisitasCmd.Flags().IntVar(&dbVisitasLimit, "limit", 50, "Número máximo de visitas a mostrar")
	dbRechazadosCmd.Flags().IntVar(&dbRechazadosLimit, "limit", 50, "Número máximo de registros a mostrar")

	dbCmd.AddCommand(dbStatsCmd)
	dbCmd.AddCommand(dbVisitasCmd)
	dbCmd.AddCommand(dbRechazadosCmd)
}

// formatBytes convierte bytes a una representación legible (KB, MB).
func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/1024/1024)
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
