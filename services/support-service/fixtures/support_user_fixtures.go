package fixtures

import (
	"context"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/password"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultPasswordPlain est un mot de passe par défaut conforme à la policy (≥12 chars + 4 classes).
const DefaultPasswordPlain = "Passw0rd!Test"

// DefaultPasswordHash est un hash argon2id pré-calculé de DefaultPasswordPlain.
// Usage : éviter le coût argon2 dans les tests qui ne testent pas le hashing.
// Pour un hash frais, utiliser BuildPasswordHash().
var DefaultPasswordHash string

func init() {
	h, err := password.Hash(DefaultPasswordPlain)
	if err != nil {
		panic(fmt.Sprintf("fixtures: cannot build default password hash: %v", err))
	}
	DefaultPasswordHash = h
}

// BuildPasswordHash génère un hash argon2id pour un mot de passe donné (tests qui vérifient le hashing).
func BuildPasswordHash(plain string) string {
	h, err := password.Hash(plain)
	if err != nil {
		panic(fmt.Sprintf("fixtures: BuildPasswordHash failed: %v", err))
	}
	return h
}

type SupportUserOption func(*domain.SupportUser)

func WithUserID(id string) SupportUserOption {
	return func(u *domain.SupportUser) { u.UserID = id }
}

func WithEmail(email string) SupportUserOption {
	return func(u *domain.SupportUser) { u.Email = email }
}

func WithPasswordHash(hash string) SupportUserOption {
	return func(u *domain.SupportUser) { u.PasswordHash = hash }
}

func WithFirstName(name string) SupportUserOption {
	return func(u *domain.SupportUser) { u.FirstName = name }
}

func WithLastName(name string) SupportUserOption {
	return func(u *domain.SupportUser) { u.LastName = name }
}

func WithRole(role string) SupportUserOption {
	return func(u *domain.SupportUser) { u.Role = role }
}

func WithAdminRole() SupportUserOption {
	return WithRole(domain.RoleAdmin)
}

func WithSupportRole() SupportUserOption {
	return WithRole(domain.RoleSupport)
}

func WithActive(active bool) SupportUserOption {
	return func(u *domain.SupportUser) { u.IsActive = active }
}

func WithInactive() SupportUserOption {
	return WithActive(false)
}

func WithMustChangePassword(must bool) SupportUserOption {
	return func(u *domain.SupportUser) { u.MustChangePassword = must }
}

func WithEmailChangedAt(t time.Time) SupportUserOption {
	return func(u *domain.SupportUser) { u.EmailChangedAt = &t }
}

func WithNoEmailChangedAt() SupportUserOption {
	return func(u *domain.SupportUser) { u.EmailChangedAt = nil }
}

func WithPasswordChangedAt(t time.Time) SupportUserOption {
	return func(u *domain.SupportUser) { u.PasswordChangedAt = t }
}

func WithCreatedAt(t time.Time) SupportUserOption {
	return func(u *domain.SupportUser) { u.CreatedAt = t }
}

func WithDeleted() SupportUserOption {
	return func(u *domain.SupportUser) {
		now := time.Now().UTC().Truncate(time.Microsecond)
		u.DeletedAt = &now
		u.IsActive = false
	}
}

func WithDeletedAt(t time.Time) SupportUserOption {
	return func(u *domain.SupportUser) {
		u.DeletedAt = &t
		u.IsActive = false
	}
}

// NewTestSupportUser crée un SupportUser avec des valeurs par défaut valides :
// role=support, actif, pas de mustChange, PasswordHash pointant vers DefaultPasswordPlain.
// Les timestamps sont placés 1s dans le passé et tronqués à la microseconde pour un round-trip DB stable.
func NewTestSupportUser(opts ...SupportUserOption) *domain.SupportUser {
	now := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	u := &domain.SupportUser{
		UserID:             uuid.NewString(),
		Email:              fmt.Sprintf("support-%s@tissimah.local", uuid.NewString()[:8]),
		PasswordHash:       DefaultPasswordHash,
		FirstName:          "Test",
		LastName:           "Support",
		Role:               domain.RoleSupport,
		IsActive:           true,
		MustChangePassword: false,
		PasswordChangedAt:  now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// NewTestAdmin est un raccourci : admin actif, sans mustChange.
func NewTestAdmin(opts ...SupportUserOption) *domain.SupportUser {
	all := append([]SupportUserOption{WithAdminRole()}, opts...)
	return NewTestSupportUser(all...)
}

// InsertSupportUser insère un SupportUser dans la base de test.
func InsertSupportUser(ctx context.Context, pool *pgxpool.Pool, u *domain.SupportUser) error {
	query := `
		INSERT INTO support_users (
			user_id, email, password_hash, first_name, last_name, role,
			is_active, must_change_password, email_changed_at, password_changed_at,
			created_at, updated_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := pool.Exec(ctx, query,
		u.UserID, u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Role,
		u.IsActive, u.MustChangePassword, u.EmailChangedAt, u.PasswordChangedAt,
		u.CreatedAt, u.UpdatedAt, u.DeletedAt,
	)
	return err
}
