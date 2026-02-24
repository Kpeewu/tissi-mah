package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
)

type UserRepositoryRead interface {
	GetByUserID(ctx context.Context, userID string) (*domain.User, error)
	GetByAuthID(ctx context.Context, authID string) (*domain.User, error)
	GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error)
}
