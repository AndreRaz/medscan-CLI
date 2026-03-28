package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
	"medscan/internal/pipeline"
)

func (a *App) buildScanView() tview.Primitive {
	form := tview.NewForm()
	
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			a.tviewApp.Draw()
		})
	logView.SetTitle(" Registro de Actividad (Log) ")
	logView.SetBorder(true)

	// Layout con el log debajo del formulario
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 9, 1, true).
		AddItem(logView, 0, 3, false)

	form.AddInputField("Ruta (Carpeta)", "./docs/", 40, nil, nil).
		AddInputField("Umbral de nitidez", "100.0", 10, nil, nil).
		AddButton("Iniciar Escaneo", func() {
			folderItem := form.GetFormItemByLabel("Ruta (Carpeta)")
			var folder string
			if input, ok := folderItem.(*tview.InputField); ok {
				folder = strings.TrimSpace(input.GetText())
			}

			blurItem := form.GetFormItemByLabel("Umbral de nitidez")
			var blurStr string
			if input, ok := blurItem.(*tview.InputField); ok {
				blurStr = strings.TrimSpace(input.GetText())
			}
			blurThreshold, _ := strconv.ParseFloat(blurStr, 64)

			logView.SetText(fmt.Sprintf("[yellow::b]Iniciando digitalización en: %s[-]\n", folder))

			events := make(chan pipeline.ScanEvent, 10)
			
			cfg := pipeline.Config{
				Folder:        folder,
				BlurThreshold: blurThreshold,
				Db:            a.db,
				Transcriber:   a.llm,
			}
			
			// Hilo trabajador (scanner)
			go func() {
				res, err := pipeline.RunScanner(cfg, events)
				
				a.tviewApp.QueueUpdateDraw(func() {
					if err != nil {
						logView.Write([]byte(fmt.Sprintf("\n[red::b]Fallo crítico: %v[-]\n", err)))
						return
					}
					logView.Write([]byte(fmt.Sprintf("\n[green::b]--- Tarea Completada ---[-]\n")))
					logView.Write([]byte(fmt.Sprintf("Auditoría -> Procesados: %d | Duplicados: %d | Rechazados: %d | Errores: %d\n", 
						res.Processed, res.Duplicates, res.BlurRej+res.FormatRej, res.APIError)))
				})
			}()

			// Hilo receptor de eventos (para actualizar la TUI sin bloquear)
			go func() {
				for evt := range events {
					var text string
					
					// Formato si es evento de error inicial o evento general
					if evt.CurrentFile == 0 {
						text = fmt.Sprintf("[-] %s -> ", evt.FileName)
					} else {
						text = fmt.Sprintf("[%d/%d] %s -> ", evt.CurrentFile, evt.TotalFiles, evt.FileName)
					}
					
					if strings.Contains(evt.Status, "Rechazado") {
						text += fmt.Sprintf("[yellow]%s[-]", evt.Status)
					} else if strings.Contains(evt.Status, "Duplicado") {
						text += fmt.Sprintf("[blue]%s[-]", evt.Status)
					} else if strings.Contains(evt.Status, "Procesado") {
						text += fmt.Sprintf("[green]%s[-]", evt.Status)
					} else {
						text += fmt.Sprintf("[red]%s[-] %s", evt.Status, evt.ErrorMessage)
					}
					
					if evt.BlurScore > 0 {
						text += fmt.Sprintf(" (blur=%.1f)", evt.BlurScore)
					}
					text += "\n"

					a.tviewApp.QueueUpdateDraw(func() {
						logView.Write([]byte(text))
						logView.ScrollToEnd()
					})
				}
			}()
		}).
		AddButton("Detener temporalmente", func() {
			logView.Write([]byte("[white::d]Funcionalidad de pausa aún no implementada.[-]\n"))
		})
		
	form.SetTitle(" Configurar Escaneo Nuevo ")
	form.SetBorder(true)

	a.scanView = flex
	return flex
}
