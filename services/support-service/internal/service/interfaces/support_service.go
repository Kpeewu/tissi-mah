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
	UpdateMyProfile(ctx context.Context, userID, firstName, lastName string) error

	// ForgotPassword (public) : un agent support déclenche une demande (notifiée aux
	// admins) ; un admin reçoit directement un lien de réinitialisation par email.
	ForgotPassword(ctx context.Context, email string) error
	// ResetPassword (public) : applique un nouveau mot de passe via un token de reset.
	ResetPassword(ctx context.Context, token, newPassword string) error
	// ListPasswordResetRequests (admin) : demandes de reset en attente (dashboard).
	ListPasswordResetRequests(ctx context.Context) ([]*domain.SupportUser, error)
	// TriggerPasswordReset (admin) : envoie un lien de reset à l'agent ciblé.
	TriggerPasswordReset(ctx context.Context, userID string) error

	CreateSupportAgent(ctx context.Context, email, firstName, lastName, role string) (string, error)
	ListSupportAgents(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error)
	DeactivateSupportAgent(ctx context.Context, userID string) error
	ActivateSupportAgent(ctx context.Context, userID string) error
	DeleteSupportAgent(ctx context.Context, userID string) error
	// UpdateSupportAgent (admin) modifie l'email et/ou le rôle d'un agent.
	// Un champ vide signifie « inchangé ».
	UpdateSupportAgent(ctx context.Context, userID, newEmail, newRole string) error
}
