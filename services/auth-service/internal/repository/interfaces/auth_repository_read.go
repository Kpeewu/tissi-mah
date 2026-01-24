package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
)

type AuthRepositoryRead interface {
	// get user by his firebase id
	GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error)

	// get user by his auth id
	GetByAuthID(ctx context.Context, firebaseID string) (*domain.Auth, error)

	// get user by his email
	GetByEmail(ctx context.Context, email string) (*domain.Auth, error)

	// get user by his phone number
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*domain.Auth, error)

	// check if email already used
	EmailExists(ctx context.Context, email string) (bool, error)

	// check if phone number already used
	PhoneNumberExists(ctx context.Context, email string) (bool, error)
}
