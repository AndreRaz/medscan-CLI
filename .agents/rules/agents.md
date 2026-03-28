---
trigger: always_on
---

# AGENTS.md — mediscan

Instrucciones para agentes de IA (Claude Code, Cursor, Copilot, etc.) que trabajen en este repositorio.

---

## ¿Qué es este proyecto?

`mediscan` es una CLI en Go para digitalizar expedientes médicos en papel. Lee imágenes de documentos, las pre-procesa localmente (escala de grises, contraste, recorte), detecta fotos borrosas antes de gastar API calls, transcribe el contenido con un LLM de visión, y guarda el resultado estructurado en SQLite.

---

## Stack

| Capa | Tecnología |
|---|---|
| Lenguaje | Go 1.22+ |
| CLI framework | Cobra (`github.com/spf13/cobra`) |
| Pre-procesamiento de imagen | `github.com/disintegration/imaging` |
| Base de datos | SQLite via `github.com/mattn/go-sqlite3` |
| LLM (dev/gratis) | Google Gemini 2.5 Flash (HTTP directo) |
| LLM (producción) | Anthropic Claude (HTTP directo) |
| Config | `github.com/joho/godotenv` + env vars |
| Testing | `testing` estándar + `testify` |

---

## Estructura del repositorio

```
mediscan/
├── cmd/
│   ├── root.go           # Comando raíz, inicialización de DB
│   ├── scan.go           # mediscan scan [carpeta]
│   ├── query.go          # mediscan query [curp]
│   ├── patient.go        # mediscan patient list
│   └── export.go         # mediscan export [curp]
├── internal/
│   ├── imageproc/        # Pipeline de pre-procesamiento de imagen
│   │   ├── preprocess.go # Preprocess(path) → tempPath, error
│   │   └── blur.go       # BlurScore(path) → float64, error
│   ├── transcriber/      # Integración con LLMs (interfaz común)
│   │   ├── transcriber.go  # interface Transcriber
│   │   ├── gemini.go       # Implementación Google Gemini
│   │   └── anthropic.go    # Implementación Anthropic Claude
│   ├── parser/
│   │   └── schema.go     # struct Expediente
│   ├── store/
│   │   ├── db.go         # New(), SaveExpediente(), GetExpediente()
│   │   └── models.go     # Patient, Doctor, Visit, Treatment
│   └── scanner/
│       └── reader.go     # ScanFolder(), HashFile()
├── testdata/             # Imágenes de prueba ficticias (nunca datos reales)
├── main.go
├── go.mod
├── .env.example
├── PRD.md
├── AGENTS.md             # Este archivo
└── rules.go.md
```

---

## La interfaz Transcriber — pieza central del diseño

```go
// internal/transcriber/transcriber.go
package transcriber

import "mediscan/internal/parser"

// Transcriber es la única interfaz que los comandos conocen.
// Cualquier LLM nuevo solo necesita implementar esta interfaz.
type Transcriber interface {
    Transcribe(imagePath string) (*parser.Expediente, error)
}

// New devuelve el Transcriber correcto según la variable de entorno MEDISCAN_PROVIDER.
func New() Transcriber {
    switch os.Getenv("MEDISCAN_PROVIDER") {
    case "gemini":
        return &GeminiTranscriber{}
    default:
        return &AnthropicTranscriber{}
    }
}
```

**Regla crítica:** Los comandos en `cmd/` solo llaman `transcriber.New().Transcribe(path)`. Nunca importan `gemini.go` ni `anthropic.go` directamente. El proveedor es un detalle de implementación invisible para el resto del código.

---

## El pipeline de imagen — orden de ejecución obligatorio

El pipeline en `cmd/scan.go` sigue este orden exacto para cada archivo. **No cambies el orden sin actualizar el PRD:**

```go
// 1. Hash para deduplicación
hash, err := scanner.HashFile(filePath)
// si hash ya existe en DB → skip

// 2. Detección de borrosidad (antes de cualquier otra cosa)
score, err := imageproc.BlurScore(filePath)
if score < blurThreshold {
    // registra en rejected_files, imprime mensaje, continúa con siguiente
    continue
}

// 3. Pre-procesamiento local
processedPath, err := imageproc.Preprocess(filePath)
defer os.Remove(processedPath) // siempre limpiar

// 4. Transcripción con LLM
exp, err := t.Transcribe(processedPath)

// 5. Guardar en DB
err = db.SaveExpediente(...)
```

---

## Implementación de BlurScore

El algoritmo es Varianza del Laplaciano. Implementarlo desde cero en Go usando solo `image` de stdlib:

```go
// internal/imageproc/blur.go
func BlurScore(imagePath string) (float64, error) {
    // 1. Abrir imagen
    // 2. Convertir a escala de grises
    // 3. Aplicar kernel Laplaciano 3x3: [0,1,0],[1,-4,1],[0,1,0]
    // 4. Calcular varianza de los valores resultantes
    // score alto = imagen nítida, score bajo = imagen borrosa
}
```

El kernel Laplaciano se implementa como convolución manual sobre los píxeles — no se requiere ninguna librería de visión computacional.

---

## Implementación del Transcriber de Gemini

La API de Gemini acepta imágenes en base64 en el campo `inlineData`:

```go
// internal/transcriber/gemini.go

const geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// El body del request tiene esta estructura:
{
  "contents": [{
    "parts": [
      {
        "inlineData": {
          "mimeType": "image/jpeg",
          "data": "<base64>"
        }
      },
      {
        "text": "Digitaliza este documento médico. Responde SOLO con JSON válido..."
      }
    ]
  }],
  "generationConfig": {
    "temperature": 0.1,
    "responseMimeType": "application/json"
  }
}

// La respuesta de Gemini viene en:
// response.candidates[0].content.parts[0].text
```

**Rate limiting de Gemini:** el tier gratuito permite 10 RPM. El transcriber de Gemini debe implementar un sleep de 6 segundos entre requests para no superar ese límite. Con backoff ante 429:

```go
// Espera base entre requests en modo Gemini gratuito
time.Sleep(6 * time.Second)

// Ante error 429, backoff exponencial:
// intento 1: espera 10s
// intento 2: espera 20s
// intento 3: espera 40s → si falla, retorna error
```

---

## Implementación del Pre-procesamiento

```go
// internal/imageproc/preprocess.go
// Usa github.com/disintegration/imaging

func Preprocess(inputPath string) (outputPath string, err error) {
    img, err := imaging.Open(inputPath)

    // 1. Escala de grises
    gray := imaging.Grayscale(img)

    // 2. Contraste (factor desde env var MEDISCAN_CONTRAST, default 1.3)
    contrasted := imaging.AdjustContrast(gray, contrastFactor)

    // 3. Resize si ancho > MEDISCAN_MAX_WIDTH (default 1200px)
    if bounds.Width > maxWidth {
        resized = imaging.Resize(contrasted, maxWidth, 0, imaging.Lanczos)
    }

    // 4. Guardar en directorio temporal
    tmpPath := filepath.Join(os.TempDir(), "mediscan_"+filepath.Base(inputPath))
    err = imaging.Save(resized, tmpPath)
    return tmpPath, err
}
```

---

## Guía para tareas comunes

### Agregar un nuevo proveedor de LLM (e.g., OpenAI)

1. Crear `internal/transcriber/openai.go` con struct `OpenAITranscriber` que implemente `Transcriber`
2. Agregar el case en `transcriber.New()`: `case "openai": return &OpenAITranscriber{}`
3. Agregar `OPENAI_API_KEY` a `.env.example` y documentarlo en el PRD
4. No tocar nada en `cmd/`

### Agregar un nuevo campo al expediente

1. `internal/parser/schema.go` — campo en el struct
2. `internal/store/models.go` — campo en el model
3. `internal/store/db.go` — columna en `CREATE TABLE` y `ALTER TABLE` para migraciones
4. `internal/transcriber/gemini.go` y `anthropic.go` — añadir al system prompt
5. Tests de `store` con la nueva columna

### Ajustar el umbral de borrosidad para un consultorio específico

Solo cambiar `MEDISCAN_BLUR_THRESHOLD` en el `.env`. No requiere código. Si el consultorio usa un escáner (no celular), el umbral puede bajarse a 50. Si usan celular en mala luz, puede subirse a 150.

### Debuguear por qué una imagen está siendo rechazada

```bash
# El flag --debug-blur imprime el score sin rechazar
./mediscan scan ./docs/ --debug-blur

# O desactiva completamente la detección
MEDISCAN_BLUR_THRESHOLD=0 ./mediscan scan ./docs/
```

---

## Qué NO hacer

- No llames a la API de ningún LLM desde fuera de `internal/transcriber/`
- No implementes lógica de imágenes fuera de `internal/imageproc/`
- No agregues lógica de negocio en `cmd/` — solo orquestación
- No uses `log.Fatal` dentro de `internal/` — devuelve errores
- No hardcodees API keys — siempre desde `os.Getenv()`
- No llames a la API antes de pasar por `BlurScore` — es un desperdicio de dinero
- No guardes la imagen pre-procesada en la misma carpeta que el original — usa `os.TempDir()`
- No dejes archivos temporales sin limpiar — usa `defer os.Remove(processedPath)`
- No imprimas datos del paciente en logs de debug

---

## Variables de entorno

| Variable | Requerida | Default | Descripción |
|---|---|---|---|
| `MEDISCAN_PROVIDER` | No | `anthropic` | `gemini` o `anthropic` |
| `ANTHROPIC_API_KEY` | Si provider=anthropic | — | API key de Anthropic |
| `GEMINI_API_KEY` | Si provider=gemini | — | API key de Google AI Studio |
| `MEDISCAN_ANTHROPIC_MODEL` | No | `claude-opus-4-5` | Modelo de Anthropic |
| `MEDISCAN_GEMINI_MODEL` | No | `gemini-2.5-flash` | Modelo de Gemini |
| `MEDISCAN_DB_PATH` | No | `./mediscan.db` | Ruta al archivo SQLite |
| `MEDISCAN_MAX_WIDTH` | No | `1200` | Ancho máximo en px antes de API |
| `MEDISCAN_CONTRAST` | No | `1.3` | Factor de contraste (1.0 = sin cambio) |
| `MEDISCAN_BLUR_THRESHOLD` | No | `100.0` | Varianza mínima (0 = desactiva) |
| `MEDISCAN_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, `error` |
| `MEDISCAN_INTEGRATION` | No | — | Activa tests de integración real |

---

## Decisiones de arquitectura (ADRs)

### ADR-001: SQLite en lugar de Postgres

**Decisión:** SQLite como único almacenamiento.
**Razón:** El usuario objetivo no tiene infraestructura de servidor. Un archivo `.db` se puede respaldar con `cp`. Sin instalación, sin configuración.
**Trade-off:** No soporta múltiples escrituras concurrentes, pero mediscan es single-user.

### ADR-002: HTTP directo sin SDK para los LLMs

**Decisión:** Llamar a Gemini y Anthropic con `net/http` estándar.
**Razón:** Evita dependencias de SDKs que cambian frecuentemente. El contrato HTTP es estable. Mantiene el binario pequeño y el código completamente bajo nuestro control.

### ADR-003: Pre-procesamiento antes del blur check

**Decisión:** El blur check se hace sobre la imagen ORIGINAL, no sobre la pre-procesada.
**Razón:** El pre-procesamiento (especialmente el aumento de contraste) puede artificialmente elevar el score de varianza en imágenes borrosas. El blur check debe reflejar la calidad real de la captura, no un artefacto del procesamiento.

### ADR-004: Gemini como proveedor de desarrollo

**Decisión:** Usar Gemini 2.5 Flash (gratis) durante desarrollo, Anthropic en producción.
**Razón:** Gemini tiene tier gratuito sin tarjeta de crédito. Es suficientemente bueno para probar el pipeline completo. La interfaz `Transcriber` hace el cambio transparente.
**Trade-off:** El tier gratuito tiene 250 requests/día. Para pruebas de lotes grandes, implementar sleep de 6 segundos entre requests.

### ADR-005: CURP como llave de paciente

**Decisión:** CURP como identificador único de paciente (upsert key).
**Razón:** Es el identificador nacional estándar en México. Garantiza unicidad entre consultorios.
**Trade-off:** Si el documento no tiene CURP, el campo queda vacío y puede crear duplicados. Mitigación: validar longitud de CURP (18 chars) antes de usarla como key.

### ADR-006: Archivos temporales en os.TempDir()

**Decisión:** La imagen pre-procesada se guarda en el directorio temporal del sistema.
**Razón:** No contamina la carpeta del usuario con archivos intermedios. El sistema operativo los limpia automáticamente en reinicios. Un `defer os.Remove()` garantiza la limpieza inmediata.