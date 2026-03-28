package transcriber

import (
	"os"

	"medscan/internal/parser"
)

// Transcriber es la interfaz única que los comandos conocen.
// Cualquier proveedor de LLM nuevo solo necesita implementar esta interfaz.
// Los comandos en cmd/ nunca importan gemini.go ni anthropic.go directamente (AGENTS.md).
type Transcriber interface {
	Transcribe(imagePath string) (*parser.Expediente, error)
}

// New devuelve el Transcriber correcto según MEDISCAN_PROVIDER.
// Default: anthropic (producción). Usar "gemini" para desarrollo con el tier gratuito.
func New() Transcriber {
	switch os.Getenv("MEDISCAN_PROVIDER") {
	case "gemini":
		return &GeminiTranscriber{}
	default:
		return &AnthropicTranscriber{}
	}
}
