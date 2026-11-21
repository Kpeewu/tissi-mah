package storage

import (
	"context"
	"io"
	"time"
)

// StorageClient définit l'interface pour les opérations de stockage de fichiers.
// Compatible S3 et MinIO via aws-sdk-go-v2.
type StorageClient interface {
	// Upload envoie un fichier vers le stockage et retourne l'URL publique
	Upload(ctx context.Context, key string, data io.Reader, contentType string, size int64) (string, error)

	// Delete supprime un fichier du stockage
	Delete(ctx context.Context, key string) error

	// GeneratePresignedURL génère une URL signée à durée de vie limitée pour accéder à un fichier privé.
	// Compatible S3 (virtual-hosted style) et MinIO (path style).
	GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}
