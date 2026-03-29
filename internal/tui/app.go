package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"medscan/internal/store"
	"medscan/internal/transcriber"
)

// App encapsula la lógica principal de la interfaz interactiva.
type App struct {
	tviewApp *tview.Application
	pages    *tview.Pages
	layout   *tview.Flex
	header   *tview.TextView
	db       *store.DB
	llm      transcriber.Transcriber

	// Vistas de cada sección
	dashboardView tview.Primitive
	patientsView  tview.Primitive
	patientsTable *tview.Table
	scanView      tview.Primitive
	exportView    tview.Primitive

	// Sub-paneles del dashboard (para refresh)
	dashLogo     *tview.TextView
	dashStats    *tview.TextView
	dashActivity *tview.TextView
	dashMenu     *tview.List

	// Página activa
	activePage string
}

// NewApp crea una nueva instancia de la TUI.
func NewApp(db *store.DB, llm transcriber.Transcriber) *App {
	a := &App{
		tviewApp:   tview.NewApplication(),
		pages:      tview.NewPages(),
		db:         db,
		llm:        llm,
		activePage: "dashboard",
	}

	// Paleta de colores unificada
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.NewRGBColor(20, 12, 30)
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.SecondaryTextColor = tcell.ColorLightGray
	tview.Styles.TertiaryTextColor = tcell.ColorMediumOrchid

	a.setupLayout()
	return a
}

// setupLayout construye el layout principal.
func (a *App) setupLayout() {
	// ── Header ────────────────────────────────────────────────────────────
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.header.SetBackgroundColor(tcell.NewRGBColor(20, 12, 30))
	a.refreshHeader()

	// ── Vistas de sección ─────────────────────────────────────────────────
	a.dashboardView = a.buildDashboard()
	a.patientsView = a.buildPatientsView()
	a.scanView = a.buildScanView()
	a.exportView = a.buildExportView()

	a.pages.AddPage("dashboard", a.dashboardView, true, true)
	a.pages.AddPage("patients", a.patientsView, true, false)
	a.pages.AddPage("scan", a.scanView, true, false)
	a.pages.AddPage("export", a.exportView, true, false)

	// ── Barra de ayuda inferior ───────────────────────────────────────────
	helpBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(" [darkgray]ESC[-] [white::d]Volver[-]   [darkgray]s[-] [white::d]Digitalizar[-]   [darkgray]p[-] [white::d]Pacientes[-]   [darkgray]e[-] [white::d]Exportar[-]   [darkgray]q[-] [white::d]Salir[-]")
	helpBar.SetBackgroundColor(tcell.NewRGBColor(20, 12, 30))

	// ── Layout vertical: header | contenido | ayuda ──────────────────────
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(helpBar, 1, 0, false)

	// ── Captura de teclas globales ─────────────────────────────────────────
	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// La única tecla global será Esc para volver al menú principal.
		// Hemos quitado las letras para que no interfieran con la escritura de rutas.
		if event.Key() == tcell.KeyEsc {
			if a.activePage != "dashboard" {
				a.switchTo("dashboard")
				return nil
			}
		}
		return event
	})

	a.tviewApp.SetRoot(a.layout, true).EnableMouse(true)
}

// switchTo cambia la sección activa.
func (a *App) switchTo(page string) {
	switch page {
	case "dashboard":
		a.refreshDashboard()
	case "patients":
		a.refreshPatients()
	}

	a.activePage = page
	a.pages.SwitchToPage(page)
	a.refreshHeader()
}

// refreshHeader actualiza el encabezado superior.
func (a *App) refreshHeader() {
	dbPath := os.Getenv("MEDISCAN_DB_PATH")
	if dbPath == "" {
		dbPath = "./mediscan.db"
	}
	dbName := filepath.Base(dbPath)

	sectionLabel := sectionName(a.activePage)

	a.header.SetText(fmt.Sprintf(
		" [mediumorchid::b]MEDSCAN[-] [white::d]by C-MED-Neuxora[-]  "+
			"[darkgray]│[-]  [teal]%s[-]  "+
			"[darkgray]│[-]  [white::d]DB: %s[-]",
		sectionLabel, dbName,
	))
}

// sectionName devuelve el nombre legible de la sección activa.
func sectionName(page string) string {
	switch page {
	case "dashboard":
		return "Dashboard"
	case "patients":
		return "Expedientes"
	case "scan":
		return "Digitalizar"
	case "export":
		return "Exportar DB"
	default:
		return page
	}
}

// Run arranca la aplicación interactiva.
func (a *App) Run() error {
	return a.tviewApp.Run()
}
