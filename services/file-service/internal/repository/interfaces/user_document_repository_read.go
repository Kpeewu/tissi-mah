package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type UserDocumentRepositoryRead interface {
	// Récupère un document par son ID
	GetByID(ctx context.Context, documentID string) (*domain.UserDocument, error)

	// Récupère tous les documents d'un utilisateur
	GetByUserID(ctx context.Context, userID string) ([]*domain.UserDocument, error)

	// Récupère le document courant d'un utilisateur par type
	GetCurrentByUserIDAndType(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error)
}
