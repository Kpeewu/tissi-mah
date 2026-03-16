package client

import "context"

// AuthClient définit les opérations inter-service vers auth-service.
// Implémenté par AuthServiceClient en production, mockable pour les tests.
type AuthClient interface {
	GetAuthInfo(ctx context.Context, authID string) (*AuthInfo, error)
	Close() error
}

// FileClient définit les opérations inter-service vers file-service.
type FileClient interface {
	// GetDocumentExpiry retourne expired_at du document courant, "" si non trouvé ou erreur (non-bloquant).
	GetDocumentExpiry(ctx context.Context, userID, documentType string) string
	// UploadProfilePicture uploade l'image sur S3 via file-service et retourne l'URL (bloquant).
	UploadProfilePicture(ctx context.Context, userID string, imageBytes []byte) (string, error)
	Close() error
}
