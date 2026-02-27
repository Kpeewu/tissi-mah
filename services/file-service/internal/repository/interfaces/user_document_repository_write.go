package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
)

type UserDocumentRepositoryWrite interface {
	// Crée un nouveau document utilisateur
	Create(ctx context.Context, doc *domain.UserDocument) (string, error)

	// Met à jour un document utilisateur
	Update(ctx context.Context, doc *domain.UserDocument) (*domain.UserDocument, error)

	// Supprime un document utilisateur
	Delete(ctx context.Context, documentID string) error

	// Marque un document comme remplacé (is_current = false)
	MarkAsReplaced(ctx context.Context, documentID string, replacedBy string) error
}
