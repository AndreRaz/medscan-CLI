<div align="center">

```
  ███╗   ███╗███████╗██████╗ ███████╗ ██████╗ █████╗ ███╗  ██╗
  ████╗ ████║██╔════╝██╔══██╗██╔════╝██╔════╝██╔══██╗████╗ ██║
  ██╔████╔██║█████╗  ██║  ██║███████╗██║     ███████║██╔██╗██║
  ██║╚██╔╝██║██╔══╝  ██║  ██║╚════██║██║     ██╔══██║██║╚████║
  ██║ ╚═╝ ██║███████╗██████╔╝███████║╚██████╗██║  ██║██║ ╚███║
  ╚═╝     ╚═╝╚══════╝╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚══╝
```

**Digitalizador de expedientes médicos con visión por IA**

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)](https://github.com/AndreRaz/medscan-CLI/releases)
[![License](https://img.shields.io/badge/license-MIT-purple?style=flat-square)](LICENSE)
[![Releases](https://img.shields.io/github/v/release/AndreRaz/medscan-CLI?style=flat-square&color=teal)](https://github.com/AndreRaz/medscan-CLI/releases)

</div>

---

`medscan` es una herramienta CLI/TUI escrita en Go que digitaliza expedientes médicos en papel usando visión por IA. Apunta a una carpeta con fotos, y el sistema pre-procesa las imágenes localmente, detecta borrosidad antes de gastar tokens, transcribe el contenido a JSON estructurado y lo persiste en SQLite — todo desde la terminal.

<div align="center">
  <img src="assets/tui_screenshot.png" alt="medscan TUI — Panel de Control" width="860">
  <br/>
  <sub>Panel de Control Principal — dashboard con estadísticas en tiempo real y log de actividad</sub>
</div>

---

## Caracteristicas

| Feature | Descripcion |
|---|---|
| **TUI interactiva** | Dashboard con estadisticas, maestro-detalle de pacientes y log de escaneo en tiempo real |
| **Doble proveedor de IA** | Gemini 2.5 Flash (gratis, sin tarjeta) o Anthropic Claude (mayor precision) — intercambiables por env var |
| **Deteccion de borrosidad** | Varianza del Laplaciano implementada en Go — rechaza fotos malas ANTES de consumir cuota de API |
| **Pre-procesamiento local** | Escala de grises, contraste, resize a max 1200px — el LLM procesa texto limpio, menos tokens, mas precision |
| **SQLite embebido** | Un solo archivo binario. Sin servidores, sin instalaciones adicionales |
| **Deduplicacion SHA-256** | El mismo documento no se procesa dos veces aunque este en la misma carpeta |
| **Setup interactivo** | `medscan setup` configura todo en minutos — valida la API key antes de guardarla |
| **Cross-platform** | Binarios precompilados para Linux, macOS (Intel/ARM) y Windows |

---

## Instalacion rapida (binario precompilado)

No requiere instalar Go ni ningun compilador.

### 1. Descargar el ejecutable

Ve a la seccion **[Releases](https://github.com/AndreRaz/medscan-CLI/releases)** y descarga el binario correspondiente a tu sistema:

| Sistema | Archivo |
|---|---|
| Linux (Intel/AMD) | `medscan-linux-amd64` |
| Linux (ARM / Raspberry Pi) | `medscan-linux-arm64` |
| macOS (Apple Silicon M1/M2/M3) | `medscan-darwin-arm64` |
| macOS (Intel) | `medscan-darwin-amd64` |
| Windows (64 bits) | `medscan-windows-amd64.exe` |

### 2. Dar permisos de ejecucion (Linux y macOS)

```bash
# Renombrar para comodidad
mv medscan-linux-amd64 medscan        # Linux
mv medscan-darwin-arm64 medscan       # macOS Apple Silicon

# Dar permisos de ejecucion
chmod +x ./medscan
```

> **macOS**: si el sistema bloquea la ejecucion, ir a **Ajustes del Sistema > Privacidad y Seguridad** y autorizar la app manualmente.

### 3. Instalar globalmente (opcional pero recomendado)

Para ejecutar `medscan` desde cualquier directorio sin `./`:

```bash
# Linux y macOS — instala en ~/.local/bin sin necesitar permisos root
./medscan install

# Luego recarga tu shell
source ~/.bashrc   # o ~/.zshrc
```

En **Windows**, el comando `medscan install` imprime las instrucciones exactas para agregar el ejecutable al PATH del sistema.

### 4. Configuracion inicial

```bash
./medscan setup
```

El asistente interactivo configura:
- Proveedor de IA (Gemini gratuito o Anthropic)
- API Key (con validacion antes de guardar)
- Ruta de la base de datos
- Parametros de procesamiento de imagen

Al finalizar genera un archivo `.env` listo para usar.

---

## Uso

### Interfaz grafica (TUI) — recomendado

```bash
medscan
```

Lanza el panel de control interactivo. Navegacion con teclado o raton.

### Escanear una carpeta

```bash
medscan scan ./docs/expedientes/
```

Procesa todas las imagenes de la carpeta: validacion → deteccion de borrosidad → pre-procesamiento → transcripcion por IA → guardado en SQLite.

### Consultar pacientes

```bash
# Listar todos los pacientes registrados
medscan patient list

# Expediente completo de un paciente (por CURP)
medscan query CURP123456HDFXXX00

# Exportar expediente a JSON
medscan export CURP123456HDFXXX00
```

### Estadisticas de la base de datos

```bash
medscan db stats          # Resumen general
medscan db visitas        # Historial de visitas
medscan db rechazados     # Archivos rechazados con motivo y blur score
medscan db export ./backup/medscan.db        # Copia de seguridad
medscan db export ./backup/medscan.db --move # Mover la DB
```

---

## Obtener API Keys (gratuito)

### Google Gemini — sin tarjeta de credito

1. Ir a [https://aistudio.google.com](https://aistudio.google.com)
2. Clic en **Get API key** → crear proyecto → copiar key
3. Limites del tier gratuito: 10 requests/min, 250 requests/dia

### Anthropic Claude — mayor precision en letra medica dificil

1. Ir a [https://console.anthropic.com](https://console.anthropic.com)
2. **Settings → API Keys → Create Key**
3. Requiere cuenta con saldo

---

## Configuracion completa

Todas las opciones se configuran via variables de entorno en el archivo `.env`:

```bash
# Proveedor (default: anthropic)
MEDISCAN_PROVIDER=gemini              # gemini | anthropic

# API Keys
GEMINI_API_KEY=AIzaSy...
ANTHROPIC_API_KEY=sk-ant-...

# Modelos
MEDISCAN_GEMINI_MODEL=gemini-2.5-flash
MEDISCAN_ANTHROPIC_MODEL=claude-opus-4-5

# Base de datos
MEDISCAN_DB_PATH=./mediscan.db

# Procesamiento de imagen
MEDISCAN_MAX_WIDTH=1200               # px maximo (reduce tokens)
MEDISCAN_CONTRAST=1.3                 # multiplicador de contraste
MEDISCAN_BLUR_THRESHOLD=100.0         # varianza minima (0 = desactivado)

# Logging
MEDISCAN_LOG_LEVEL=info               # debug | info | warn | error
```

### Referencia de blur score

| Condicion de la foto | Score tipico | Accion |
|---|---|---|
| Nitida, buena iluminacion | 200 – 800 | Procesada normalmente |
| Ligeramente movida | 80 – 150 | Procesada con advertencia |
| Muy borrosa | < 80 | Rechazada — no consume API |
| Completamente fuera de foco | < 20 | Rechazada con error claro |

---

## Pipeline de procesamiento

Cada imagen pasa por las siguientes etapas antes de llegar a la IA:

```
Foto original (celular del medico)
        │
        ▼
  [1] Validacion — formato y tamano (max 20MB)
        │
        ▼
  [2] Deteccion de borrosidad (Varianza del Laplaciano)
        │  score < umbral → rechazada, sin llamada a API
        ▼
  [3] Pre-procesamiento local
        │  · Escala de grises
        │  · Ajuste de contraste
        │  · Resize a max 1200px de ancho
        ▼
  [4] Codificacion base64 → envio a API
        │
        ▼
  [5] Parseo JSON → SQLite
```

---

## Compilar desde el codigo fuente

Requiere Go 1.22 o superior.

```bash
git clone https://github.com/AndreRaz/medscan-CLI.git
cd medscan-CLI

# Compilar para la plataforma actual
make build

# Compilar para todas las plataformas (Linux, macOS, Windows)
make build-all

# Instalar globalmente en ~/.local/bin
make install
```

Los binarios multiplataforma quedan en `./dist/`.

---

## Stack tecnologico

| Componente | Tecnologia |
|---|---|
| Lenguaje | Go 1.22+ |
| TUI | [tview](https://github.com/rivo/tview) + [tcell](https://github.com/gdamore/tcell) |
| Base de datos | SQLite via [go-sqlite3](https://github.com/mattn/go-sqlite3) |
| CLI | [Cobra](https://github.com/spf13/cobra) |
| Procesamiento de imagen | [imaging](https://github.com/disintegration/imaging) |
| Proveedor IA (default) | Google Gemini 2.5 Flash |
| Proveedor IA (precision) | Anthropic Claude Opus |

---

## Licencia

MIT — libre para uso personal y comercial.
