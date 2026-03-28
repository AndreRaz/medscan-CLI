package cmd

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"medscan/internal/store"
	"medscan/internal/transcriber"
	"medscan/internal/tui"
)

var (
	// db es la conexión global a la base de datos, inicializada en root.
	db *store.DB
)

// rootCmd es el comando raíz de medscan.
var rootCmd = &cobra.Command{
	Use:   "medscan",
	Short: "Digitaliza expedientes médicos con visión por IA",
	Long: `medscan — Digitalizador de expedientes clínicos en papel.

Usa visión por IA para transcribir documentos médicos a JSON estructurado
y los persiste en una base de datos SQLite local y consultable.

Proveedores de LLM:
  MEDISCAN_PROVIDER=gemini    → Google Gemini 2.5 Flash (gratis, para desarrollo)
  MEDISCAN_PROVIDER=anthropic → Anthropic Claude (precisión alta, para producción)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		llm := transcriber.New()
		app := tui.NewApp(db, llm)
		return app.Run()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initDB()
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if db != nil {
			return db.Close()
		}
		return nil
	},
}

// Execute ejecuta el comando raíz.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Cargar .env si existe (silencioso si no existe)
	_ = godotenv.Load()

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(patientCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(dbCmd)
}

// initDB abre la conexión a la base de datos y ejecuta migraciones.
func initDB() error {
	dbPath := store.GetDBPath()
	var err error
	db, err = store.New(dbPath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir la base de datos en %s: %w", dbPath, err)
	}
	return nil
}
