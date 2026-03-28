package store

import (
	"database/sql"
	"fmt"
)

// DBStats contiene estadísticas globales de la base de datos.
type DBStats struct {
	TotalPacientes      int
	TotalVisitas        int
	TotalTratamientos   int
	TotalRechazados     int
	TotalFallidos       int
	TamanioBytes        int64
}

// GetStats devuelve las estadísticas globales de la base de datos.
func (db *DB) GetStats() (*DBStats, error) {
	stats := &DBStats{}
	queries := []struct {
		dest  *int
		query string
	}{
		{&stats.TotalPacientes, `SELECT COUNT(*) FROM patients`},
		{&stats.TotalVisitas, `SELECT COUNT(*) FROM visits`},
		{&stats.TotalTratamientos, `SELECT COUNT(*) FROM treatments`},
		{&stats.TotalRechazados, `SELECT COUNT(*) FROM rejected_files`},
		{&stats.TotalFallidos, `SELECT COUNT(*) FROM failed_files`},
	}
	for _, q := range queries {
		if err := db.conn.QueryRow(q.query).Scan(q.dest); err != nil {
			return nil, fmt.Errorf("consulta stats: %w", err)
		}
	}
	return stats, nil
}

// VisitRow representa una fila en la vista de visitas recientes.
type VisitRow struct {
	ID             int64
	PacienteNombre string
	PacienteCURP   string
	DoctorNombre   string
	Fecha          string
	Diagnostico    string
	BlurScore      float64
	ProcesadoEnMs  int64
	ArchivoOrigen  string
	NumTratamientos int
}

// ListVisits devuelve las visitas ordenadas por fecha desc, con límite.
func (db *DB) ListVisits(limit int) ([]VisitRow, error) {
	rows, err := db.conn.Query(`
		SELECT
			v.id,
			p.nombre,
			COALESCE(p.curp, ''),
			COALESCE(d.nombre, ''),
			COALESCE(v.fecha, ''),
			COALESCE(v.diagnostico, ''),
			v.blur_score,
			v.procesado_en_ms,
			COALESCE(v.archivo_origen, ''),
			(SELECT COUNT(*) FROM treatments t WHERE t.visit_id = v.id)
		FROM visits v
		LEFT JOIN patients p ON p.id = v.patient_id
		LEFT JOIN doctors d ON d.id = v.doctor_id
		ORDER BY v.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VisitRow
	for rows.Next() {
		var r VisitRow
		err := rows.Scan(&r.ID, &r.PacienteNombre, &r.PacienteCURP, &r.DoctorNombre,
			&r.Fecha, &r.Diagnostico, &r.BlurScore, &r.ProcesadoEnMs,
			&r.ArchivoOrigen, &r.NumTratamientos)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// RejectedRow representa una fila en la vista de archivos rechazados.
type RejectedRow struct {
	ID        int64
	FilePath  string
	Motivo    string
	BlurScore float64
	ScannedAt string
}

// ListRejected devuelve los archivos rechazados más recientes.
func (db *DB) ListRejected(limit int) ([]RejectedRow, error) {
	rows, err := db.conn.Query(`
		SELECT id, file_path, motivo, blur_score, scanned_at
		FROM rejected_files
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RejectedRow
	for rows.Next() {
		var r RejectedRow
		var bs sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.FilePath, &r.Motivo, &bs, &r.ScannedAt); err != nil {
			return nil, err
		}
		r.BlurScore = bs.Float64
		result = append(result, r)
	}
	return result, nil
}
