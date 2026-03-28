# 🏥 medscan CLI

`medscan` es una utilidad de línea de comandos (CLI) en Go para digitalizar expedientes médicos físicos (papel) y convertirlos al instante en un formato estructurado JSON. Utiliza Modelos de Lenguaje (LLMs) con capacidad de visión y persistencia local en SQLite.

![medscan banner](https://img.shields.io/badge/go-1.22+-00ADD8.svg?logo=go)
![sqlite](https://img.shields.io/badge/sqlite-local-blue.svg)
![AI Vision](https://img.shields.io/badge/AI_Vision-Gemini%20%7C%20Claude-orange.svg)

---

## 🔥 Características

- **Visión por IA avanzado:** Digitaliza texto médico (incluyendo letra difícil), diagnósticos y recetas utilizando Google Gemini 2.5 Flash o Anthropic Claude.
- **Configuración rápida (`setup`):** Configuración automática interactiva para no lidiar con archivos ni rutas.
- **Pre-procesamiento dinámico local:** Mejora del contraste, conversión a escala de grises y recorte dinámico de las imágenes para optimizar la lectura y *ahorrar tokens*.
- **Detección de fotos borrosas:** Calcula la varianza del Laplaciano **antes** de gastar llamadas a la API. Las fotos ilegibles se rechazan instantáneamente.
- **Base de datos local:** Almacena todos los expedientes, visitas e historial en una base de datos local SQLite (`mediscan.db`). No necesitas infra de base de datos externa.
- **Deduplicación:** Evita procesar dos veces el mismo archivo comparando el SHA-256.

---

## 🚀 Instalación y Uso Rápido (Para tu equipo)

### 1. Descarga el binario

Para un despliegue directo sin instalar Go, descarga el binario desde la pestaña de **Releases** y dale permisos de ejecución.

O, si tienes **Go** instalado, compila el código fuente en 1 segundo:

```bash
git clone https://github.com/AndreRaz/medscan-CLI.git
cd medscan-CLI
make build
```

### 2. Configura tu API Key (Automático)

En lugar de crear archivos a mano, `medscan` incluye un asistente interactivo:

```bash
./medscan setup
```
El asistente te guiará para:
1. Crear gratis una API Key en [aistudio.google.com](https://aistudio.google.com/) en 3 clicks sin tarjeta.
2. Definir dónde ubicar la base de datos (por defecto `./mediscan.db`).
3. Crear tu archivo de entorno listo para usarse.

### 3. ¡Empieza a Escanear!

Coloca las fotos (.jpg, .png) de los documentos en una carpeta y envíala a escanear:

```bash
./medscan scan ./docs/
```

### 4. Consultar pacientes o historiales

```bash
# Consultar pacientes registrados:
./medscan patient list

# Mostrar TODO el historial y visitas médicas de un paciente por su CURP:
./medscan query GACM850101HMCRLS09

# Buscar a un paciente por nombre (y obtener su ID si no tiene CURP):
./medscan query --nombre "Filomena"

# Exportar un expediente JSON a tu disco (soporta --id):
./medscan export --id 2 -o paciente.json
```

---

## 🗄️ Visor de Base de Datos Local

`medscan` también integra comandos para diagnosticar todo el sistema local:

```bash
./medscan db stats       # Estadísticas (número de expedientes, tamaño bytes, items fallidos)
./medscan db visitas     # Muestra las visitas médicas recientes estilo tabla de DB
./medscan db rechazados  # ¿Por qué se rechazó un archivo? Aquí salen las fotos borrosas
```

---

## 🛠 Variables de entorno avanzadas / Configuración Manual

Si prefieres usar CI/CD o no usar `setup`, puedes crear un archivo `.env` basado en `.env.example`:

| Variable | Descripción | Default |
|----------|-------------|---------|
| `MEDISCAN_PROVIDER` | Proveedor: `gemini` (gratis y rápido) o `anthropic` | `anthropic` |
| `GEMINI_API_KEY` | Tu llave para Google AI Studio | - |
| `MEDISCAN_DB_PATH` | Ruta absoluta/relativa al archivo de SQLite | `./mediscan.db` |
| `MEDISCAN_BLUR_THRESHOLD` | Sensibilidad a borrosidad. Con buena luz pon `300` | `100.0` |
| `MEDISCAN_MAX_WIDTH` | Ancho de preprocesamiento, 1200px rinde excelente | `1200` |

---

## 📄 Arquitectura & ADRs

El diseño técnico ha sido documentado en el PRD del sistema.
Destacan:
- **`disintegration/imaging`**: Transformaciones y pre-procesamiento sin OpenCV.
- **Laplaciano en Go Puro`: Para evitar dependencias nativas engorrosas de instalar en consultorios.
- SQLite (`mattn/go-sqlite3`): Embebido localmente. Limitado a consultas Single-user pero muy distribuible.
