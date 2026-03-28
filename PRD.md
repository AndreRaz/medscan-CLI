# PRD — medscan CLI
**Product Requirements Document v1.1**
**Fecha:** 2026-03-28
**Estado:** Draft

---

## 1. Contexto y problema

Los consultorios médicos pequeños y medianos en México siguen manejando expedientes clínicos en papel. Esto genera tres problemas concretos:

- **Pérdida de información**: documentos deteriorados, extraviados o ilegibles con el tiempo.
- **Acceso lento**: encontrar el historial de un paciente toma minutos u horas.
- **Sin visibilidad cruzada**: un médico nuevo no puede ver los tratamientos anteriores sin revisar toda la carpeta física.

`medscan` resuelve esto: una CLI en Go que digitaliza documentos médicos usando visión por IA, los estructura en JSON y los persiste en una base de datos local consultable.

---

## 2. Objetivo del producto

Construir una CLI en Go que permita a personal administrativo o médico:

1. Apuntar a una carpeta con fotos/escaneos de documentos en papel.
2. Pre-procesar las imágenes **localmente** (blanco y negro, contraste, recorte) antes de enviarlas a la nube.
3. Detectar y rechazar fotos borrosas **antes de gastar tokens de API**.
4. Transcribir automáticamente el contenido a un esquema JSON estructurado.
5. Guardar el expediente en una base de datos local (SQLite).
6. Consultar el expediente completo de un paciente en cualquier momento.

**Definición de éxito:** Un usuario sin conocimientos técnicos puede digitalizar 20 documentos en menos de 5 minutos con menos del 5% de error en campos críticos, y la CLI rechaza fotos malas antes de enviarlas a la API.

---

## 3. Usuarios objetivo

| Rol | Necesidad principal |
|---|---|
| Administrador de consultorio | Digitalizar lotes de documentos acumulados |
| Médico tratante | Consultar historial completo antes de la cita |
| Médico de guardia | Ver tratamientos previos de un paciente nuevo |

---

## 4. Proveedor de LLM

### Para desarrollo y pruebas (gratis, sin tarjeta)
**Google Gemini 2.5 Flash** — tier gratuito:
- 10 requests por minuto (RPM)
- 250 requests por día (RPD)
- Sin costo, sin tarjeta de crédito
- Soporta imágenes en base64
- Endpoint: `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent`

> **Cómo obtener la API key:** ir a https://aistudio.google.com → "Get API key" → crear proyecto → copiar key. No requiere tarjeta.

### Para producción
**Anthropic Claude** (`claude-opus-4-5`) — mayor precisión en documentos médicos con letra difícil.

El proveedor es **intercambiable** mediante variable de entorno `MEDISCAN_PROVIDER=gemini|anthropic`. La interfaz interna `Transcriber` abstrae completamente al proveedor — el resto del código no cambia.

---

## 5. Pipeline de procesamiento de imagen

Antes de enviar cualquier imagen a la API, la CLI ejecuta un pipeline local en Go:

```
Imagen original (foto con celular del doctor)
        │
        ▼
[1] Validación de formato y tamaño
        │  Rechaza archivos > 20MB o formatos no soportados
        ▼
[2] Detección de borrosidad (Varianza del Laplaciano)
        │  Si score < umbral → rechaza con mensaje claro al usuario
        │  No se hace ninguna llamada a la API
        ▼
[3] Pre-procesamiento (disintegration/imaging)
        │  · Convertir a escala de grises
        │  · Aumentar contraste
        │  · Ajustar brillo si la imagen está muy oscura
        │  · Recorte automático de bordes blancos
        │  · Redimensionar a máx. 1200px de ancho
        ▼
[4] Codificación base64 → envío a API
        ▼
[5] Parseo de JSON → guardado en SQLite
```

### Beneficios del pre-procesamiento local

- **Menos errores de transcripción**: el LLM procesa texto negro sobre fondo blanco limpio.
- **Menos tokens**: imágenes más pequeñas y limpias consumen menos tokens de imagen.
- **Ahorro real**: imágenes borrosas no consumen ningún API call.
- **Feedback inmediato**: el doctor sabe en segundos si necesita retomar la foto.

---

## 6. Detección de calidad de imagen

### Algoritmo: Varianza del Laplaciano

La borrosidad se detecta calculando la varianza del filtro Laplaciano sobre la imagen en escala de grises. Una imagen nítida tiene varianza alta (bordes bien definidos). Una borrosa tiene varianza baja.

```
score = varianza(laplaciano(imagen_en_gris))

si score < MEDISCAN_BLUR_THRESHOLD (default: 100.0):
    → rechaza sin llamar a la API
    → imprime: "⚠  receta_marzo.jpg borrosa (score: 42.3). Tómela de nuevo."
    → registra en tabla rejected_files con motivo y score
```

### Tabla de referencia de scores

| Condición de la foto | Score típico | Acción |
|---|---|---|
| Nítida, buena iluminación | 200–800 | Continúa al pre-procesamiento |
| Ligeramente movida | 80–150 | Advertencia, continúa |
| Muy borrosa | < 80 | Rechazada, no se envía a API |
| Completamente fuera de foco | < 20 | Rechazada con error claro |

El umbral es configurable. Con `MEDISCAN_BLUR_THRESHOLD=0` se desactiva la detección (útil para debug).

---

## 7. Esquema de datos

```json
{
  "paciente": {
    "nombre": "string",
    "curp": "string — llave única",
    "nss": "string",
    "fecha_nacimiento": "YYYY-MM-DD",
    "telefono": "string",
    "domicilio": "string"
  },
  "doctor": {
    "nombre": "string",
    "cedula": "string — llave única",
    "especialidad": "string"
  },
  "visita": {
    "fecha": "YYYY-MM-DD",
    "diagnostico": "string",
    "sintomas": "string",
    "notas": "string",
    "archivo_origen": "string",
    "blur_score": "float64 — score de nitidez al momento de escanear",
    "procesado_en_ms": "int64 — duración total del pipeline"
  },
  "tratamiento": [
    {
      "medicamento": "string",
      "dosis": "string",
      "frecuencia": "string",
      "duracion": "string",
      "indicaciones": "string"
    }
  ]
}
```

---

## 8. Alcance — v1.0

### Dentro del alcance

- `mediscan scan [carpeta]` — pipeline completo: validación → blur → preproceso → API → DB
- `mediscan query [curp|--nombre]` — expediente completo de un paciente
- `mediscan patient list` — lista de todos los pacientes
- `mediscan export [curp]` — exporta expediente a JSON
- Pre-procesamiento local: escala de grises, contraste, recorte, resize
- Detección de borrosidad antes de cada llamada a API
- Soporte dual de proveedores: Gemini (dev/gratis) y Anthropic (producción)
- Base de datos local SQLite sin servidor
- Deduplicación por SHA-256

### Fuera del alcance (v2+)

- Interfaz web o GUI
- Soporte nativo de PDF
- Corrección de perspectiva (foto tomada en ángulo)
- OCR completamente offline
- Multi-usuario / autenticación

---

## 9. Criterios de aceptación

### Pre-procesamiento de imagen
- [ ] La imagen pre-procesada es visiblemente más legible que el original
- [ ] Imágenes de más de 1200px de ancho se reducen antes del envío
- [ ] El pipeline toma menos de 500ms por imagen
- [ ] Los archivos temporales del pre-procesamiento se eliminan al terminar

### Detección de borrosidad
- [ ] Imágenes borrosas son rechazadas antes de llamar a cualquier API
- [ ] El score de blur se imprime para que el usuario entienda el rechazo
- [ ] El umbral es configurable sin recompilar
- [ ] Al final del `scan`, se lista cuántas fotos fueron rechazadas y sus scores

### Integración con Gemini gratuito
- [ ] Con `MEDISCAN_PROVIDER=gemini` usa Gemini en lugar de Anthropic
- [ ] Respeta el rate limit de 10 RPM con sleep automático entre requests
- [ ] Ante error 429 de Gemini, espera y reintenta con backoff exponencial (máx 3 intentos)

### Pipeline completo
- [ ] Una imagen borrosa nunca llega a la API (0 API calls desperdiciados)
- [ ] El log muestra el estado de cada etapa: `[blur:245.3] [resize:1200px] [gemini:✓]`
- [ ] El resumen final del `scan` muestra: procesados / rechazados por blur / errores de API / duplicados

---

## 10. Manejo de errores

| Escenario | Comportamiento |
|---|---|
| Foto borrosa (score < umbral) | Rechazada antes de API, mensaje con score, registrada en DB |
| Foto oscura pero nítida | Pre-procesamiento ajusta brillo, continúa normalmente |
| Error 429 de Gemini | Espera con backoff exponencial, reintenta hasta 3 veces |
| Gemini regresa JSON malformado | Log de error, guarda en `failed_files`, continúa |
| API key no configurada | Error inmediato con instrucciones precisas |
| Imagen corrupta (no decodificable) | Log de error, continúa con siguiente archivo |
| Documento sin texto (foto de pared) | Campos vacíos en JSON, se guarda con advertencia `[sin_datos]` |

---

## 11. Configuración completa

```bash
# Proveedor de LLM (default: anthropic)
MEDISCAN_PROVIDER=gemini              # gemini | anthropic

# API Keys
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AIzaSy...              # obtener en aistudio.google.com

# Modelos
MEDISCAN_ANTHROPIC_MODEL=claude-opus-4-5
MEDISCAN_GEMINI_MODEL=gemini-2.5-flash

# Base de datos
MEDISCAN_DB_PATH=./mediscan.db

# Pre-procesamiento de imagen
MEDISCAN_MAX_WIDTH=1200               # px máximo antes de enviar a API
MEDISCAN_CONTRAST=1.3                 # multiplicador de contraste (1.0 = sin cambio)
MEDISCAN_BLUR_THRESHOLD=100.0         # varianza mínima aceptable (0 = desactivado)

# Logging
MEDISCAN_LOG_LEVEL=info               # debug | info | warn | error
```

---

## 12. Métricas de calidad

| Métrica | Meta |
|---|---|
| Imágenes borrosas rechazadas antes de API | 100% de las que están bajo el umbral |
| Reducción de tokens por imagen tras pre-procesamiento | ≥ 30% |
| Precisión en nombre del paciente (imagen nítida) | ≥ 95% |
| Precisión en medicamentos | ≥ 90% |
| Tiempo de pre-procesamiento por imagen | < 500ms |
| Tiempo total por imagen incluyendo API | < 10 segundos |

---

## 13. Hitos del proyecto

| Hito | Entregable | Estimado |
|---|---|---|
| M1 | `scan` end-to-end con Gemini gratis (sin pre-procesamiento aún) | Semana 1 |
| M2 | Pre-procesamiento local (escala de grises, contraste, resize) | Semana 2 |
| M3 | Detección de borrosidad + rechazo pre-API | Semana 2 |
| M4 | `query` + `patient list` + `export` | Semana 3 |
| M5 | Soporte dual Gemini/Anthropic intercambiable por env var | Semana 3 |
| M6 | Retry con backoff + resumen de errores al final del scan | Semana 4 |
| M7 | Tests de integración + binario multiplataforma | Semana 5 |