package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildPatientsView() tview.Primitive {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)
	table.SetTitle(" Pacientes Registrados ")
	table.SetBorder(true)

	// Panel derecho para ver todo el documento JSON o Formato Lectura
	details := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	details.SetTitle(" Expediente Completo ")
	details.SetBorder(true)

	// Cuando el usuario mueve la selección en la tabla, precarga detalles.
	table.SetSelectionChangedFunc(func(row, column int) {
		if row == 0 { 
			details.SetText("Selecciona un paciente de la lista...")
			return 
		}

		cell := table.GetCell(row, 0)
		if cell == nil || cell.GetReference() == nil {
			details.SetText("")
			return
		}

		patientID := cell.GetReference().(int64)
		
		// Obtener info completa
		exp, visitas, tratamientos, err := a.db.GetExpedienteByID(patientID)
		if err != nil {
			details.SetText(fmt.Sprintf("[red::b]No se pudo cargar: %v[-]", err))
			return
		}

		// Renderizar expediente en texto con markup
		content := fmt.Sprintf(`[white::b]%s[-]
CURP:      [aqua]%s[-]
NSS:       %s
Fecha Nac: %s
Domicilio: %s
Teléfono:  %s

[white::b]--- %d Visitas Registradas ---[-]
`, exp.Paciente.Nombre, exp.Paciente.CURP, exp.Paciente.NSS, exp.Paciente.FechaNacimiento, exp.Paciente.Domicilio, exp.Paciente.Telefono, len(visitas))

		for i, v := range visitas {
			content += fmt.Sprintf("\n[yellow::b]Visita #%d - Fecha: %s[-]\n", i+1, v.Fecha)
			content += fmt.Sprintf("[white::b]Diagnóstico:[-] %s\n", v.Diagnostico)
			content += fmt.Sprintf("[white::b]Síntomas:[-]    %s\n", v.Sintomas)
			content += fmt.Sprintf("[white::b]Notas:[-]       %s\n", v.Notas)
			
			if len(tratamientos) > i && len(tratamientos[i]) > 0 {
				content += "\n[white::b]Tratamiento Recetado:[-]\n"
				for _, t := range tratamientos[i] {
					content += fmt.Sprintf("  • [green]%s[-] - %s, %s\n", t.Medicamento, t.Dosis, t.Frecuencia)
					if t.Indicaciones != "" {
						content += fmt.Sprintf("    Indicaciones: %s\n", t.Indicaciones)
					}
				}
			}
			content += "\n"
		}

		details.SetText(content)
		details.ScrollToBeginning()
	})

	// Layout dividido (1 tercio para la lista, 2 tercios para el expediente)
	flex := tview.NewFlex().
		AddItem(table, 0, 1, true).
		AddItem(details, 0, 2, false)

	a.patientsView = flex
	a.patientsTable = table

	a.refreshPatients()
	return flex
}

// refreshPatients carga los pacientes desde la DB a la tabla.
func (a *App) refreshPatients() {
	if a.patientsTable == nil {
		return
	}
	
	table := a.patientsTable
	table.Clear()

	patients, err := a.db.ListPatients()
	if err != nil {
		table.SetCell(0, 0, tview.NewTableCell("Error cargando pacientes").SetTextColor(tcell.ColorRed))
		return
	}

	// Títulos de columnas
	table.SetCell(0, 0, tview.NewTableCell("Nombre").SetTextColor(tcell.ColorMediumOrchid).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("CURP").SetTextColor(tcell.ColorMediumOrchid).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Visitas").SetTextColor(tcell.ColorMediumOrchid).SetSelectable(false))

	if len(patients) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("  Sin pacientes registrados. Usá Digitalizar para empezar.").
			SetTextColor(tcell.ColorDimGray).
			SetSelectable(false))
		return
	}

	for i, p := range patients {
		curp := p.CURP
		if curp == "" {
			curp = "N/A"
		}

		cNom := tview.NewTableCell(p.Nombre).SetReference(p.ID)
		cCurp := tview.NewTableCell(curp)
		cVisits := tview.NewTableCell(fmt.Sprintf("%d", p.VisitCount)).SetTextColor(tcell.ColorDarkCyan)

		table.SetCell(i+1, 0, cNom)
		table.SetCell(i+1, 1, cCurp)
		table.SetCell(i+1, 2, cVisits)
	}

	// Reiniciar selección al primer paciente si existe
	if len(patients) > 0 {
		table.Select(1, 0)
	}
}
