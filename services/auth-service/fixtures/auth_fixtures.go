package fixtures

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthOption func(*domain.Auth)

func WithEmail(email string) AuthOption {
	return func(a *domain.Auth) {
		a.Email = &email
	}
}

func WithPhoneNumber(phoneNumber string) AuthOption {
	return func(a *domain.Auth) {
		a.PhoneNumber = &phoneNumber
	}
}

// WithAuthID personnalise l'AuthID
func WithAuthID(authID string) AuthOption {
	return func(a *domain.Auth) {
		a.AuthID = authID
	}
}

// WithFirebaseID personnalise le FirebaseID
func WithFirebaseID(firebaseID string) AuthOption {
	return func(a *domain.Auth) {
		a.FirebaseID = firebaseID
	}
}

// WithNoEmail met l'email à nil
func WithNoEmail() AuthOption {
	return func(a *domain.Auth) {
		a.Email = nil
	}
}

// WithNoPhoneNumber met le téléphone à nil
func WithNoPhoneNumber() AuthOption {
	return func(a *domain.Auth) {
		a.PhoneNumber = nil
	}
}

// WithInactive marque le compte comme inactif
func WithInactive() AuthOption {
	return func(a *domain.Auth) {
		a.IsActive = false
	}
}

// WithSuspended marque le compte comme suspendu
func WithSuspended(endDate time.Time) AuthOption {
	return func(a *domain.Auth) {
		a.IsSuspended = true
		a.SuspensionEndDate = &endDate
	}
}

// WithDeleted marque le compte comme supprimé
func WithDeleted() AuthOption {
	return func(a *domain.Auth) {
		now := time.Now().UTC()
		a.DeletedAt = &now
	}
}

func NewTestAuth(opts ...AuthOption) *domain.Auth {
	now := time.Now().UTC()
	id := uuid.New().String()

	// Valeurs par défaut
	defaultEmail := "test_" + id[:8] + "@example.com"
	defaultPhone := "+228" + id[:8]

	auth := &domain.Auth{
		AuthID:            "auth_" + id,
		FirebaseID:        "firebase_" + id,
		Email:             &defaultEmail,
		PhoneNumber:       &defaultPhone,
		IsActive:          true,
		IsSuspended:       false,
		SuspensionEndDate: nil,
		CreatedAt:         now,
		UpdatedAt:         now,
		DeletedAt:         nil,
	}

	// Appliquer les options personnalisées
	for _, opt := range opts {
		opt(auth)
	}

	return auth
}

// InsertAuth insère un Auth dans la base de données de test
func InsertAuth(ctx context.Context, pool *pgxpool.Pool, auth *domain.Auth) error {
	query := `
		INSERT INTO auth (
			auth_id, firebase_id, email, phone_number,
			is_active, is_suspended, suspension_end_date,
			created_at, updated_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := pool.Exec(ctx, query,
		auth.AuthID,
		auth.FirebaseID,
		auth.Email,
		auth.PhoneNumber,
		auth.IsActive,
		auth.IsSuspended,
		auth.SuspensionEndDate,
		auth.CreatedAt,
		auth.UpdatedAt,
		auth.DeletedAt,
	)

	return err
}
