package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
)

type AuthRepositoryWrite interface {

	// add new user auth credentials
	Create(ctx context.Context, auth *domain.Auth) (string, error)

	// update user auth informations
	Update(ctx context.Context, auth *domain.Auth) (*domain.Auth, error)

	// delete user auth informations
	Delete(ctx context.Context, auth *domain.Auth) error
}
