package store

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"medscan/internal/parser"
)

// DB es el wrapper sobre la conexión SQLite.
type DB struct {
	conn *sql.DB
}

// New abre (o crea) la base de datos en dbPath y ejecuta las migraciones.
func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("abrir DB: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migraciones: %w", err)
	}
	return db, nil
}

// Close cierra la conexión con la base de datos.
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate crea las tablas si no existen.
func (db *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS patients (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre           TEXT NOT NULL,
    curp             TEXT UNIQUE,
    nss              TEXT,
    fecha_nacimiento TEXT,
    telefono         TEXT,
    domicilio        TEXT
);

CREATE TABLE IF NOT EXISTS doctors (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre        TEXT NOT NULL,
    cedula        TEXT UNIQUE,
    especialidad  TEXT
);

CREATE TABLE IF NOT EXISTS visits (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id     INTEGER NOT NULL REFERENCES patients(id),
    doctor_id      INTEGER REFERENCES doctors(id),
    fecha          TEXT,
    diagnostico    TEXT,
    sintomas       TEXT,
    notas          TEXT,
    archivo_origen TEXT,
    file_hash      TEXT UNIQUE,
    blur_score     REAL,
    procesado_en_ms INTEGER
);

CREATE TABLE IF NOT EXISTS treatments (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    visit_id     INTEGER NOT NULL REFERENCES visits(id),
    medicamento  TEXT,
    dosis        TEXT,
    frecuencia   TEXT,
    duracion     TEXT,
    indicaciones TEXT
);

CREATE TABLE IF NOT EXISTS rejected_files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path  TEXT NOT NULL,
    file_hash  TEXT,
    motivo     TEXT NOT NULL,
    blur_score REAL,
    scanned_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS failed_files (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path  TEXT NOT NULL,
    file_hash  TEXT,
    motivo     TEXT NOT NULL,
    scanned_at TEXT DEFAULT (datetime('now'))
);
`
	_, err := db.conn.Exec(schema)
	return err
}

// HashExists comprueba si ya existe un archivo con ese hash (deduplicación).
func (db *DB) HashExists(hash string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM visits WHERE file_hash = ?
		UNION ALL
		SELECT COUNT(*) FROM rejected_files WHERE file_hash = ?
		UNION ALL
		SELECT COUNT(*) FROM failed_files WHERE file_hash = ?
	`, hash, hash, hash).Scan(&count)
	if err != nil {
		// QueryRow con UNION no hace Scan limpio; usar subconsulta
		var c1, c2, c3 int
		_ = db.conn.QueryRow(`SELECT COUNT(*) FROM visits WHERE file_hash = ?`, hash).Scan(&c1)
		_ = db.conn.QueryRow(`SELECT COUNT(*) FROM rejected_files WHERE file_hash = ?`, hash).Scan(&c2)
		_ = db.conn.QueryRow(`SELECT COUNT(*) FROM failed_files WHERE file_hash = ?`, hash).Scan(&c3)
		return (c1 + c2 + c3) > 0, nil
	}
	return count > 0, nil
}

// SaveExpediente persiste todo el expediente en la DB.
// Devuelve error si falla; usa upsert para pacientes y doctores.
func (db *DB) SaveExpediente(exp *parser.Expediente, fileHash string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("iniciar transacción: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Upsert paciente (llave: CURP si existe, sino nombre)
	var patientID int64
	if exp.Paciente.CURP != "" {
		err = tx.QueryRow(`
			INSERT INTO patients (nombre, curp, nss, fecha_nacimiento, telefono, domicilio)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(curp) DO UPDATE SET
				nombre = excluded.nombre,
				nss = excluded.nss,
				fecha_nacimiento = excluded.fecha_nacimiento,
				telefono = excluded.telefono,
				domicilio = excluded.domicilio
			RETURNING id
		`, exp.Paciente.Nombre, exp.Paciente.CURP, exp.Paciente.NSS,
			exp.Paciente.FechaNacimiento, exp.Paciente.Telefono, exp.Paciente.Domicilio,
		).Scan(&patientID)
	} else {
		var res sql.Result
		res, err = tx.Exec(`
			INSERT INTO patients (nombre, curp, nss, fecha_nacimiento, telefono, domicilio)
			VALUES (?, ?, ?, ?, ?, ?)
		`, exp.Paciente.Nombre, exp.Paciente.CURP, exp.Paciente.NSS,
			exp.Paciente.FechaNacimiento, exp.Paciente.Telefono, exp.Paciente.Domicilio,
		)
		if err == nil {
			patientID, err = res.LastInsertId()
		}
	}
	if err != nil {
		return fmt.Errorf("upsert paciente: %w", err)
	}

	// Upsert doctor (llave: cedula)
	var doctorID int64
	if exp.Doctor.Cedula != "" {
		err = tx.QueryRow(`
			INSERT INTO doctors (nombre, cedula, especialidad)
			VALUES (?, ?, ?)
			ON CONFLICT(cedula) DO UPDATE SET
				nombre = excluded.nombre,
				especialidad = excluded.especialidad
			RETURNING id
		`, exp.Doctor.Nombre, exp.Doctor.Cedula, exp.Doctor.Especialidad,
		).Scan(&doctorID)
	} else {
		var res sql.Result
		res, err = tx.Exec(`
			INSERT INTO doctors (nombre, cedula, especialidad) VALUES (?, ?, ?)
		`, exp.Doctor.Nombre, exp.Doctor.Cedula, exp.Doctor.Especialidad)
		if err == nil {
			doctorID, err = res.LastInsertId()
		}
	}
	if err != nil {
		return fmt.Errorf("upsert doctor: %w", err)
	}

	// Insertar visita
	var visitID int64
	err = tx.QueryRow(`
		INSERT INTO visits (patient_id, doctor_id, fecha, diagnostico, sintomas, notas, archivo_origen, file_hash, blur_score, procesado_en_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, patientID, doctorID, exp.Visita.Fecha, exp.Visita.Diagnostico,
		exp.Visita.Sintomas, exp.Visita.Notas, exp.Visita.ArchivoOrigen,
		fileHash, exp.Visita.BlurScore, exp.Visita.ProcesadoEnMs,
	).Scan(&visitID)
	if err != nil {
		return fmt.Errorf("insertar visita: %w", err)
	}

	// Insertar tratamientos
	for _, t := range exp.Tratamiento {
		_, err = tx.Exec(`
			INSERT INTO treatments (visit_id, medicamento, dosis, frecuencia, duracion, indicaciones)
			VALUES (?, ?, ?, ?, ?, ?)
		`, visitID, t.Medicamento, t.Dosis, t.Frecuencia, t.Duracion, t.Indicaciones)
		if err != nil {
			return fmt.Errorf("insertar tratamiento: %w", err)
		}
	}

	return tx.Commit()
}

// GetExpediente devuelve el expediente completo de un paciente por CURP.
func (db *DB) GetExpediente(curp string) (*parser.Expediente, []parser.Visita, [][]parser.Tratamiento, error) {
	// Obtener paciente
	var p parser.Paciente
	err := db.conn.QueryRow(`
		SELECT nombre, curp, nss, fecha_nacimiento, telefono, domicilio
		FROM patients WHERE curp = ?
	`, curp).Scan(&p.Nombre, &p.CURP, &p.NSS, &p.FechaNacimiento, &p.Telefono, &p.Domicilio)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("paciente no encontrado (CURP=%s): %w", curp, err)
	}

	// Obtener todas las visitas con su doctor
	rows, err := db.conn.Query(`
		SELECT v.id, v.fecha, v.diagnostico, v.sintomas, v.notas, v.archivo_origen, v.blur_score, v.procesado_en_ms,
		       d.nombre, d.cedula, d.especialidad
		FROM visits v
		LEFT JOIN doctors d ON d.id = v.doctor_id
		LEFT JOIN patients p ON p.id = v.patient_id
		WHERE p.curp = ?
		ORDER BY v.fecha DESC
	`, curp)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("obtener visitas: %w", err)
	}
	defer rows.Close()

	var visitas []parser.Visita
	var doctores []parser.Doctor
	var visitIDs []int64

	for rows.Next() {
		var visita parser.Visita
		var doctor parser.Doctor
		var visitID int64
		err := rows.Scan(
			&visitID, &visita.Fecha, &visita.Diagnostico, &visita.Sintomas,
			&visita.Notas, &visita.ArchivoOrigen, &visita.BlurScore, &visita.ProcesadoEnMs,
			&doctor.Nombre, &doctor.Cedula, &doctor.Especialidad,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("escanear visita: %w", err)
		}
		visitas = append(visitas, visita)
		doctores = append(doctores, doctor)
		visitIDs = append(visitIDs, visitID)
	}

	// Obtener tratamientos por visita
	var allTratamientos [][]parser.Tratamiento
	for _, vid := range visitIDs {
		trows, err := db.conn.Query(`
			SELECT medicamento, dosis, frecuencia, duracion, indicaciones
			FROM treatments WHERE visit_id = ?
		`, vid)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("obtener tratamientos: %w", err)
		}
		var tratamientos []parser.Tratamiento
		for trows.Next() {
			var t parser.Tratamiento
			if err := trows.Scan(&t.Medicamento, &t.Dosis, &t.Frecuencia, &t.Duracion, &t.Indicaciones); err != nil {
				trows.Close()
				return nil, nil, nil, err
			}
			tratamientos = append(tratamientos, t)
		}
		trows.Close()
		allTratamientos = append(allTratamientos, tratamientos)
	}

	_ = doctores // usados en contexto extendido si se necesita

	// Devolver el expediente base (primera visita reference) más todas las visitas
	var exp *parser.Expediente
	if len(visitas) > 0 {
		exp = &parser.Expediente{
			Paciente:    p,
			Doctor:      doctores[0],
			Visita:      visitas[0],
			Tratamiento: allTratamientos[0],
		}
	} else {
		exp = &parser.Expediente{Paciente: p}
	}

	return exp, visitas, allTratamientos, nil
}

// GetExpedienteByID devuelve el expediente de un paciente por su ID numérico.
// Útil para pacientes que no tienen CURP registrado.
func (db *DB) GetExpedienteByID(patientID int64) (*parser.Expediente, []parser.Visita, [][]parser.Tratamiento, error) {
	var p parser.Paciente
	err := db.conn.QueryRow(`
		SELECT nombre, COALESCE(curp,''), COALESCE(nss,''), COALESCE(fecha_nacimiento,''), COALESCE(telefono,''), COALESCE(domicilio,'')
		FROM patients WHERE id = ?
	`, patientID).Scan(&p.Nombre, &p.CURP, &p.NSS, &p.FechaNacimiento, &p.Telefono, &p.Domicilio)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("paciente no encontrado (ID=%d): %w", patientID, err)
	}

	rows, err := db.conn.Query(`
		SELECT v.id, COALESCE(v.fecha,''), COALESCE(v.diagnostico,''), COALESCE(v.sintomas,''),
		       COALESCE(v.notas,''), COALESCE(v.archivo_origen,''), v.blur_score, v.procesado_en_ms,
		       COALESCE(d.nombre,''), COALESCE(d.cedula,''), COALESCE(d.especialidad,'')
		FROM visits v
		LEFT JOIN doctors d ON d.id = v.doctor_id
		WHERE v.patient_id = ?
		ORDER BY v.fecha DESC
	`, patientID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("obtener visitas: %w", err)
	}
	defer rows.Close()

	var visitas []parser.Visita
	var doctores []parser.Doctor
	var visitIDs []int64

	for rows.Next() {
		var visita parser.Visita
		var doctor parser.Doctor
		var vid int64
		if err := rows.Scan(&vid, &visita.Fecha, &visita.Diagnostico, &visita.Sintomas,
			&visita.Notas, &visita.ArchivoOrigen, &visita.BlurScore, &visita.ProcesadoEnMs,
			&doctor.Nombre, &doctor.Cedula, &doctor.Especialidad); err != nil {
			return nil, nil, nil, err
		}
		visitas = append(visitas, visita)
		doctores = append(doctores, doctor)
		visitIDs = append(visitIDs, vid)
	}

	var allTratamientos [][]parser.Tratamiento
	for _, vid := range visitIDs {
		trows, err := db.conn.Query(`
			SELECT medicamento, dosis, frecuencia, duracion, indicaciones
			FROM treatments WHERE visit_id = ?
		`, vid)
		if err != nil {
			return nil, nil, nil, err
		}
		var tratamientos []parser.Tratamiento
		for trows.Next() {
			var t parser.Tratamiento
			if err := trows.Scan(&t.Medicamento, &t.Dosis, &t.Frecuencia, &t.Duracion, &t.Indicaciones); err != nil {
				trows.Close()
				return nil, nil, nil, err
			}
			tratamientos = append(tratamientos, t)
		}
		trows.Close()
		allTratamientos = append(allTratamientos, tratamientos)
	}

	var exp *parser.Expediente
	if len(visitas) > 0 {
		exp = &parser.Expediente{
			Paciente:    p,
			Doctor:      doctores[0],
			Visita:      visitas[0],
			Tratamiento: allTratamientos[0],
		}
	} else {
		exp = &parser.Expediente{Paciente: p}
	}

	return exp, visitas, allTratamientos, nil
}


// ListPatients devuelve todos los pacientes ordenados por nombre.
func (db *DB) ListPatients() ([]Patient, error) {
	rows, err := db.conn.Query(`SELECT id, nombre, curp, nss, fecha_nacimiento, telefono, domicilio FROM patients ORDER BY nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patients []Patient
	for rows.Next() {
		var p Patient
		if err := rows.Scan(&p.ID, &p.Nombre, &p.CURP, &p.NSS, &p.FechaNacimiento, &p.Telefono, &p.Domicilio); err != nil {
			return nil, err
		}
		patients = append(patients, p)
	}
	return patients, nil
}

// SaveRejectedFile registra un archivo rechazado (blur, formato, tamaño, etc.).
func (db *DB) SaveRejectedFile(filePath, fileHash, motivo string, blurScore float64) error {
	_, err := db.conn.Exec(`
		INSERT INTO rejected_files (file_path, file_hash, motivo, blur_score) VALUES (?, ?, ?, ?)
	`, filePath, fileHash, motivo, blurScore)
	return err
}

// SaveFailedFile registra un archivo que falló en la transcripción de la API.
func (db *DB) SaveFailedFile(filePath, fileHash, motivo string) error {
	_, err := db.conn.Exec(`
		INSERT INTO failed_files (file_path, file_hash, motivo) VALUES (?, ?, ?)
	`, filePath, fileHash, motivo)
	return err
}

// SearchByNombre busca pacientes cuyo nombre contenga el texto dado.
func (db *DB) SearchByNombre(nombre string) ([]Patient, error) {
	rows, err := db.conn.Query(`
		SELECT id, nombre, curp, nss, fecha_nacimiento, telefono, domicilio
		FROM patients WHERE nombre LIKE ? ORDER BY nombre
	`, "%"+nombre+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patients []Patient
	for rows.Next() {
		var p Patient
		if err := rows.Scan(&p.ID, &p.Nombre, &p.CURP, &p.NSS, &p.FechaNacimiento, &p.Telefono, &p.Domicilio); err != nil {
			return nil, err
		}
		patients = append(patients, p)
	}
	return patients, nil
}

// GetDBPath devuelve la ruta del archivo DB configurada en la variable de entorno.
func GetDBPath() string {
	if path := os.Getenv("MEDISCAN_DB_PATH"); path != "" {
		return path
	}
	return "./mediscan.db"
}
