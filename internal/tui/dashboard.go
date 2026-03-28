package tui

import (
	"fmt"
	"github.com/rivo/tview"
)

// buildDashboard construye la vista inicial de métricas.
func (a *App) buildDashboard() tview.Primitive {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	text.SetBorder(true).SetTitle(" Dashboard Estadístico ")
	a.dashboardView = text
	a.refreshDashboard()
	return text
}

// refreshDashboard consulta la DB para actualizar los contadores.
func (a *App) refreshDashboard() {
	if a.dashboardView == nil {
		return
	}
	tv, ok := a.dashboardView.(*tview.TextView)
	if !ok {
		return
	}

	stats, err := a.db.GetStats()
	if err != nil {
		tv.SetText(fmt.Sprintf("\n\n[red]Error al cargar estadísticas: %v[-]", err))
		return
	}

	content := fmt.Sprintf(`

[white::b]medscan — Resumen Local[-]



Pacientes Registrados:    [green::b]%d[-]
Visitas Médicas Generadas: [green::b]%d[-]
Tratamientos Preescritos: [green::b]%d[-]

Archivos Rechazados:      [yellow::b]%d[-] (borrosos o no válidos)
Auditoría Fallidos:       [red::b]%d[-]  (errores lógicos)



Navegación: Usa [white::b]TAB[-] para saltar entre los paneles.
Flechas / J-K para desplazarte por listas.`,
		stats.TotalPacientes, stats.TotalVisitas, stats.TotalTratamientos, stats.TotalRechazados, stats.TotalFallidos)

	tv.SetText(content)
}
