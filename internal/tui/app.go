package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"medscan/internal/store"
	"medscan/internal/transcriber"
)

// App encapsula la lógica principal de la interfaz interactiva.
type App struct {
	tviewApp *tview.Application
	pages    *tview.Pages
	menu     *tview.List
	layout   *tview.Flex
	db       *store.DB
	llm      transcriber.Transcriber

	// Referencias a las vistas para actualizarlas
	dashboardView tview.Primitive
	patientsView  tview.Primitive
	patientsTable *tview.Table
	scanView      tview.Primitive
}

// NewApp crea una nueva instancia de la TUI.
func NewApp(db *store.DB, llm transcriber.Transcriber) *App {
	a := &App{
		tviewApp: tview.NewApplication(),
		pages:    tview.NewPages(),
		db:       db,
		llm:      llm,
	}
	a.setupLayout()
	return a
}

func (a *App) setupLayout() {
	a.menu = tview.NewList().
		AddItem("Dashboard", "Resumen y estadisticas", 'd', func() {
			a.switchTo("dashboard")
		}).
		AddItem("Pacientes", "Ver expedientes medicos", 'p', func() {
			a.switchTo("patients")
		}).
		AddItem("Escanear", "Digitalizar nuevos documentos", 's', func() {
			a.switchTo("scan")
		}).
		AddItem("Salir", "Cerrar aplicacion", 'q', func() {
			a.tviewApp.Stop()
		})
	a.menu.SetTitle(" medscan CLI ").SetBorder(true)

	a.dashboardView = a.buildDashboard()
	a.patientsView = a.buildPatientsView()
	a.scanView = a.buildScanView()

	a.pages.AddPage("dashboard", a.dashboardView, true, true)
	a.pages.AddPage("patients", a.patientsView, true, false)
	a.pages.AddPage("scan", a.scanView, true, false)

	a.layout = tview.NewFlex().
		AddItem(a.menu, 35, 1, true).
		AddItem(a.pages, 0, 3, false)

	// Capturar eventos globales: con TAB saltamos entre el menú y el panel principal.
	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			if a.menu.HasFocus() {
				// Foco al panel activo
				name, _ := a.pages.GetFrontPage()
				if name == "patients" {
					a.tviewApp.SetFocus(a.patientsView)
				} else if name == "scan" {
					a.tviewApp.SetFocus(a.scanView)
				}
			} else {
				a.tviewApp.SetFocus(a.menu)
			}
			return nil
		}
		return event
	})

	a.tviewApp.SetRoot(a.layout, true).EnableMouse(true)
}

func (a *App) switchTo(page string) {
	// Refrescar contenido si es necesario antes de cambiar de página
	if page == "dashboard" {
		a.refreshDashboard()
	} else if page == "patients" {
		a.refreshPatients()
	}

	a.pages.SwitchToPage(page)
	a.tviewApp.SetFocus(a.menu)
}

// Run arranca la aplicación interactiva.
func (a *App) Run() error {
	return a.tviewApp.Run()
}
