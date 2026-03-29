package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"
)

// buildExportView construye la pantalla de exportación de la base de datos.
func (a *App) buildExportView() tview.Primitive {
	// Ruta predeterminada a Descargas del usuario
	homeDir, _ := os.UserHomeDir()
	defaultDest := filepath.Join(homeDir, "Downloads", "mediscan_backup.db")

	// Panel de estado / log
	statusView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	statusView.SetBorder(true).
		SetTitle(" Estado de la exportación ").
		SetTitleColor(tview.Styles.SecondaryTextColor)

	statusView.SetText(
		"\n  [white::d]Elige una ruta de destino y presiona un botón.[-]\n\n" +
			"  [teal::b]Copiar[-][white::d] — crea una copia, la DB activa no cambia.[-]\n\n" +
			"  [mediumorchid::b]Mover[-][white::d] — mueve la DB a la ruta indicada y actualiza\n" +
			"  la configuración para que medscan use la nueva ubicación.[-]",
	)

	// Formulario principal
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Exportar / Mover Base de Datos ").
		SetTitleColor(tview.Styles.PrimaryTextColor)

	form.AddInputField("Ruta de destino", defaultDest, 60, nil, nil)

	// Botón Copiar
	form.AddButton("Copiar DB", func() {
		dest := getFormValue(form, "Ruta de destino")
		if dest == "" {
			statusView.SetText("[red::b]Error: La ruta de destino no puede estar vacía.[-]")
			return
		}
		if err := exportDB(false, dest, statusView); err != nil {
			statusView.SetText(fmt.Sprintf("[red::b]Error al copiar:\n  %v[-]", err))
		}
	})

	// Botón Mover
	form.AddButton("Mover DB", func() {
		dest := getFormValue(form, "Ruta de destino")
		if dest == "" {
			statusView.SetText("[red::b]Error: La ruta de destino no puede estar vacía.[-]")
			return
		}
		// Confirmación en la propia vista (no modal para mayor simplicidad)
		statusView.SetText(fmt.Sprintf(
			"[yellow::b]¿Confirmar? La DB se moverá a:\n  %s\n\nPresiona Mover DB de nuevo para confirmar, o cambia la ruta.[-]",
			dest,
		))
		// Usamos el text del status como "primer clic = aviso, segundo = ejecutar"
		// Reemplazamos el botón con uno que confirma directamente
		replaceMoveBtnWithConfirm(form, func() {
			if err := exportDB(true, dest, statusView); err != nil {
				statusView.SetText(fmt.Sprintf("[red::b]Error al mover:\n  %v[-]", err))
			}
		})
		a.tviewApp.Draw()
	})

	form.AddButton("Volver", func() {
		a.switchTo("dashboard")
	})

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 12, 1, true).
		AddItem(statusView, 0, 2, false)

	a.exportView = layout
	return layout
}

// getFormValue extrae el texto de un InputField por su label.
func getFormValue(form *tview.Form, label string) string {
	item := form.GetFormItemByLabel(label)
	if input, ok := item.(*tview.InputField); ok {
		return strings.TrimSpace(input.GetText())
	}
	return ""
}

// replaceMoveBtnWithConfirm sustituye el botón "Mover DB" por "Confirmar Mover".
func replaceMoveBtnWithConfirm(form *tview.Form, action func()) {
	// Eliminar botones actuales y reconstruir con confirmación
	// tview no soporta RemoveButton directamente; lo simulamos redibujando.
	// Estrategia: simplemente reemplazamos el handler con el action directo.
	// La próxima vez que el usuario presione el botón (que ahora dice "Confirmar"),
	// se ejecuta el movimiento.
	for i := 0; i < form.GetButtonCount(); i++ {
		btn := form.GetButton(i)
		if btn != nil && btn.GetLabel() == "Mover DB" {
			btn.SetLabel("Confirmar Mover")
			btn.SetSelectedFunc(action)
			return
		}
	}
}

// exportDB copia o mueve la DB al destino indicado.
// Si move=true, también actualiza MEDISCAN_DB_PATH en el .env activo.
func exportDB(move bool, dest string, status *tview.TextView) error {
	src := os.Getenv("MEDISCAN_DB_PATH")
	if src == "" {
		src = "./mediscan.db"
	}

	// Resolver rutas absolutas
	src, _ = filepath.Abs(src)
	dest, _ = filepath.Abs(expandHome(dest))

	// ── Verificar origen ───────────────────────────────────────────────────
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("no se encontró la base de datos en: %s", src)
	}

	// ── Verificar si el destino ya existe ──────────────────────────────────
	if _, err := os.Stat(dest); err == nil {
		// Renombrar el existente como .bak
		bak := dest + ".bak"
		_ = os.Rename(dest, bak)
		status.SetText(fmt.Sprintf("[yellow]Archivo existente renombrado a:\n  %s[-]\n\nCopiando...", bak))
	}

	// ── Crear directorio de destino si no existe ───────────────────────────
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("no se pudo crear el directorio: %w", err)
	}

	// ── Copiar el archivo ──────────────────────────────────────────────────
	if err := copyFile(src, dest); err != nil {
		return fmt.Errorf("error copiando el archivo: %w", err)
	}

	if !move {
		status.SetText(fmt.Sprintf(
			"[green::b]Copia completada[-]\n\n"+
				"  Origen:  [white]%s[-]\n"+
				"  Destino: [white]%s[-]\n\n"+
				"  [white::d]La base de datos activa sigue siendo la original.[-]",
			src, dest,
		))
		return nil
	}

	// ── Si es Mover: eliminar el original y actualizar el .env ─────────────
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("copia OK, pero no se pudo eliminar el original (%s): %w", src, err)
	}

	if err := updateEnvDBPath(dest); err != nil {
		status.SetText(fmt.Sprintf(
			"[yellow::b]DB movida pero no se pudo actualizar .env[-]\n"+
				"  Actualiza manualmente: MEDISCAN_DB_PATH=%s\n\n"+
				"  Error: %v",
			dest, err,
		))
		return nil
	}

	// Actualizar la variable de entorno en el proceso actual
	_ = os.Setenv("MEDISCAN_DB_PATH", dest)

	status.SetText(fmt.Sprintf(
		"[green::b]Base de datos movida correctamente[-]\n\n"+
			"  Nueva ubicación: [white]%s[-]\n\n"+
			"  [white::d]El archivo .env fue actualizado.\n"+
			"  Reinicia medscan para que la nueva ruta surta efecto.[-]",
		dest,
	))
	return nil
}

// copyFile copia src → dst byte a byte.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// updateEnvDBPath actualiza (o añade) MEDISCAN_DB_PATH en el archivo .env.
func updateEnvDBPath(newPath string) error {
	envFile := ".env"

	content, err := os.ReadFile(envFile)
	if err != nil {
		// Si no existe el .env, lo creamos con solo esta línea
		return os.WriteFile(envFile, []byte("MEDISCAN_DB_PATH="+newPath+"\n"), 0600)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "MEDISCAN_DB_PATH=") {
			lines[i] = "MEDISCAN_DB_PATH=" + newPath
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "MEDISCAN_DB_PATH="+newPath)
	}

	return os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0600)
}

// expandHome expande el prefijo "~" a la ruta del home del usuario.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
