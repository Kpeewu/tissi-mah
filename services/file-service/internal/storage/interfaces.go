package storage

import (
	"context"
	"io"
)

// StorageClient définit l'interface pour les opérations de stockage de fichiers.
// Compatible S3 et MinIO via aws-sdk-go-v2.
type StorageClient interface {
	// Upload envoie un fichier vers le stockage et retourne l'URL publique
	Upload(ctx context.Context, key string, data io.Reader, contentType string, size int64) (string, error)

	// Delete supprime un fichier du stockage
	Delete(ctx context.Context, key string) error

	// GenerateURL retourne l'URL publique d'un fichier
	GenerateURL(key string) string
}
