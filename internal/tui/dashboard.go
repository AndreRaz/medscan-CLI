package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// logoLines contiene el arte ASCII del logo MEDSCAN.
var logoLines = []string{
	"",
	"  ███╗   ███╗███████╗██████╗ ",
	"  ████╗ ████║██╔════╝██╔══██╗",
	"  ██╔████╔██║█████╗  ██║  ██║",
	"  ██║╚██╔╝██║██╔══╝  ██║  ██║",
	"  ██║ ╚═╝ ██║███████╗██████╔╝",
	"  ╚═╝     ╚═╝╚══════╝╚═════╝ ",
	"",
	"   ███████╗ ██████╗ █████╗ ███╗  ██╗",
	"   ██╔════╝██╔════╝██╔══██╗████╗ ██║",
	"   ███████╗██║     ███████║██╔██╗██║",
	"   ╚════██║██║     ██╔══██║██║╚████║",
	"   ███████║╚██████╗██║  ██║██║ ╚███║",
	"   ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚══╝",
	"",
	"  [teal::b]by C-MED-Neuxora[-]",
}

// buildDashboard construye la vista principal con estadísticas, actividad y menú.
func (a *App) buildDashboard() tview.Primitive {
	// ── Panel izquierdo: Logo ─────────────────────────────────────────────
	logoView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	logoView.SetBorder(true).
		SetBorderColor(tcell.ColorMediumOrchid).
		SetTitle(" MEDSCAN ").
		SetTitleColor(tcell.ColorMediumOrchid)
	logoView.SetText("[mediumorchid::b]" + strings.Join(logoLines, "\n") + "[-]")

	// ── Panel central: Estadísticas ────────────────────────────────────────
	statsView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	statsView.SetBorder(true).
		SetBorderColor(tcell.ColorMediumOrchid).
		SetTitle(" Resumen ").
		SetTitleColor(tcell.ColorMediumOrchid)

	// ── Panel derecho: Actividad reciente ─────────────────────────────────
	activityView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetWordWrap(true)
	activityView.SetBorder(true).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitle(" Actividad Reciente ").
		SetTitleColor(tcell.ColorDarkCyan)

	// ── Panel inferior: Menú de navegación ────────────────────────────────
	menu := tview.NewList().
		AddItem("Digitalizar Documentos", "Iniciar un nuevo escaneo de imágenes", 's', func() { a.switchTo("scan") }).
		AddItem("Pacientes y Expedientes", "Ver historial y expedientes médicos", 'p', func() { a.switchTo("patients") }).
		AddItem("Exportar Base de Datos", "Copiar o mover tu base de datos SQLite", 'e', func() { a.switchTo("export") }).
		AddItem("Salir", "Cerrar medscan", 'q', func() { a.tviewApp.Stop() })

	menu.SetTitle(" Menú Principal (Click o Enter) ").
		SetBorder(true).
		SetBorderColor(tcell.ColorMediumOrchid)

	menu.SetMainTextColor(tcell.ColorAqua)
	menu.SetSecondaryTextColor(tcell.ColorLightGray)
	menu.SetShortcutColor(tcell.ColorMediumOrchid)
	menu.SetSelectedBackgroundColor(tcell.ColorMediumOrchid)

	// ── Layout: logo | stats | actividad ─────────────────────────────────
	topRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(logoView, 40, 0, false).
		AddItem(statsView, 0, 1, false).
		AddItem(activityView, 0, 2, false)

	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topRow, 0, 1, false).
		AddItem(menu, 10, 1, true)

	a.dashLogo = logoView
	a.dashStats = statsView
	a.dashActivity = activityView
	a.dashMenu = menu
	a.dashboardView = mainLayout

	a.refreshDashboard()
	return mainLayout
}

// refreshDashboard actualiza los paneles del dashboard con datos frescos de la DB.
func (a *App) refreshDashboard() {
	if a.dashStats == nil {
		return
	}

	// ── Estadísticas ────────────────────────────────────────────────────────
	stats, err := a.db.GetStats()
	if err != nil {
		a.dashStats.SetText(fmt.Sprintf("\n  [red]Error cargando estadísticas:\n  %v[-]", err))
	} else {
		rechazadosColor := "green"
		if stats.TotalRechazados > 0 {
			rechazadosColor = "yellow"
		}
		errorColor := "green"
		if stats.TotalFallidos > 0 {
			errorColor = "red"
		}

		a.dashStats.SetText(fmt.Sprintf(
			"\n"+
				"  [white::d]Pacientes registrados[-]\n"+
				"  [aqua::b]%d[-]\n\n"+
				"  [white::d]Visitas médicas[-]\n"+
				"  [aqua::b]%d[-]\n\n"+
				"  [white::d]Tratamientos prescritos[-]\n"+
				"  [aqua::b]%d[-]\n\n"+
				"  [white::d]Archivos rechazados[-]\n"+
				"  [%s::b]%d[-]\n\n"+
				"  [white::d]Errores de transcripción[-]\n"+
				"  [%s::b]%d[-]\n",
			stats.TotalPacientes,
			stats.TotalVisitas,
			stats.TotalTratamientos,
			rechazadosColor, stats.TotalRechazados,
			errorColor, stats.TotalFallidos,
		))
	}

	// ── Actividad reciente ──────────────────────────────────────────────────
	visits, err := a.db.ListVisits(8)
	if err != nil {
		a.dashActivity.SetText(fmt.Sprintf("\n  [red]Error cargando actividad: %v[-]", err))
		return
	}

	if len(visits) == 0 {
		a.dashActivity.SetText(
			"\n  [white::d]No hay actividad registrada aún.\n\n" +
				"  Usa la opción [white::b]Digitalizar Documentos[-][white::d]\n" +
				"  del menú inferior para empezar.[-]",
		)
		return
	}

	var sb strings.Builder
	sb.WriteString("\n")

	for i, v := range visits {
		nombre := v.PacienteNombre
		if len(nombre) > 24 {
			nombre = nombre[:22] + ".."
		}
		dx := v.Diagnostico
		if len(dx) > 36 {
			dx = dx[:34] + ".."
		}
		fecha := v.Fecha
		if len(fecha) > 10 {
			fecha = fecha[:10]
		}

		rowColor := "white"
		if i%2 == 0 {
			rowColor = "lightgray"
		}

		sb.WriteString(fmt.Sprintf(
			"  [%s::b]%s[-]  [teal]%s[-]\n  [white::d]%s[-]\n\n",
			rowColor, nombre, fecha, dx,
		))
	}
	a.dashActivity.SetText(sb.String())
}
