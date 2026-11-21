package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
)

type AuthService interface {
	GetUserByFirebaseID(ctx context.Context, firebaseID string) (*domain.Auth, error)
	RegisterUser(ctx context.Context, name string, firstName string, email string, phoneNumber string, profilePhotoURL string, birthDate string) (*domain.UserPreview, error)
	CheckEmail(ctx context.Context, email string) (bool, error)
	CheckPhoneNumber(ctx context.Context, phoneNumber string) (bool, error)
	DeleteUserAccount(ctx context.Context, firebaseID string) error
	GetAuthInfo(ctx context.Context, authID string) (*domain.Auth, error)
	// SuspendAccount suspend ou bannit un compte. suspendedUntil=nil + isBanned=true = ban permanent.
	SuspendAccount(ctx context.Context, authID string, suspendedUntil *time.Time, isBanned bool) error
}
