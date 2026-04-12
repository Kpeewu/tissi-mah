package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
)

// LoginResult contient le sessionID OTP et son TTL.
type LoginResult struct {
	OTPSessionID     string
	ExpiresInSeconds int
}

// VerifyOTPResult contient les tokens émis après validation OTP.
type VerifyOTPResult struct {
	AccessToken        string
	AccessExpiresAt    int64
	RefreshToken       string
	RefreshExpiresAt   int64
	Role               string
	MustChangePassword bool
}

// RefreshResult contient les nouveaux tokens issus d'une rotation.
type RefreshResult struct {
	AccessToken      string
	AccessExpiresAt  int64
	RefreshToken     string
	RefreshExpiresAt int64
}

// SupportService est la façade métier du support-service.
type SupportService interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	VerifyOTP(ctx context.Context, sessionID, code string) (*VerifyOTPResult, error)
	ResendOTP(ctx context.Context, sessionID string) (*LoginResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*RefreshResult, error)
	Logout(ctx context.Context, refreshToken string) error

	Me(ctx context.Context, userID string) (*domain.SupportUser, error)
	ChangeMyPassword(ctx context.Context, userID, currentPassword, newPassword string) error
	ChangeMyEmail(ctx context.Context, userID, newEmail, currentPassword string) error

	CreateSupportAgent(ctx context.Context, email, firstName, lastName string) (string, error)
	ListSupportAgents(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error)
	DeactivateSupportAgent(ctx context.Context, userID string) error
}
