# MEDSCAN

`medscan` es una utilidad de línea de comandos (CLI) en Go para digitalizar expedientes médicos físicos (papel) y convertirlos al instante en un formato estructurado JSON. Utiliza Modelos de Lenguaje (LLMs) con capacidad de visión y persistencia local en SQLite.

![medscan banner](https://img.shields.io/badge/go-1.22+-00ADD8.svg?logo=go)
![sqlite](https://img.shields.io/badge/sqlite-local-blue.svg)
![AI Vision](https://img.shields.io/badge/AI_Vision-Gemini%20%7C%20Claude-orange.svg)

---

<div align="center">
  <img src="assets/tui_screenshot.png" alt="medscan TUI Interface" width="700">
  <br />
  <p><i>Interfaz gráfica de Terminal (TUI) incorporada: Exploración interactiva y digitalización en vivo</i></p>
</div>

---

## Características Principales

- **Visión por IA avanzada:** Digitaliza texto médico (incluyendo letra difícil), diagnósticos y recetas utilizando Google Gemini 2.5 Flash o Anthropic Claude.
- **Configuración rápida (`setup`):** Configuración automática e interactiva para evitar la configuración manual de archivos y rutas.
- **Pre-procesamiento dinámico local:** Mejora del contraste, conversión a escala de grises y recorte dinámico de las imágenes para optimizar la lectura y reducir el consumo de tokens.
- **Detección de imágenes borrosas:** Calcula la varianza del Laplaciano antes de consumir cuota de la API. Las fotografías ilegibles se rechazan de manera instantánea.
- **Base de datos local:** Almacena todos los expedientes, visitas e historiales en una base de datos local SQLite (`mediscan.db`), eliminando la necesidad de infraestructura de base de datos externa.
- **Deduplicación de archivos:** Evita el procesamiento duplicado del mismo archivo mediante la validación de su firma SHA-256.

---

## Instalación y Uso

### 1. Obtener el binario

Para un despliegue directo sin instalar el entorno de Go, se recomienda descargar el binario desde la sección de **Releases** en GitHub y asignarle permisos de ejecución.

Alternativamente, si el entorno de Go está configurado, la compilación desde el código fuente se realiza ejecutando los siguientes comandos:

```bash
git clone https://github.com/AndreRaz/medscan-CLI.git
cd medscan-CLI
make build
```

### 2. Configurar la API Key

El sistema incluye un asistente interactivo para simplificar el proceso de configuración inicial:

```bash
./medscan setup
```
El asistente presentará los siguientes pasos:
1. Instrucciones para generar una API Key en [aistudio.google.com](https://aistudio.google.com/) de forma gratuita.
2. Definición de la ruta para el archivo de la base de datos (por defecto `./mediscan.db`).
3. Generación automática del archivo de entorno local `.env`.

### 3. Ejecutar el escáner de documentos

Colocar las imágenes de los documentos médicos (.jpg, .png) en un directorio específico y ejecutar el escaneo:

```bash
./medscan scan ./docs/
```

### 4. Consultar expedientes y registros

Los datos guardados en la base local pueden ser consultados mediante la CLI:

```bash
# Listar todos los pacientes registrados:
./medscan patient list

# Mostrar el historial completo y visitas médicas de un paciente mediante su CURP:
./medscan query GACM850101HMCRLS09

# Buscar un paciente por su nombre (útil para obtener el ID de consulta si no tiene CURP):
./medscan query --nombre "Filomena"

# Exportar un expediente JSON a disco (soporta el uso del identificador numérico interno --id):
./medscan export --id 2 -o paciente.json
```

---

## Visor de Base de Datos Local

`medscan` integra herramientas para diagnosticar el estado del almacenamiento local:

```bash
./medscan db stats       # Estadísticas generales (volumen de pacientes, tamaño del archivo, errores)
./medscan db visitas     # Muestra una tabla con el registro de las visitas médicas recientes
./medscan db rechazados  # Lista los archivos descartados durante el escaneo y el motivo (ej. borrosidad)
```

---

## Configuración Manual y Variables de Entorno

Para uso en integraciones continuas (CI/CD) o despliegues automatizados, es posible crear un archivo `.env` manual usando `.env.example` como plantilla:

| Variable | Descripción | Valor por defecto |
|----------|-------------|---------|
| `MEDISCAN_PROVIDER` | Proveedor seleccionado: `gemini` o `anthropic` | `anthropic` |
| `GEMINI_API_KEY` | Llave de autenticación para Google AI Studio | - |
| `MEDISCAN_DB_PATH` | Ruta (absoluta o relativa) al archivo SQLite | `./mediscan.db` |
| `MEDISCAN_BLUR_THRESHOLD` | Límite de varianza aceptable para considerar la imagen nítida | `100.0` |
| `MEDISCAN_MAX_WIDTH` | Ancho máximo de preprocesamiento | `1200` |

---

## Arquitectura y Decisiones de Diseño

Los detalles técnicos completos se encuentran en la documentación del sistema (PRD).
Los aspectos fundamentales de la arquitectura incluyen:
- Uso de **`disintegration/imaging`** para realizar transformaciones de imagen estándar y preprocesamiento sin recurrir a dependencias complejas como OpenCV.
- Implementación de **Laplaciano en Go Puro** para garantizar una detección eficiente de enfoque directamente desde la librería estándar.
- Adopción de **SQLite (`mattn/go-sqlite3`)** embebido localmente, lo cual facilita el despliegue de la aplicación sin perder capacidades relacionales.
