package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
)

type SupportUserWriteRepository interface {
	Create(ctx context.Context, u *domain.SupportUser) error
	UpdatePassword(ctx context.Context, userID, hash string, mustChange bool) error
	UpdateEmail(ctx context.Context, userID, newEmail string) error
	Deactivate(ctx context.Context, userID string) error
	Activate(ctx context.Context, userID string) error
	// SoftDelete marque le compte comme supprimé (deleted_at = NOW()).
	SoftDelete(ctx context.Context, userID string) error
}
