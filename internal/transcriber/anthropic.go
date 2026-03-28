package transcriber

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"medscan/internal/parser"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// AnthropicTranscriber implementa Transcriber usando Anthropic Claude.
type AnthropicTranscriber struct{}

// anthropicRequest es la estructura del body enviado a la API de Anthropic.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type   string             `json:"type"`
	Source *anthropicSource   `json:"source,omitempty"` // para imágenes
	Text   string             `json:"text,omitempty"`   // para texto
}

type anthropicSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type"`
	Data      string `json:"data"` // base64
}

// anthropicResponse es la estructura de la respuesta de Anthropic.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// anthropicSystemPrompt es el prompt del sistema para Claude.
const anthropicSystemPrompt = `Eres un sistema de digitalización de expedientes médicos en México.
Analiza cuidadosamente la imagen del documento médico y extrae toda la información visible.
Responde SOLO con JSON válido siguiendo exactamente este esquema, sin markdown, sin explicaciones:

{
  "paciente": {
    "nombre": "nombre completo del paciente",
    "curp": "CURP de 18 caracteres o cadena vacía",
    "nss": "número de seguridad social o cadena vacía",
    "fecha_nacimiento": "YYYY-MM-DD o cadena vacía",
    "telefono": "teléfono o cadena vacía",
    "domicilio": "dirección completa o cadena vacía"
  },
  "doctor": {
    "nombre": "nombre completo del doctor",
    "cedula": "cédula profesional o cadena vacía",
    "especialidad": "especialidad médica o cadena vacía"
  },
  "visita": {
    "fecha": "YYYY-MM-DD o cadena vacía",
    "diagnostico": "diagnóstico o cadena vacía",
    "sintomas": "síntomas descritos o cadena vacía",
    "notas": "notas adicionales del médico o cadena vacía",
    "archivo_origen": "",
    "blur_score": 0,
    "procesado_en_ms": 0
  },
  "tratamiento": [
    {
      "medicamento": "nombre del medicamento",
      "dosis": "dosis",
      "frecuencia": "frecuencia de administración",
      "duracion": "duración del tratamiento",
      "indicaciones": "instrucciones adicionales"
    }
  ]
}

Si un campo no es visible o no aparece, usa "". Para tratamiento sin medicamentos usa [].`

// Transcribe toma una imagen, la envía a Claude y devuelve el expediente.
func (a *AnthropicTranscriber) Transcribe(imagePath string) (*parser.Expediente, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY no configurada. Obtén tu key en https://console.anthropic.com")
	}

	model := os.Getenv("MEDISCAN_ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-opus-4-5"
	}

	// Codificar imagen en base64
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("leer imagen: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)
	mimeType := mimeTypeFromPath(imagePath)

	// Construir request
	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: 2048,
		System:    anthropicSystemPrompt,
		Messages: []anthropicMessage{
			{
				Role: "user",
				Content: []anthropicBlock{
					{
						Type: "image",
						Source: &anthropicSource{
							Type:      "base64",
							MediaType: mimeType,
							Data:      b64,
						},
					},
					{
						Type: "text",
						Text: "Digitaliza este documento médico y devuelve el JSON.",
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serializar request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("crear request HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llamada HTTP a Anthropic: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("leer respuesta de Anthropic: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Anthropic respondió %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("parsear respuesta de Anthropic: %w", err)
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("error de Anthropic: %s", anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("respuesta vacía de Anthropic")
	}

	// Extraer y parsear el JSON del expediente
	jsonText := strings.TrimSpace(anthropicResp.Content[0].Text)

	var exp parser.Expediente
	if err := json.Unmarshal([]byte(jsonText), &exp); err != nil {
		return nil, fmt.Errorf("parsear JSON del expediente: %w (respuesta: %s)", err, jsonText[:minA(len(jsonText), 200)])
	}

	return &exp, nil
}

func minA(a, b int) int {
	if a < b {
		return a
	}
	return b
}
