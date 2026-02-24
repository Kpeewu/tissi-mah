package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
)

type UserRepositoryWrite interface {
	Create(ctx context.Context, user *domain.User) (string, error)
	Update(ctx context.Context, user *domain.User) (*domain.User, error)
	Delete(ctx context.Context, userID string) error
}
