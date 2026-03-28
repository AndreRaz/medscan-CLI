package transcriber

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"medscan/internal/parser"
)

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// GeminiTranscriber implementa Transcriber usando Google Gemini 2.5 Flash.
// Rate limit del tier gratuito: 10 RPM → sleep de 6s entre requests.
type GeminiTranscriber struct{}

// geminiRequest es la estructura del body enviado a la API de Gemini.
type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
	Text       string            `json:"text,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType"`
}

// geminiResponse es la estructura de la respuesta de Gemini.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// systemPrompt es el prompt enviado a Gemini para digitalizar documentos médicos.
const geminiSystemPrompt = `Eres un sistema de digitalización de expedientes médicos en México.
Analiza cuidadosamente la imagen del documento médico y extrae toda la información visible.
Responde SOLO con JSON válido siguiendo exactamente este esquema:

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

Si un campo no es visible o no aparece en el documento, usa cadena vacía "". 
Para tratamiento, devuelve un arreglo vacío [] si no hay medicamentos.
IMPORTANTE: Responde ÚNICAMENTE con el JSON. Sin explicaciones, sin markdown, sin bloques de código.`

// Transcribe toma una imagen, la envía a Gemini y devuelve el expediente.
func (g *GeminiTranscriber) Transcribe(imagePath string) (*parser.Expediente, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY no configurada. Obtenla en https://aistudio.google.com")
	}

	model := os.Getenv("MEDISCAN_GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}

	// Codificar imagen en base64
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("leer imagen: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)

	// Determinar MIME type por extensión
	mimeType := mimeTypeFromPath(imagePath)

	// Construir request body
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{
						InlineData: &geminiInlineData{
							MimeType: mimeType,
							Data:     b64,
						},
					},
					{
						Text: geminiSystemPrompt,
					},
				},
			},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:      0.1,
			ResponseMimeType: "application/json",
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("serializar request: %w", err)
	}

	url := fmt.Sprintf(geminiAPIURL, model, apiKey)

	// Backoff exponencial ante error 429 (máximo 3 intentos)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			wait := time.Duration(10*(1<<(attempt-1))) * time.Second // 10s, 20s, 40s
			fmt.Printf("  ⏳ Gemini rate limit (429). Esperando %v antes de reintentar (intento %d/3)...\n", wait, attempt+1)
			time.Sleep(wait)
		}

		// Sleep base para respetar 10 RPM del tier gratuito (6s entre requests)
		if attempt == 0 {
			time.Sleep(6 * time.Second)
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			lastErr = fmt.Errorf("llamada HTTP a Gemini: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("leer respuesta de Gemini: %w", err)
			continue
		}

		// Manejar rate limit
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limit de Gemini (429)")
			continue
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("Gemini respondió %d: %s", resp.StatusCode, string(body))
			break // No reintentar en errores que no sean 429
		}

		// Parsear respuesta
		var geminiResp geminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			lastErr = fmt.Errorf("parsear respuesta de Gemini: %w", err)
			break
		}

		if geminiResp.Error != nil {
			lastErr = fmt.Errorf("error de Gemini %d: %s", geminiResp.Error.Code, geminiResp.Error.Message)
			break
		}

		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			lastErr = fmt.Errorf("respuesta vacía de Gemini")
			break
		}

		// Extraer JSON del texto de respuesta
		jsonText := geminiResp.Candidates[0].Content.Parts[0].Text
		jsonText = strings.TrimSpace(jsonText)

		var exp parser.Expediente
		if err := json.Unmarshal([]byte(jsonText), &exp); err != nil {
			lastErr = fmt.Errorf("parsear JSON del expediente: %w (respuesta: %s)", err, jsonText[:min(len(jsonText), 200)])
			break
		}

		return &exp, nil
	}

	return nil, fmt.Errorf("Gemini falló después de 3 intentos: %w", lastErr)
}

// mimeTypeFromPath devuelve el MIME type según la extensión del archivo.
func mimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "image/jpeg"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
