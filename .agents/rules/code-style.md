# Reglas de código Go — mediscan

Estas reglas son obligatorias para todo código Go en este repositorio.

---

## 1. Formato y estilo

### Usa `gofmt` siempre

Todo el código debe pasar `gofmt` sin cambios. Configura tu editor para aplicarlo al guardar. En CI:

```bash
test -z "$(gofmt -l .)"
```

### Nombres en inglés, comentarios en español

El código (variables, funciones, tipos) va en inglés para seguir las convenciones de Go. Los comentarios de documentación van en español porque el equipo es hispanohablante.

```go
// BlurScore calcula la nitidez de una imagen usando la varianza del Laplaciano.
// Un score alto indica imagen nítida; score bajo indica imagen borrosa.
// Retorna ErrInvalidImage si el archivo no puede decodificarse.
func BlurScore(imagePath string) (float64, error) {
```

### Longitud de línea: máximo 100 caracteres

No es obligatorio en Go, pero líneas más largas se deben romper para legibilidad.

---

## 2. Manejo de errores

### Siempre propaga con contexto usando `%w`

```go
// MAL — error sin contexto, imposible de debuguear
if err != nil {
    return err
}

// BIEN — contexto claro de dónde y qué falló
if err != nil {
    return fmt.Errorf("imageproc: aplicando contraste a %s: %w", path, err)
}
```

El prefijo debe identificar el paquete (`imageproc:`, `store:`, `transcriber:`).

### Nunca ignores un error con `_`

```go
// MAL
result, _ := json.Marshal(data)

// BIEN
result, err := json.Marshal(data)
if err != nil {
    return fmt.Errorf("serializando expediente: %w", err)
}
```

Excepción única: cuando la documentación de la función garantiza que nunca retorna error (e.g., `bytes.Buffer.Write`).

### Errores centinela para casos de negocio conocidos

```go
// internal/store/db.go
var (
    ErrPatientNotFound = errors.New("paciente no encontrado")
    ErrDuplicateFile   = errors.New("archivo ya procesado")
)

// internal/imageproc/blur.go
var (
    ErrImageTooBlurry  = errors.New("imagen demasiado borrosa")
    ErrInvalidImage    = errors.New("imagen no válida o corrupta")
)
```

Úsalos en `cmd/` con `errors.Is()`:

```go
if errors.Is(err, imageproc.ErrImageTooBlurry) {
    fmt.Printf("  ⚠  %s está borrosa, tómala de nuevo\n", filepath.Base(f))
    continue
}
```

---

## 3. Estructura de funciones

### Guard clauses al inicio

```go
func (db *DB) SaveExpediente(p Patient, d Doctor, v Visit, treatments []Treatment) error {
    if p.Nombre == "" {
        return fmt.Errorf("store: nombre del paciente es requerido")
    }
    if v.Fecha == "" {
        return fmt.Errorf("store: fecha de visita es requerida")
    }
    // lógica principal
}
```

### Retorno temprano, sin `else` anidado

```go
// MAL
func processImage(path string) (string, error) {
    if path != "" {
        score, err := BlurScore(path)
        if err == nil {
            if score > threshold {
                // lógica...
            } else {
                return "", ErrImageTooBlurry
            }
        } else {
            return "", err
        }
    } else {
        return "", errors.New("path vacío")
    }
    return result, nil
}

// BIEN
func processImage(path string) (string, error) {
    if path == "" {
        return "", errors.New("path vacío")
    }
    score, err := BlurScore(path)
    if err != nil {
        return "", fmt.Errorf("calculando blur: %w", err)
    }
    if score < threshold {
        return "", ErrImageTooBlurry
    }
    // lógica principal
    return result, nil
}
```

### Límite de 50 líneas por función

Si una función supera ~50 líneas, extrae subfunciones con nombres descriptivos. El pipeline de scan debe ser legible de un vistazo.

---

## 4. Paquetes y dependencias

### Dependencias en una sola dirección

```
cmd/  →  internal/imageproc
cmd/  →  internal/transcriber
cmd/  →  internal/store
cmd/  →  internal/scanner

internal/transcriber  →  internal/parser
internal/imageproc    →  (solo stdlib + imaging)
internal/store        →  (solo stdlib + go-sqlite3)
internal/scanner      →  (solo stdlib)
```

`internal/` nunca importa `cmd/`. Los paquetes internos no se importan entre sí excepto la dependencia `transcriber → parser`.

### Agrupa los imports en tres bloques

```go
import (
    // 1. stdlib
    "encoding/json"
    "fmt"
    "image"
    "os"
    "path/filepath"

    // 2. dependencias externas
    "github.com/disintegration/imaging"
    "github.com/spf13/cobra"

    // 3. paquetes internos
    "mediscan/internal/imageproc"
    "mediscan/internal/store"
)
```

---

## 5. Pre-procesamiento de imagen (`internal/imageproc`)

### Limpia siempre los archivos temporales

```go
processedPath, err := imageproc.Preprocess(originalPath)
if err != nil {
    return err
}
defer os.Remove(processedPath) // SIEMPRE, incluso si hay errores después
```

El `defer` debe estar inmediatamente después de obtener el `processedPath`. No lo pongas más abajo en el flujo, podría no ejecutarse si hay un `return` temprano.

### Blur check SIEMPRE sobre la imagen original

El score de borrosidad se calcula sobre el archivo original, nunca sobre el pre-procesado:

```go
// CORRECTO — score sobre original
score, err := imageproc.BlurScore(originalPath)
if score < threshold {
    return ErrImageTooBlurry
}

// Después del check, pre-procesar
processedPath, err := imageproc.Preprocess(originalPath)

// MAL — calcular blur DESPUÉS del pre-procesamiento
// El contraste puede inflar el score artificialmente
```

### No modifiques la imagen original nunca

`Preprocess()` siempre escribe a un archivo temporal nuevo. El archivo original en la carpeta del usuario queda intacto. Si el usuario quiere conservar la versión pre-procesada, es una decisión explícita del usuario (flag futuro).

---

## 6. Integración con LLMs (`internal/transcriber`)

### La interfaz Transcriber es inviolable

```go
type Transcriber interface {
    Transcribe(imagePath string) (*parser.Expediente, error)
}
```

**Nunca cambies esta firma.** Si un nuevo LLM necesita parámetros adicionales, agrégalos en su struct constructor, no en la interfaz.

### Rate limiting de Gemini — es tu responsabilidad

El tier gratuito de Gemini tiene 10 RPM. El `GeminiTranscriber` debe dormir entre requests:

```go
// GeminiTranscriber.Transcribe debe incluir:
time.Sleep(6 * time.Second) // antes de cada request en modo free tier
```

Y backoff exponencial ante errores 429:

```go
func (g *GeminiTranscriber) transcribeWithRetry(path string) (*parser.Expediente, error) {
    delays := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second}
    for i, delay := range delays {
        exp, err := g.doRequest(path)
        if err == nil {
            return exp, nil
        }
        if !isRateLimitError(err) {
            return nil, err // error distinto al 429, no reintentes
        }
        if i < len(delays)-1 {
            log.Printf("rate limit de Gemini, esperando %s...", delay)
            time.Sleep(delay)
        }
    }
    return nil, fmt.Errorf("transcriber: límite de API de Gemini agotado tras 3 intentos")
}
```

### El system prompt es la pieza más sensible del proyecto

Está en cada implementación de Transcriber. Cualquier cambio al prompt **debe** ir acompañado de:
1. Pruebas con al menos 5 imágenes de `testdata/`
2. Comparación del JSON resultante antes/después
3. Descripción del cambio en el commit

Un prompt roto afecta el 100% del procesamiento.

### Parseo defensivo del JSON de respuesta

El LLM puede devolver JSON envuelto en backticks markdown. Siempre limpia antes de parsear:

```go
func cleanJSON(raw string) string {
    raw = strings.TrimSpace(raw)
    raw = strings.TrimPrefix(raw, "```json")
    raw = strings.TrimPrefix(raw, "```")
    raw = strings.TrimSuffix(raw, "```")
    return strings.TrimSpace(raw)
}
```

---

## 7. Concurrencia

### El pipeline de scan es secuencial por default

Gemini gratuito tiene 10 RPM. Procesar en paralelo sin control dispara errores 429. El comando `scan` procesa archivos uno por uno con el sleep incorporado en `GeminiTranscriber`.

Si en el futuro se habilita procesamiento paralelo (para Anthropic pagado), usar un semáforo explícito:

```go
const maxConcurrent = 3
sem := make(chan struct{}, maxConcurrent)

for _, file := range files {
    sem <- struct{}{}
    go func(f string) {
        defer func() { <-sem }()
        processFile(f)
    }(file)
}
// Esperar a que terminen todas las goroutines
for i := 0; i < maxConcurrent; i++ {
    sem <- struct{}{}
}
```

---

## 8. Tests

### Nombra los tests con `TestNombreFuncion_Escenario`

```go
func TestBlurScore_ImagenNítida(t *testing.T) { ... }
func TestBlurScore_ImagenBorrosa(t *testing.T) { ... }
func TestPreprocess_ReduceAncho(t *testing.T) { ... }
func TestSaveExpediente_CURPDuplicada(t *testing.T) { ... }
```

### DB en memoria para tests de store

```go
func testDB(t *testing.T) *store.DB {
    t.Helper()
    db, err := store.New(":memory:")
    if err != nil {
        t.Fatalf("abriendo DB de test: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

### Imágenes de prueba en `testdata/`

El directorio `testdata/` contiene imágenes ficticias para tests de `imageproc`. Deben cubrir:
- `testdata/sharp.jpg` — imagen nítida de documento de texto
- `testdata/blurry.jpg` — imagen borrosa
- `testdata/dark.jpg` — imagen oscura pero nítida
- `testdata/wide.jpg` — imagen de más de 1200px de ancho

**Nunca** agregues fotos de documentos médicos reales al repositorio.

### Marca los tests de integración

```go
func TestGeminiTranscribe_Integracion(t *testing.T) {
    if os.Getenv("MEDISCAN_INTEGRATION") == "" {
        t.Skip("requiere MEDISCAN_INTEGRATION=true y GEMINI_API_KEY")
    }
    // ...
}
```

---

## 9. Logging y output al usuario

### Stdout para output normal, stderr para errores

```go
// Output normal → stdout
fmt.Printf("  ✓ [blur:%.0f] [gemini:✓] %s — %s\n", score, filepath.Base(f), patient.Nombre)

// Errores → stderr
fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", filepath.Base(f), err)
```

### El resumen final del scan es obligatorio

Al terminar un `scan`, siempre imprime:

```
══════════════════════════════════════
  Escaneo completado
══════════════════════════════════════
  Procesados:    18
  Rechazados:     2  (borrosos — ver arriba)
  Duplicados:     1  (ya en base de datos)
  Errores API:    0
```

### No imprimas datos sensibles en logs de debug

```go
// MAL
log.Printf("debug: procesando paciente %s CURP %s", p.Nombre, p.CURP)

// BIEN
log.Printf("debug: procesando archivo %s (%d bytes)", filepath.Base(path), len(data))
```

---

## 10. SQL

### Siempre usa placeholders

```go
// MAL — SQL injection
query := "SELECT * FROM patients WHERE curp = '" + curp + "'"

// BIEN
row := db.conn.QueryRow("SELECT * FROM patients WHERE curp = ?", curp)
```

### Cierra siempre `*sql.Rows` y verifica `rows.Err()`

```go
rows, err := db.conn.Query("SELECT * FROM patients")
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    // ...
}
return rows.Err() // errores de iteración
```

### Transacciones para escrituras múltiples

Cualquier operación que escriba en más de una tabla va en una transacción con `defer tx.Rollback()`:

```go
tx, err := db.conn.Begin()
if err != nil {
    return err
}
defer tx.Rollback() // no-op si ya se hizo Commit

// ... inserts ...

return tx.Commit()
```

---

## 11. Comandos Cobra

### Usa `RunE` (no `Run`) para poder retornar errores

```go
var scanCmd = &cobra.Command{
    Use:   "scan [carpeta]",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return doScan(args[0])
    },
}
```

### Define flags en `init()` del archivo del comando

```go
// cmd/scan.go
func init() {
    scanCmd.Flags().Bool("recursive", false, "Procesar subcarpetas")
    scanCmd.Flags().Bool("debug-blur", false, "Mostrar score de blur sin rechazar")
}
```

---

## Checklist antes de hacer commit

- [ ] `gofmt -l .` no devuelve archivos
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./...` pasa (tests unitarios, sin integración)
- [ ] Los errores nuevos tienen contexto con `%w`
- [ ] Los archivos temporales de `imageproc` tienen `defer os.Remove()`
- [ ] No hay API keys ni datos de pacientes en el código o tests
- [ ] El blur check ocurre ANTES del pre-procesamiento en el pipeline
- [ ] `GeminiTranscriber` tiene sleep de 6s y backoff ante 429
- [ ] Las funciones nuevas tienen comentario en español
- [ ] Si se modificó el schema de DB, hay migración compatible con `ALTER TABLE`