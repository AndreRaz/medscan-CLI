package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
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
	logView.SetTitle(" Registro de Actividad (Log) ").SetBorder(true)

	// ── Campo de ruta: ancho 0 = se expande hasta llenar el espacio ────────
	rutaField := tview.NewInputField().
		SetLabel("Ruta (Carpeta)  ").
		SetText("./docs/").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.NewRGBColor(30, 20, 50))

	blurField := tview.NewInputField().
		SetLabel("Umbral nitidez  ").
		SetText("100.0").
		SetFieldWidth(12).
		SetFieldBackgroundColor(tcell.NewRGBColor(30, 20, 50))

	form.AddFormItem(rutaField).
		AddFormItem(blurField).
		AddButton("Iniciar Escaneo", func() {
			folder := strings.TrimSpace(rutaField.GetText())
			folder = resolveHome(folder)

			// Verificar que la carpeta existe antes de iniciar
			if _, err := os.Stat(folder); os.IsNotExist(err) {
				logView.SetText(fmt.Sprintf(
					"[red::b]Error: La carpeta no existe:[-]\n  %s\n\n"+
						"[white::d]Verifica la ruta e intenta de nuevo.[-]", folder))
				return
			}

			blurStr := strings.TrimSpace(blurField.GetText())
			blurThreshold, _ := strconv.ParseFloat(blurStr, 64)

			absFolder, _ := filepath.Abs(folder)
			logView.SetText(fmt.Sprintf("[yellow::b]Iniciando digitalización en:[-]\n  %s\n\n", absFolder))

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
					logView.Write([]byte("\n[green::b]--- Tarea Completada ---[-]\n"))
					logView.Write([]byte(fmt.Sprintf(
						"Auditoría -> Procesados: %d | Duplicados: %d | Rechazados: %d | Errores: %d\n",
						res.Processed, res.Duplicates, res.BlurRej+res.FormatRej, res.APIError)))
				})
			}()

			// Hilo receptor de eventos (actualiza la TUI sin bloquear)
			go func() {
				for evt := range events {
					var text string

					if evt.CurrentFile == 0 {
						text = fmt.Sprintf("[-] %s -> ", evt.FileName)
					} else {
						text = fmt.Sprintf("[%d/%d] %s -> ", evt.CurrentFile, evt.TotalFiles, evt.FileName)
					}

					switch {
					case strings.Contains(evt.Status, "Rechazado"):
						text += fmt.Sprintf("[yellow]%s[-]", evt.Status)
					case strings.Contains(evt.Status, "Duplicado"):
						text += fmt.Sprintf("[blue]%s[-]", evt.Status)
					case strings.Contains(evt.Status, "Procesado"):
						text += fmt.Sprintf("[green]%s[-]", evt.Status)
					default:
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
		AddButton("Carpeta actual", func() {
			cwd, err := os.Getwd()
			if err == nil {
				rutaField.SetText(cwd)
			}
		}).
		AddButton("Limpiar log", func() {
			logView.SetText("")
		})

	form.SetTitle(" Configurar Escaneo ").SetBorder(true)

	// Línea de ayuda con ejemplos de rutas
	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetText("  [white::d]Rutas aceptadas:  [white]./docs/  [white::d]|  [white]/ruta/absoluta/  [white::d]|  [white]~/Documentos/  [white::d](~ se expande automáticamente)[-]")
	hint.SetBackgroundColor(tcell.NewRGBColor(12, 8, 20))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 11, 1, true).
		AddItem(hint, 1, 0, false).
		AddItem(logView, 0, 3, false)

	a.scanView = flex
	return flex
}

// resolveHome expande el prefijo "~/" al directorio home del usuario.
func resolveHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
