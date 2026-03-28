package parser

// Expediente es el struct central que representa un documento médico digitalizado.
// Es el contrato de datos entre el LLM y la base de datos.
type Expediente struct {
	Paciente    Paciente      `json:"paciente"`
	Doctor      Doctor        `json:"doctor"`
	Visita      Visita        `json:"visita"`
	Tratamiento []Tratamiento `json:"tratamiento"`
}

// Paciente contiene los datos demográficos del paciente.
// CURP es la llave única (ADR-005).
type Paciente struct {
	Nombre          string `json:"nombre"`
	CURP            string `json:"curp"`
	NSS             string `json:"nss"`
	FechaNacimiento string `json:"fecha_nacimiento"` // YYYY-MM-DD
	Telefono        string `json:"telefono"`
	Domicilio       string `json:"domicilio"`
}

// Doctor contiene los datos del médico tratante.
// Cedula es la llave única.
type Doctor struct {
	Nombre       string `json:"nombre"`
	Cedula       string `json:"cedula"`
	Especialidad string `json:"especialidad"`
}

// Visita representa una consulta médica individual.
type Visita struct {
	Fecha         string  `json:"fecha"` // YYYY-MM-DD
	Diagnostico   string  `json:"diagnostico"`
	Sintomas      string  `json:"sintomas"`
	Notas         string  `json:"notas"`
	ArchivoOrigen string  `json:"archivo_origen"`
	BlurScore     float64 `json:"blur_score"`
	ProcesadoEnMs int64   `json:"procesado_en_ms"`
}

// Tratamiento representa un medicamento recetado en una visita.
type Tratamiento struct {
	Medicamento  string `json:"medicamento"`
	Dosis        string `json:"dosis"`
	Frecuencia   string `json:"frecuencia"`
	Duracion     string `json:"duracion"`
	Indicaciones string `json:"indicaciones"`
}
