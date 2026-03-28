package store

// Patient representa un paciente en la base de datos.
type Patient struct {
	ID              int64
	Nombre          string
	CURP            string
	NSS             string
	FechaNacimiento string
	Telefono        string
	Domicilio       string
}

// Doctor representa un médico en la base de datos.
type Doctor struct {
	ID           int64
	Nombre       string
	Cedula       string
	Especialidad string
}

// Visit representa una visita médica en la base de datos.
type Visit struct {
	ID            int64
	PatientID     int64
	DoctorID      int64
	Fecha         string
	Diagnostico   string
	Sintomas      string
	Notas         string
	ArchivoOrigen string
	FileHash      string
	BlurScore     float64
	ProcesadoEnMs int64
}

// Treatment representa un tratamiento farmacológico en la base de datos.
type Treatment struct {
	ID           int64
	VisitID      int64
	Medicamento  string
	Dosis        string
	Frecuencia   string
	Duracion     string
	Indicaciones string
}

// RejectedFile representa un archivo rechazado durante el scan.
type RejectedFile struct {
	ID        int64
	FilePath  string
	FileHash  string
	Motivo    string // "blur", "formato", "tamaño", "corrupto"
	BlurScore float64
	ScannedAt string
}

// FailedFile representa un archivo que falló en la transcripción de la API.
type FailedFile struct {
	ID        int64
	FilePath  string
	FileHash  string
	Motivo    string
	ScannedAt string
}
