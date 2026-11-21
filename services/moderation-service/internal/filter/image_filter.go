package filter

import (
	"fmt"
)

const maxImageSize = 10 * 1024 * 1024 // 10 Mo

// magicBytes associe les premiers octets d'un fichier à son type MIME.
var magicBytes = map[string][]byte{
	"image/jpeg": {0xFF, 0xD8, 0xFF},
	"image/png":  {0x89, 0x50, 0x4E, 0x47},
	"image/gif":  {0x47, 0x49, 0x46, 0x38},
	"image/webp": {0x52, 0x49, 0x46, 0x46}, // RIFF header
	"image/bmp":  {0x42, 0x4D},
}

// allowedMimeTypes liste les types MIME acceptés pour une photo de profil.
var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
	"image/heic": true,
	"image/heif": true,
}

// ImageFilterResult est le résultat de la validation locale d'une image.
type ImageFilterResult struct {
	Valid  bool
	Reason string
}

// ValidateImage effectue les vérifications locales sur une image avant l'appel API.
func ValidateImage(data []byte, mimeType string) ImageFilterResult {
	if len(data) == 0 {
		return ImageFilterResult{false, "empty image data"}
	}

	if len(data) > maxImageSize {
		return ImageFilterResult{false, fmt.Sprintf("image too large: %d bytes (max %d)", len(data), maxImageSize)}
	}

	if !allowedMimeTypes[mimeType] {
		return ImageFilterResult{false, fmt.Sprintf("unsupported mime type: %s", mimeType)}
	}

	// Vérification des magic bytes (ignorer HEIC/HEIF qui ont une structure plus complexe)
	if mimeType != "image/heic" && mimeType != "image/heif" {
		if !hasValidMagicBytes(data, mimeType) {
			// Essayer de détecter le type réel et voir si c'est quand même une image valide
			if !isAnyKnownImage(data) {
				return ImageFilterResult{false, "invalid image format: magic bytes mismatch"}
			}
		}
	}

	return ImageFilterResult{Valid: true}
}

func hasValidMagicBytes(data []byte, mimeType string) bool {
	// Normaliser image/jpg → image/jpeg
	if mimeType == "image/jpg" {
		mimeType = "image/jpeg"
	}

	magic, ok := magicBytes[mimeType]
	if !ok {
		return true // Pas de vérification pour les types sans magic bytes connus
	}
	if len(data) < len(magic) {
		return false
	}
	for i, b := range magic {
		if data[i] != b {
			return false
		}
	}
	return true
}

func isAnyKnownImage(data []byte) bool {
	for _, magic := range magicBytes {
		if len(data) >= len(magic) {
			match := true
			for i, b := range magic {
				if data[i] != b {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}
