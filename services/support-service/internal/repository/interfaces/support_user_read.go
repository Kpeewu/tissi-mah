package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
)

type SupportUserReadRepository interface {
	GetByID(ctx context.Context, userID string) (*domain.SupportUser, error)
	GetByEmail(ctx context.Context, email string) (*domain.SupportUser, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	List(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error)
	ListPendingPasswordResets(ctx context.Context) ([]*domain.SupportUser, error)
	ListAdminEmails(ctx context.Context) ([]string, error)
}
