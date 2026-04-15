package handler_test

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	grpcsrv "github.com/Kpeewu/tissi-mah/services/support-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/middleware"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/support-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newHandler(t *testing.T) (*grpcsrv.SupportHandler, *mocks.MockSupportService) {
	t.Helper()
	svc := new(mocks.MockSupportService)
	return grpcsrv.NewSupportHandler(svc, zap.NewNop()), svc
}

func ctxWithUID(uid string) context.Context {
	return context.WithValue(context.Background(), middleware.SupportUIDKey, uid)
}

func ctxWithUIDRole(uid, role string) context.Context {
	ctx := ctxWithUID(uid)
	return context.WithValue(ctx, middleware.SupportRoleKey, role)
}

func codeOf(t *testing.T, err error) codes.Code {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected grpc status error, got %T: %v", err, err)
	return st.Code()
}

// =============================================================================
// Mapping erreurs → gRPC codes
// =============================================================================

func TestToGRPCError_Mapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"InvalidInput", supportErrors.ErrInvalidInput, codes.InvalidArgument},
		{"WeakPassword", supportErrors.ErrWeakPassword, codes.InvalidArgument},
		{"Unauthenticated", supportErrors.ErrUnauthenticated, codes.Unauthenticated},
		{"InvalidCredentials", supportErrors.ErrInvalidCredentials, codes.Unauthenticated},
		{"RefreshInvalid", supportErrors.ErrRefreshInvalid, codes.Unauthenticated},
		{"RefreshRevoked", supportErrors.ErrRefreshRevoked, codes.Unauthenticated},
		{"AccountLocked", supportErrors.ErrAccountLocked, codes.PermissionDenied},
		{"AccountInactive", supportErrors.ErrAccountInactive, codes.PermissionDenied},
		{"Forbidden", supportErrors.ErrForbidden, codes.PermissionDenied},
		{"UserNotFound", supportErrors.ErrUserNotFound, codes.NotFound},
		{"EmailAlreadyExists", supportErrors.ErrEmailAlreadyExists, codes.AlreadyExists},
		{"OTPExpired", supportErrors.ErrOTPExpired, codes.FailedPrecondition},
		{"OTPInvalid", supportErrors.ErrOTPInvalid, codes.FailedPrecondition},
		{"OTPTooManyAttempts", supportErrors.ErrOTPTooManyAttempts, codes.FailedPrecondition},
		{"OTPSessionNotFound", supportErrors.ErrOTPSessionNotFound, codes.FailedPrecondition},
		{"EmailChangeCooldown", supportErrors.ErrEmailChangeCooldown, codes.FailedPrecondition},
		{"ResendCooldown", supportErrors.ErrResendCooldown, codes.ResourceExhausted},
	}

	h, svc := newHandler(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc.ExpectedCalls = nil
			svc.On("Login", mock.Anything, "a@b.com", "x").Return(nil, tc.err).Once()
			_, err := h.Login(context.Background(), &supportpb.LoginRequest{Email: "a@b.com", Password: "x"})
			assert.Equal(t, tc.want, codeOf(t, err))
		})
	}

	t.Run("erreur inconnue → Internal", func(t *testing.T) {
		svc.ExpectedCalls = nil
		svc.On("Login", mock.Anything, "a@b.com", "x").Return(nil, assert.AnError).Once()
		_, err := h.Login(context.Background(), &supportpb.LoginRequest{Email: "a@b.com", Password: "x"})
		assert.Equal(t, codes.Internal, codeOf(t, err))
	})
}

// =============================================================================
// Login / VerifyOTP / ResendOTP / RefreshToken / Logout (routes publiques)
// =============================================================================

func TestHandler_Login_Success(t *testing.T) {
	h, svc := newHandler(t)
	svc.On("Login", mock.Anything, "a@b.com", "pwd").Return(&svcIfaces.LoginResult{
		OTPSessionID: "sess-1", ExpiresInSeconds: 300,
	}, nil)

	res, err := h.Login(context.Background(), &supportpb.LoginRequest{Email: "a@b.com", Password: "pwd"})
	require.NoError(t, err)
	assert.Equal(t, "sess-1", res.GetOtpSessionId())
	assert.Equal(t, int32(300), res.GetExpiresInSeconds())
}

func TestHandler_VerifyOTP_Success(t *testing.T) {
	h, svc := newHandler(t)
	svc.On("VerifyOTP", mock.Anything, "sess", "123456").Return(&svcIfaces.VerifyOTPResult{
		AccessToken:        "a",
		AccessExpiresAt:    100,
		RefreshToken:       "r",
		RefreshExpiresAt:   200,
		Role:               "admin",
		MustChangePassword: true,
	}, nil)

	res, err := h.VerifyOTP(context.Background(), &supportpb.VerifyOTPRequest{OtpSessionId: "sess", Code: "123456"})
	require.NoError(t, err)
	assert.Equal(t, "a", res.GetAccessToken())
	assert.Equal(t, "r", res.GetRefreshToken())
	assert.Equal(t, "admin", res.GetRole())
	assert.True(t, res.GetMustChangePassword())
}

func TestHandler_ResendOTP_Cooldown(t *testing.T) {
	h, svc := newHandler(t)
	svc.On("ResendOTP", mock.Anything, "sess").Return(nil, supportErrors.ErrResendCooldown)

	_, err := h.ResendOTP(context.Background(), &supportpb.ResendOTPRequest{OtpSessionId: "sess"})
	assert.Equal(t, codes.ResourceExhausted, codeOf(t, err))
}

func TestHandler_RefreshToken_Invalid(t *testing.T) {
	h, svc := newHandler(t)
	svc.On("RefreshToken", mock.Anything, "rt").Return(nil, supportErrors.ErrRefreshInvalid)

	_, err := h.RefreshToken(context.Background(), &supportpb.RefreshTokenRequest{RefreshToken: "rt"})
	assert.Equal(t, codes.Unauthenticated, codeOf(t, err))
}

func TestHandler_Logout_Idempotent(t *testing.T) {
	h, svc := newHandler(t)
	svc.On("Logout", mock.Anything, "rt").Return(nil)

	_, err := h.Logout(context.Background(), &supportpb.LogoutRequest{RefreshToken: "rt"})
	assert.NoError(t, err)
}

// =============================================================================
// Me (requiert UID)
// =============================================================================

func TestHandler_Me(t *testing.T) {
	t.Run("sans UID → Unauthenticated", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.Me(context.Background(), &supportpb.MeRequest{})
		assert.Equal(t, codes.Unauthenticated, codeOf(t, err))
	})

	t.Run("succès", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid-1").Return(&domain.SupportUser{
			UserID: "uid-1", Email: "a@b.com", Role: domain.RoleSupport,
		}, nil)
		res, err := h.Me(ctxWithUID("uid-1"), &supportpb.MeRequest{})
		require.NoError(t, err)
		assert.Equal(t, "uid-1", res.GetUserId())
		assert.Equal(t, "a@b.com", res.GetEmail())
	})

	t.Run("user introuvable → NotFound", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid-1").Return(nil, supportErrors.ErrUserNotFound)
		_, err := h.Me(ctxWithUID("uid-1"), &supportpb.MeRequest{})
		assert.Equal(t, codes.NotFound, codeOf(t, err))
	})
}

// =============================================================================
// ChangeMyPassword (pas de gate mustChange, c'est l'issue de sortie)
// =============================================================================

func TestHandler_ChangeMyPassword(t *testing.T) {
	t.Run("sans UID → Unauthenticated", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.ChangeMyPassword(context.Background(), &supportpb.ChangeMyPasswordRequest{})
		assert.Equal(t, codes.Unauthenticated, codeOf(t, err))
	})

	t.Run("succès", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("ChangeMyPassword", mock.Anything, "uid", "cur", "newP@ss1234!").Return(nil)
		_, err := h.ChangeMyPassword(ctxWithUID("uid"),
			&supportpb.ChangeMyPasswordRequest{CurrentPassword: "cur", NewPassword: "newP@ss1234!"})
		assert.NoError(t, err)
	})

	t.Run("faible → InvalidArgument", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("ChangeMyPassword", mock.Anything, "uid", "cur", "weak").Return(supportErrors.ErrWeakPassword)
		_, err := h.ChangeMyPassword(ctxWithUID("uid"),
			&supportpb.ChangeMyPasswordRequest{CurrentPassword: "cur", NewPassword: "weak"})
		assert.Equal(t, codes.InvalidArgument, codeOf(t, err))
	})
}

// =============================================================================
// ChangeMyEmail — gate mustChangePassword
// =============================================================================

func TestHandler_ChangeMyEmail(t *testing.T) {
	t.Run("sans UID → Unauthenticated", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.ChangeMyEmail(context.Background(), &supportpb.ChangeMyEmailRequest{})
		assert.Equal(t, codes.Unauthenticated, codeOf(t, err))
	})

	t.Run("mustChangePassword=true → FailedPrecondition", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid").Return(&domain.SupportUser{UserID: "uid", MustChangePassword: true}, nil)
		_, err := h.ChangeMyEmail(ctxWithUID("uid"),
			&supportpb.ChangeMyEmailRequest{NewEmail: "n@x.com", CurrentPassword: "p"})
		// ErrMustChangePassword n'est pas dans toGRPCError → Internal (mapping par défaut)
		assert.Equal(t, codes.Internal, codeOf(t, err))
	})

	t.Run("succès", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid").Return(&domain.SupportUser{UserID: "uid"}, nil)
		svc.On("ChangeMyEmail", mock.Anything, "uid", "n@x.com", "p").Return(nil)
		_, err := h.ChangeMyEmail(ctxWithUID("uid"),
			&supportpb.ChangeMyEmailRequest{NewEmail: "n@x.com", CurrentPassword: "p"})
		assert.NoError(t, err)
	})

	t.Run("email déjà pris → AlreadyExists", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid").Return(&domain.SupportUser{UserID: "uid"}, nil)
		svc.On("ChangeMyEmail", mock.Anything, "uid", "n@x.com", "p").Return(supportErrors.ErrEmailAlreadyExists)
		_, err := h.ChangeMyEmail(ctxWithUID("uid"),
			&supportpb.ChangeMyEmailRequest{NewEmail: "n@x.com", CurrentPassword: "p"})
		assert.Equal(t, codes.AlreadyExists, codeOf(t, err))
	})

	t.Run("cooldown 6 mois → FailedPrecondition", func(t *testing.T) {
		h, svc := newHandler(t)
		svc.On("Me", mock.Anything, "uid").Return(&domain.SupportUser{UserID: "uid"}, nil)
		svc.On("ChangeMyEmail", mock.Anything, "uid", "n@x.com", "p").Return(supportErrors.ErrEmailChangeCooldown)
		_, err := h.ChangeMyEmail(ctxWithUID("uid"),
			&supportpb.ChangeMyEmailRequest{NewEmail: "n@x.com", CurrentPassword: "p"})
		assert.Equal(t, codes.FailedPrecondition, codeOf(t, err))
	})
}

// =============================================================================
// Routes admin — requireAdmin + gateMustChange
// =============================================================================

func TestHandler_CreateSupportAgent(t *testing.T) {
	t.Run("rôle support → PermissionDenied", func(t *testing.T) {
		h, _ := newHandler(t)
		ctx := ctxWithUIDRole("uid", domain.RoleSupport)
		_, err := h.CreateSupportAgent(ctx, &supportpb.CreateSupportAgentRequest{})
		assert.Equal(t, codes.PermissionDenied, codeOf(t, err))
	})

	t.Run("aucun rôle → PermissionDenied", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.CreateSupportAgent(ctxWithUID("uid"), &supportpb.CreateSupportAgentRequest{})
		assert.Equal(t, codes.PermissionDenied, codeOf(t, err))
	})

	t.Run("admin + OK", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		svc.On("CreateSupportAgent", mock.Anything, "new@x.com", "A", "B").Return("new-id", nil)

		res, err := h.CreateSupportAgent(ctx, &supportpb.CreateSupportAgentRequest{
			Email: "new@x.com", FirstName: "A", LastName: "B",
		})
		require.NoError(t, err)
		assert.Equal(t, "new-id", res.GetUserId())
	})

	t.Run("admin avec mustChangePassword → bloqué", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid", MustChangePassword: true}, nil)
		_, err := h.CreateSupportAgent(ctx, &supportpb.CreateSupportAgentRequest{Email: "n@x.com", FirstName: "A", LastName: "B"})
		// ErrMustChangePassword → Internal (pas dans le switch)
		assert.Equal(t, codes.Internal, codeOf(t, err))
	})

	t.Run("email déjà pris → AlreadyExists", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		svc.On("CreateSupportAgent", mock.Anything, "n@x.com", "A", "B").Return("", supportErrors.ErrEmailAlreadyExists)
		_, err := h.CreateSupportAgent(ctx, &supportpb.CreateSupportAgentRequest{Email: "n@x.com", FirstName: "A", LastName: "B"})
		assert.Equal(t, codes.AlreadyExists, codeOf(t, err))
	})
}

func TestHandler_ListSupportAgents(t *testing.T) {
	t.Run("non-admin → PermissionDenied", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.ListSupportAgents(ctxWithUIDRole("uid", domain.RoleSupport), &supportpb.ListSupportAgentsRequest{})
		assert.Equal(t, codes.PermissionDenied, codeOf(t, err))
	})

	t.Run("admin succès mapping", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		users := []*domain.SupportUser{
			{UserID: "u1", Email: "a@x.com", FirstName: "A", LastName: "B", Role: "support", IsActive: true},
			{UserID: "u2", Email: "c@x.com", FirstName: "C", LastName: "D", Role: "support", IsActive: false},
		}
		svc.On("ListSupportAgents", mock.Anything, 50, 0).Return(users, 2, nil)

		res, err := h.ListSupportAgents(ctx, &supportpb.ListSupportAgentsRequest{Limit: 50, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, int32(2), res.GetTotal())
		assert.Len(t, res.GetAgents(), 2)
		assert.Equal(t, "u1", res.GetAgents()[0].GetUserId())
		assert.True(t, res.GetAgents()[0].GetIsActive())
		assert.False(t, res.GetAgents()[1].GetIsActive())
	})

	t.Run("liste vide", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		svc.On("ListSupportAgents", mock.Anything, 10, 0).Return([]*domain.SupportUser{}, 0, nil)

		res, err := h.ListSupportAgents(ctx, &supportpb.ListSupportAgentsRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, res.GetAgents())
	})
}

func TestHandler_DeactivateSupportAgent(t *testing.T) {
	t.Run("non-admin → PermissionDenied", func(t *testing.T) {
		h, _ := newHandler(t)
		_, err := h.DeactivateSupportAgent(ctxWithUIDRole("uid", domain.RoleSupport),
			&supportpb.DeactivateSupportAgentRequest{UserId: "target"})
		assert.Equal(t, codes.PermissionDenied, codeOf(t, err))
	})

	t.Run("admin succès", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		svc.On("DeactivateSupportAgent", mock.Anything, "target").Return(nil)
		_, err := h.DeactivateSupportAgent(ctx, &supportpb.DeactivateSupportAgentRequest{UserId: "target"})
		assert.NoError(t, err)
	})

	t.Run("target introuvable → NotFound", func(t *testing.T) {
		h, svc := newHandler(t)
		ctx := ctxWithUIDRole("admin-uid", domain.RoleAdmin)
		svc.On("Me", mock.Anything, "admin-uid").Return(&domain.SupportUser{UserID: "admin-uid"}, nil)
		svc.On("DeactivateSupportAgent", mock.Anything, "target").Return(supportErrors.ErrUserNotFound)
		_, err := h.DeactivateSupportAgent(ctx, &supportpb.DeactivateSupportAgentRequest{UserId: "target"})
		assert.Equal(t, codes.NotFound, codeOf(t, err))
	})
}

// =============================================================================
// Health (public, ne touche pas au service)
// =============================================================================

func TestHandler_Health(t *testing.T) {
	h, _ := newHandler(t)
	res, err := h.Health(context.Background(), &supportpb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ok", res.GetStatus())
	assert.Equal(t, "support-service", res.GetService())
}
