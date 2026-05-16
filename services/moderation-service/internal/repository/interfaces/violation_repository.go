package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
)

type ViolationRepository interface {
	// CountByUserAndType retourne le nombre total de violations pour cet utilisateur et ce type de contenu.
	CountByUserAndType(ctx context.Context, userID, contentType string) (int, error)
	// Create enregistre une nouvelle violation.
	Create(ctx context.Context, v *domain.UserViolation) error
}
