package scanner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SupportedExtensions define los formatos de imagen aceptados.
var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".tiff": true,
	".tif":  true,
}

// MaxFileSizeBytes es el tamaño máximo de archivo permitido (20MB).
const MaxFileSizeBytes = 20 * 1024 * 1024

// FileInfo contiene la información de un archivo listo para procesar.
type FileInfo struct {
	Path string
	Hash string
	Size int64
}

// ValidationError representa un error de validación de archivo.
type ValidationError struct {
	Path   string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Reason)
}

// ScanFolder recorre una carpeta y devuelve archivos de imagen válidos.
// Los archivos con formato no soportado o > 20MB se devuelven en rejected.
func ScanFolder(folder string) (files []FileInfo, rejected []ValidationError, err error) {
	err = filepath.Walk(folder, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Validar formato
		if !SupportedExtensions[ext] {
			rejected = append(rejected, ValidationError{Path: path, Reason: "formato no soportado (" + ext + ")"})
			return nil
		}

		// Validar tamaño
		if info.Size() > MaxFileSizeBytes {
			rejected = append(rejected, ValidationError{
				Path:   path,
				Reason: fmt.Sprintf("archivo demasiado grande (%.1f MB, máx 20 MB)", float64(info.Size())/1024/1024),
			})
			return nil
		}

		// Calcular hash
		hash, err := HashFile(path)
		if err != nil {
			rejected = append(rejected, ValidationError{Path: path, Reason: "no se pudo leer el archivo: " + err.Error()})
			return nil
		}

		files = append(files, FileInfo{
			Path: path,
			Hash: hash,
			Size: info.Size(),
		})
		return nil
	})
	return
}

// HashFile calcula el SHA-256 de un archivo y devuelve el hex string.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
