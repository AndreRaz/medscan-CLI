package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"medscan/internal/imageproc"
	"medscan/internal/scanner"
	"medscan/internal/store"
	"medscan/internal/transcriber"
)

// ScanEvent representa un evento de progreso durante el escaneo.
type ScanEvent struct {
	TotalFiles   int
	CurrentFile  int
	FileName     string
	Status       string // ej: "Duplicado", "Rechazado (Borrosa)", "Procesado", "Error"
	ErrorMessage string // Si hubo error
	BlurScore    float64
}

// Config parámetros ajustables del pipeline
type Config struct {
	Folder        string
	BlurThreshold float64
	DebugBlur     bool
	Db            *store.DB
	Transcriber   transcriber.Transcriber
}

// Result consolida las métricas finales
type Result struct {
	TotalFiles      int
	Processed       int
	Duplicates      int
	BlurRej         int
	FormatRej       int
	APIError        int
	FormatRejections []scanner.ValidationError
}

// RunScanner ejecuta el pipeline de escaneo notificando el progreso a un canal.
func RunScanner(cfg Config, events chan<- ScanEvent) (*Result, error) {
	defer close(events)

	// Obtener archivos válidos
	files, formatRejected, err := scanner.ScanFolder(cfg.Folder)
	if err != nil {
		return nil, fmt.Errorf("error al escanear carpeta: %w", err)
	}

	result := &Result{
		TotalFiles:      len(files) + len(formatRejected),
		FormatRej:       len(formatRejected),
		FormatRejections: formatRejected,
	}

	// Registrar formatos de archivo inválidos en base de datos
	for _, rej := range formatRejected {
		events <- ScanEvent{
			TotalFiles:   len(files),
			CurrentFile:  0,
			FileName:     filepath.Base(rej.Path),
			Status:       "Rechazado (Formato/Tamaño)",
			ErrorMessage: rej.Reason,
		}
		_ = cfg.Db.SaveRejectedFile(rej.Path, "", "formato", 0)
	}

	for i, file := range files {
		startTime := time.Now()
		baseName := filepath.Base(file.Path)
		evt := ScanEvent{
			TotalFiles:  len(files),
			CurrentFile: i + 1,
			FileName:    baseName,
		}

		// 1. Deduplicación
		exists, _ := cfg.Db.HashExists(file.Hash)
		if exists {
			evt.Status = "Duplicado (Omitido)"
			events <- evt
			result.Duplicates++
			continue
		}

		// 2. Detección de blur
		blurScore, _ := imageproc.BlurScore(file.Path)
		evt.BlurScore = blurScore
		
		if cfg.BlurThreshold > 0 && blurScore < cfg.BlurThreshold {
			evt.Status = "Rechazado (Borrosa)"
			events <- evt
			_ = cfg.Db.SaveRejectedFile(file.Path, file.Hash, "blur", blurScore)
			result.BlurRej++
			continue
		}

		// 3. Preprocesamiento local
		processedPath, err := imageproc.Preprocess(file.Path)
		if err != nil {
			evt.Status = "Error (Pre-procesamiento)"
			evt.ErrorMessage = err.Error()
			events <- evt
			_ = cfg.Db.SaveFailedFile(file.Path, file.Hash, "preprocesamiento: "+err.Error())
			result.APIError++
			continue
		}

		// 4. Transcripción LLC
		exp, err := cfg.Transcriber.Transcribe(processedPath)
		os.Remove(processedPath) // Clean up temp file immediately

		if err != nil {
			evt.Status = "Error (API Transcripción)"
			evt.ErrorMessage = err.Error()
			events <- evt
			_ = cfg.Db.SaveFailedFile(file.Path, file.Hash, "transcripción: "+err.Error())
			result.APIError++
			continue
		}

		// 5. Consolidar metadata e Insertar en SQLite
		elapsed := time.Since(startTime).Milliseconds()
		exp.Visita.ArchivoOrigen = file.Path
		exp.Visita.BlurScore = blurScore
		exp.Visita.ProcesadoEnMs = elapsed

		if err := cfg.Db.SaveExpediente(exp, file.Hash); err != nil {
			evt.Status = "Error (Base de Datos)"
			evt.ErrorMessage = err.Error()
			events <- evt
			_ = cfg.Db.SaveFailedFile(file.Path, file.Hash, "db: "+err.Error())
			result.APIError++
			continue
		}

		evt.Status = "Procesado"
		if exp.Paciente.Nombre != "" {
			evt.Status += fmt.Sprintf(" [%s]", exp.Paciente.Nombre)
		}
		events <- evt
		result.Processed++
	}

	return result, nil
}
