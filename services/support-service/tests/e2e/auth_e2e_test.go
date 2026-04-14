package e2e

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	supportpb "github.com/Kpeewu/tissi-mah/services/support-service/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcCode(t *testing.T, err error) codes.Code {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected grpc status, got %T: %v", err, err)
	return st.Code()
}

func TestE2E_FullLoginFlow(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	user := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, user))

	// 1. Login → session OTP + email envoyé
	loginRes, err := d.client.Login(ctx, &supportpb.LoginRequest{
		Email: "agent@x.com", Password: fixtures.DefaultPasswordPlain,
	})
	require.NoError(t, err)
	require.NotEmpty(t, loginRes.GetOtpSessionId())

	waitEmails(t, d.email, 1)
	code := extractOTP(t, d.email)

	// 2. VerifyOTP → access + refresh tokens
	verifyRes, err := d.client.VerifyOTP(ctx, &supportpb.VerifyOTPRequest{
		OtpSessionId: loginRes.GetOtpSessionId(),
		Code:         code,
	})
	require.NoError(t, err)
	require.NotEmpty(t, verifyRes.GetAccessToken())
	require.NotEmpty(t, verifyRes.GetRefreshToken())
	assert.Equal(t, "support", verifyRes.GetRole())

	// 3. Me avec UID (simule middleware api-gateway après validation du JWT)
	meCtx := withSupport(ctx, user.UserID, "support")
	me, err := d.client.Me(meCtx, &supportpb.MeRequest{})
	require.NoError(t, err)
	assert.Equal(t, "agent@x.com", me.GetEmail())

	// 4. Refresh → nouveaux tokens, ancien invalidé
	refRes, err := d.client.RefreshToken(ctx, &supportpb.RefreshTokenRequest{
		RefreshToken: verifyRes.GetRefreshToken(),
	})
	require.NoError(t, err)
	assert.NotEqual(t, verifyRes.GetRefreshToken(), refRes.GetRefreshToken())

	// Ancien refresh rejeté
	_, err = d.client.RefreshToken(ctx, &supportpb.RefreshTokenRequest{
		RefreshToken: verifyRes.GetRefreshToken(),
	})
	assert.Equal(t, codes.Unauthenticated, grpcCode(t, err))

	// 5. Logout nouveau refresh (nécessite UID en metadata)
	_, err = d.client.Logout(meCtx, &supportpb.LogoutRequest{RefreshToken: refRes.GetRefreshToken()})
	require.NoError(t, err)

	// Après logout, re-use du refresh → Unauthenticated
	_, err = d.client.RefreshToken(ctx, &supportpb.RefreshTokenRequest{RefreshToken: refRes.GetRefreshToken()})
	require.Error(t, err)
}

func TestE2E_VerifyOTP_WrongCode(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	user := fixtures.NewTestSupportUser(fixtures.WithEmail("b@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, user))

	loginRes, err := d.client.Login(ctx, &supportpb.LoginRequest{
		Email: "b@x.com", Password: fixtures.DefaultPasswordPlain,
	})
	require.NoError(t, err)
	waitEmails(t, d.email, 1)

	// 3 essais ratés → session supprimée + TooManyAttempts
	for i := 0; i < 3; i++ {
		_, err := d.client.VerifyOTP(ctx, &supportpb.VerifyOTPRequest{
			OtpSessionId: loginRes.GetOtpSessionId(),
			Code:         "000000",
		})
		assert.Equal(t, codes.FailedPrecondition, grpcCode(t, err))
	}

	// 4e essai : session inexistante
	_, err = d.client.VerifyOTP(ctx, &supportpb.VerifyOTPRequest{
		OtpSessionId: loginRes.GetOtpSessionId(),
		Code:         "000000",
	})
	assert.Equal(t, codes.FailedPrecondition, grpcCode(t, err))
}

func TestE2E_Login_Lockout(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()
	ctx := context.Background()

	user := fixtures.NewTestSupportUser(fixtures.WithEmail("locked@x.com"))
	require.NoError(t, fixtures.InsertSupportUser(ctx, testPool, user))

	// 5 échecs → 6e appel (même correct) → PermissionDenied
	for i := 0; i < 5; i++ {
		_, err := d.client.Login(ctx, &supportpb.LoginRequest{
			Email: "locked@x.com", Password: "WrongPass!1",
		})
		assert.Equal(t, codes.Unauthenticated, grpcCode(t, err))
	}

	_, err := d.client.Login(ctx, &supportpb.LoginRequest{
		Email: "locked@x.com", Password: fixtures.DefaultPasswordPlain,
	})
	assert.Equal(t, codes.PermissionDenied, grpcCode(t, err))
}

func TestE2E_ProtectedRoute_NoUID(t *testing.T) {
	cleanAll(t)
	d := newE2E(t)
	defer d.cleanup()

	// Sans metadata → rejeté par l'intercepteur
	_, err := d.client.Me(context.Background(), &supportpb.MeRequest{})
	require.Error(t, err)
}
