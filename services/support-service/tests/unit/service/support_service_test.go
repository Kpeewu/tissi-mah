package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/config"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/password"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/service"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/token"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/support-service/tests/mocks"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// Helpers
// =============================================================================

type testDeps struct {
	svc       svcIfaces.SupportService
	cfg       *config.Config
	readRepo  *mocks.MockSupportUserReadRepository
	writeRepo *mocks.MockSupportUserWriteRepository
	email     *mocks.SpyEmailSender
	otpStore  *otp.Store
	jwt       *token.JWTSigner
	refresh   *token.RefreshStore
	redisSrv  *miniredis.Miniredis
	redisCli  *redis.Client
}

func svc(d *testDeps) svcIfaces.SupportService { return d.svc }

func newDeps(t *testing.T) *testDeps {
	t.Helper()

	srv, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	cli := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = cli.Close() })

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:          "test-secret-that-is-at-least-32-bytes-long-xxx",
			AccessTTLHours:  1,
			RefreshTTLHours: 24,
		},
		OTP: config.OTPConfig{
			TTLSeconds:        300,
			MaxAttempts:       3,
			ResendCooldownSec: 300,
		},
		RateLimit: config.RateLimitConfig{
			FailThreshold:     5,
			FailWindowSeconds: 86400,
		},
	}

	readRepo := new(mocks.MockSupportUserReadRepository)
	writeRepo := new(mocks.MockSupportUserWriteRepository)
	email := mocks.NewSpyEmailSender()

	otpStore := otp.NewStore(cli,
		time.Duration(cfg.OTP.TTLSeconds)*time.Second,
		time.Duration(cfg.OTP.ResendCooldownSec)*time.Second,
		time.Duration(cfg.RateLimit.FailWindowSeconds)*time.Second,
	)
	jwtSig := token.NewJWTSigner(cfg.JWT.Secret, cfg.JWT.AccessTTLHours)
	refresh := token.NewRefreshStore(cli, cfg.JWT.RefreshTTLHours)

	svc := service.NewSupportService(cfg, readRepo, writeRepo, otpStore, jwtSig, refresh, email, zap.NewNop())

	return &testDeps{
		svc:       svc,
		cfg:       cfg,
		readRepo:  readRepo,
		writeRepo: writeRepo,
		email:     email,
		otpStore:  otpStore,
		jwt:       jwtSig,
		refresh:   refresh,
		redisSrv:  srv,
		redisCli:  cli,
	}
}

// waitForEmails attend que le spy ait reçu au moins n appels, avec timeout.
// Nécessaire car les envois sont dans des goroutines.
func waitForEmails(t *testing.T, email *mocks.SpyEmailSender, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if email.Count() >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout attendant %d emails, reçu %d", n, email.Count())
}

// =============================================================================
// Login
// =============================================================================

func TestSupportService_Login(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : crée session OTP et envoie email", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@tissimah.local"))
		d.readRepo.On("GetByEmail", mock.Anything, "agent@tissimah.local").Return(user, nil)

		res, err := svc(d).Login(ctx, "agent@tissimah.local", fixtures.DefaultPasswordPlain)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEmpty(t, res.OTPSessionID)
		assert.Equal(t, 300, res.ExpiresInSeconds)

		waitForEmails(t, d.email, 1)
		assert.Equal(t, "agent@tissimah.local", d.email.LastTo())
	})

	t.Run("email normalisé (lowercase + trim)", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@tissimah.local"))
		d.readRepo.On("GetByEmail", mock.Anything, "agent@tissimah.local").Return(user, nil)

		_, err := svc(d).Login(ctx, "  AGENT@Tissimah.Local  ", fixtures.DefaultPasswordPlain)
		require.NoError(t, err)
	})

	t.Run("email vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).Login(ctx, "", fixtures.DefaultPasswordPlain)
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("password vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).Login(ctx, "a@b.com", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("compte verrouillé (seuil d'échecs atteint)", func(t *testing.T) {
		d := newDeps(t)
		// Simuler le seuil atteint : incr 5 fois.
		for i := 0; i < 5; i++ {
			_, _ = d.otpStore.IncrFail(ctx, "locked@x.com")
		}
		_, err := svc(d).Login(ctx, "locked@x.com", "whatever")
		assert.ErrorIs(t, err, supportErrors.ErrAccountLocked)
	})

	t.Run("email inexistant → ErrInvalidCredentials + incrémente fail counter", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("GetByEmail", mock.Anything, "nope@x.com").Return(nil, supportErrors.ErrUserNotFound)

		_, err := svc(d).Login(ctx, "nope@x.com", "whatever")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidCredentials)

		locked, _ := d.otpStore.IsLocked(ctx, "nope@x.com", 1)
		assert.True(t, locked, "fail counter doit avoir été incrémenté")
	})

	t.Run("compte désactivé → ErrInvalidCredentials", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(
			fixtures.WithEmail("inactive@x.com"),
			fixtures.WithInactive(),
		)
		d.readRepo.On("GetByEmail", mock.Anything, "inactive@x.com").Return(user, nil)

		_, err := svc(d).Login(ctx, "inactive@x.com", fixtures.DefaultPasswordPlain)
		assert.ErrorIs(t, err, supportErrors.ErrInvalidCredentials)
	})

	t.Run("mot de passe incorrect → ErrInvalidCredentials + incrémente fail counter", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithEmail("agent@x.com"))
		d.readRepo.On("GetByEmail", mock.Anything, "agent@x.com").Return(user, nil)

		_, err := svc(d).Login(ctx, "agent@x.com", "WrongPassword1!")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidCredentials)
	})

	t.Run("5 échecs successifs → 6e login retourne ErrAccountLocked", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithEmail("brute@x.com"))
		d.readRepo.On("GetByEmail", mock.Anything, "brute@x.com").Return(user, nil)

		for i := 0; i < 5; i++ {
			_, err := svc(d).Login(ctx, "brute@x.com", "bad")
			assert.ErrorIs(t, err, supportErrors.ErrInvalidCredentials)
		}
		_, err := svc(d).Login(ctx, "brute@x.com", fixtures.DefaultPasswordPlain)
		assert.ErrorIs(t, err, supportErrors.ErrAccountLocked)
	})
}

// =============================================================================
// VerifyOTP
// =============================================================================

func TestSupportService_VerifyOTP(t *testing.T) {
	ctx := context.Background()

	createValidSession := func(t *testing.T, d *testDeps, user *domain.SupportUser, code string) string {
		t.Helper()
		hash, err := password.Hash(code)
		require.NoError(t, err)
		sessionID := "sess-" + user.UserID
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, &otp.Session{
			Email:    user.Email,
			UserID:   user.UserID,
			Role:     user.Role,
			CodeHash: hash,
		}))
		return sessionID
	}

	t.Run("succès : retourne tokens", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := createValidSession(t, d, user, "123456")
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		res, err := svc(d).VerifyOTP(ctx, sessionID, "123456")
		require.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.Greater(t, res.AccessExpiresAt, time.Now().Unix())
		assert.Greater(t, res.RefreshExpiresAt, time.Now().Unix())
		assert.Equal(t, user.Role, res.Role)
		assert.False(t, res.MustChangePassword)
	})

	t.Run("mustChangePassword propagé", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithMustChangePassword(true))
		sessionID := createValidSession(t, d, user, "654321")
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		res, err := svc(d).VerifyOTP(ctx, sessionID, "654321")
		require.NoError(t, err)
		assert.True(t, res.MustChangePassword)
	})

	t.Run("session inexistante → ErrOTPExpired", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).VerifyOTP(ctx, "no-such-session", "123456")
		assert.ErrorIs(t, err, supportErrors.ErrOTPExpired)
	})

	t.Run("sessionID vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).VerifyOTP(ctx, "", "123456")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("code vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).VerifyOTP(ctx, "sess", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("code incorrect → ErrOTPInvalid + incrémente attempts", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := createValidSession(t, d, user, "111111")

		_, err := svc(d).VerifyOTP(ctx, sessionID, "999999")
		assert.ErrorIs(t, err, supportErrors.ErrOTPInvalid)

		sess, err := d.otpStore.GetSession(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, 1, sess.Attempts)
	})

	t.Run("MaxAttempts atteint → supprime session + ErrOTPTooManyAttempts", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := createValidSession(t, d, user, "111111")
		sess, _ := d.otpStore.GetSession(ctx, sessionID)
		sess.Attempts = 3 // MaxAttempts dans test config
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, sess))

		_, err := svc(d).VerifyOTP(ctx, sessionID, "111111")
		assert.ErrorIs(t, err, supportErrors.ErrOTPTooManyAttempts)

		_, err = d.otpStore.GetSession(ctx, sessionID)
		assert.Error(t, err, "session doit avoir été supprimée")
	})

	t.Run("compte verrouillé entre Login et VerifyOTP → ErrAccountLocked", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithEmail("locked@x.com"))
		sessionID := createValidSession(t, d, user, "111111")
		for i := 0; i < 5; i++ {
			_, _ = d.otpStore.IncrFail(ctx, "locked@x.com")
		}

		_, err := svc(d).VerifyOTP(ctx, sessionID, "111111")
		assert.ErrorIs(t, err, supportErrors.ErrAccountLocked)
	})

	t.Run("user désactivé entre-temps → ErrAccountInactive", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithInactive())
		sessionID := createValidSession(t, d, user, "111111")
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		_, err := svc(d).VerifyOTP(ctx, sessionID, "111111")
		assert.ErrorIs(t, err, supportErrors.ErrAccountInactive)
	})

	t.Run("user introuvable → ErrAccountInactive", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := createValidSession(t, d, user, "111111")
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(nil, supportErrors.ErrUserNotFound)

		_, err := svc(d).VerifyOTP(ctx, sessionID, "111111")
		assert.ErrorIs(t, err, supportErrors.ErrAccountInactive)
	})

	t.Run("après succès : session supprimée, fail counter reset, cooldown resend effacé", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := createValidSession(t, d, user, "222222")
		_, _ = d.otpStore.IncrFail(ctx, user.Email)
		_ = d.otpStore.MarkResendCooldown(ctx, user.Email)
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		_, err := svc(d).VerifyOTP(ctx, sessionID, "222222")
		require.NoError(t, err)

		_, err = d.otpStore.GetSession(ctx, sessionID)
		assert.Error(t, err)
		locked, _ := d.otpStore.IsLocked(ctx, user.Email, 1)
		assert.False(t, locked)
	})
}

// =============================================================================
// ResendOTP
// =============================================================================

func TestSupportService_ResendOTP(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : nouveau code envoyé", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := "sess-resend"
		hash, _ := password.Hash("old")
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, &otp.Session{
			Email: user.Email, UserID: user.UserID, Role: user.Role, CodeHash: hash,
		}))
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		res, err := svc(d).ResendOTP(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, sessionID, res.OTPSessionID)

		waitForEmails(t, d.email, 1)
		assert.Equal(t, user.Email, d.email.LastTo())
	})

	t.Run("sessionID vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).ResendOTP(ctx, "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("session inexistante → ErrOTPSessionNotFound", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).ResendOTP(ctx, "no-session")
		assert.ErrorIs(t, err, supportErrors.ErrOTPSessionNotFound)
	})

	t.Run("cooldown actif → ErrResendCooldown", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := "sess-cd"
		hash, _ := password.Hash("old")
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, &otp.Session{
			Email: user.Email, UserID: user.UserID, Role: user.Role, CodeHash: hash,
		}))
		require.NoError(t, d.otpStore.MarkResendCooldown(ctx, user.Email))

		_, err := svc(d).ResendOTP(ctx, sessionID)
		assert.ErrorIs(t, err, supportErrors.ErrResendCooldown)
	})

	t.Run("compte verrouillé → ErrAccountLocked", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		sessionID := "sess-lock"
		hash, _ := password.Hash("old")
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, &otp.Session{
			Email: user.Email, UserID: user.UserID, Role: user.Role, CodeHash: hash,
		}))
		for i := 0; i < 5; i++ {
			_, _ = d.otpStore.IncrFail(ctx, user.Email)
		}

		_, err := svc(d).ResendOTP(ctx, sessionID)
		assert.ErrorIs(t, err, supportErrors.ErrAccountLocked)
	})

	t.Run("user désactivé entre-temps → ErrAccountInactive", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithInactive())
		sessionID := "sess-inact"
		hash, _ := password.Hash("old")
		require.NoError(t, d.otpStore.SaveSession(ctx, sessionID, &otp.Session{
			Email: user.Email, UserID: user.UserID, Role: user.Role, CodeHash: hash,
		}))
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		_, err := svc(d).ResendOTP(ctx, sessionID)
		assert.ErrorIs(t, err, supportErrors.ErrAccountInactive)
	})
}

// =============================================================================
// RefreshToken
// =============================================================================

func TestSupportService_RefreshToken(t *testing.T) {
	ctx := context.Background()

	issueRefresh := func(t *testing.T, d *testDeps, user *domain.SupportUser) string {
		t.Helper()
		raw, _, _, err := d.refresh.Issue(ctx, user.UserID, user.Role, "")
		require.NoError(t, err)
		return raw
	}

	t.Run("succès : rotation, nouveau token différent, ancien invalidé", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		raw := issueRefresh(t, d, user)
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		res, err := svc(d).RefreshToken(ctx, raw)
		require.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.NotEqual(t, raw, res.RefreshToken)

		// Ancien token invalidé
		_, err = d.refresh.Verify(ctx, raw)
		assert.Error(t, err)

		// Nouveau token valide
		_, err = d.refresh.Verify(ctx, res.RefreshToken)
		assert.NoError(t, err)
	})

	t.Run("token vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).RefreshToken(ctx, "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("token inexistant → ErrRefreshInvalid", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).RefreshToken(ctx, "fake-raw-token")
		assert.ErrorIs(t, err, supportErrors.ErrRefreshInvalid)
	})

	t.Run("réutilisation de l'ancien token après rotation → ErrRefreshInvalid", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		raw1 := issueRefresh(t, d, user)
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		// 1ère rotation : OK, retourne raw2
		res, err := svc(d).RefreshToken(ctx, raw1)
		require.NoError(t, err)
		raw2 := res.RefreshToken

		// Réutiliser raw1 (déjà rotaté) → rejeté
		_, err = svc(d).RefreshToken(ctx, raw1)
		assert.ErrorIs(t, err, supportErrors.ErrRefreshInvalid)

		// raw2 reste valide (token courant de la famille)
		_, err = d.refresh.Verify(ctx, raw2)
		assert.NoError(t, err)
	})

	t.Run("user désactivé entre-temps → ErrAccountInactive + famille révoquée", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithInactive())
		raw := issueRefresh(t, d, user)
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		_, err := svc(d).RefreshToken(ctx, raw)
		assert.ErrorIs(t, err, supportErrors.ErrAccountInactive)

		// Token original invalidé
		_, err = d.refresh.Verify(ctx, raw)
		assert.Error(t, err)
	})

	t.Run("user introuvable → ErrAccountInactive", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		raw := issueRefresh(t, d, user)
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(nil, supportErrors.ErrUserNotFound)

		_, err := svc(d).RefreshToken(ctx, raw)
		assert.ErrorIs(t, err, supportErrors.ErrAccountInactive)
	})
}

// =============================================================================
// Logout
// =============================================================================

func TestSupportService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : révoque le token", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		raw, _, _, err := d.refresh.Issue(ctx, user.UserID, user.Role, "")
		require.NoError(t, err)

		err = svc(d).Logout(ctx, raw)
		require.NoError(t, err)

		_, err = d.refresh.Verify(ctx, raw)
		assert.Error(t, err, "token doit être révoqué")
	})

	t.Run("token vide est idempotent (no-op)", func(t *testing.T) {
		d := newDeps(t)
		assert.NoError(t, svc(d).Logout(ctx, ""))
	})

	t.Run("token inexistant est idempotent", func(t *testing.T) {
		d := newDeps(t)
		assert.NoError(t, svc(d).Logout(ctx, "fake-raw-token"))
	})
}

// =============================================================================
// Me
// =============================================================================

func TestSupportService_Me(t *testing.T) {
	ctx := context.Background()

	t.Run("succès", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		out, err := svc(d).Me(ctx, user.UserID)
		require.NoError(t, err)
		assert.Equal(t, user.UserID, out.UserID)
	})

	t.Run("userID vide → ErrUnauthenticated", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).Me(ctx, "")
		assert.ErrorIs(t, err, supportErrors.ErrUnauthenticated)
	})

	t.Run("repo error → propagée", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("GetByID", mock.Anything, "x").Return(nil, supportErrors.ErrUserNotFound)

		_, err := svc(d).Me(ctx, "x")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

// =============================================================================
// ChangeMyPassword
// =============================================================================

func TestSupportService_ChangeMyPassword(t *testing.T) {
	ctx := context.Background()
	strongNew := "NewStr0ng!Pass"

	t.Run("succès : hash mis à jour + mustChange=false", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser(fixtures.WithMustChangePassword(true))
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)
		d.writeRepo.On("UpdatePassword", mock.Anything, user.UserID, mock.Anything, false).Return(nil)

		err := svc(d).ChangeMyPassword(ctx, user.UserID, fixtures.DefaultPasswordPlain, strongNew)
		require.NoError(t, err)
		d.writeRepo.AssertCalled(t, "UpdatePassword", mock.Anything, user.UserID, mock.Anything, false)
	})

	t.Run("user introuvable → erreur propagée", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("GetByID", mock.Anything, "x").Return(nil, supportErrors.ErrUserNotFound)

		err := svc(d).ChangeMyPassword(ctx, "x", "cur", strongNew)
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})

	t.Run("ancien mot de passe incorrect → ErrInvalidCredentials", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		err := svc(d).ChangeMyPassword(ctx, user.UserID, "WrongOld1!", strongNew)
		assert.ErrorIs(t, err, supportErrors.ErrInvalidCredentials)
	})

	t.Run("nouveau mot de passe faible → ErrWeakPassword", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)

		err := svc(d).ChangeMyPassword(ctx, user.UserID, fixtures.DefaultPasswordPlain, "weak")
		assert.ErrorIs(t, err, supportErrors.ErrWeakPassword)
	})

	t.Run("UpdatePassword renvoie erreur → propagée", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)
		dbErr := errors.New("db down")
		d.writeRepo.On("UpdatePassword", mock.Anything, user.UserID, mock.Anything, false).Return(dbErr)

		err := svc(d).ChangeMyPassword(ctx, user.UserID, fixtures.DefaultPasswordPlain, strongNew)
		assert.ErrorIs(t, err, dbErr)
	})
}

// =============================================================================
// UpdateMyProfile
// =============================================================================

func TestSupportService_UpdateMyProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("succès", func(t *testing.T) {
		d := newDeps(t)
		d.writeRepo.On("UpdateName", mock.Anything, "uid-1", "Jean", "Dupont").Return(nil)

		err := svc(d).UpdateMyProfile(ctx, "uid-1", "Jean", "Dupont")
		require.NoError(t, err)
	})

	t.Run("trim des espaces", func(t *testing.T) {
		d := newDeps(t)
		d.writeRepo.On("UpdateName", mock.Anything, "uid-1", "Jean", "Dupont").Return(nil)

		err := svc(d).UpdateMyProfile(ctx, "uid-1", "  Jean ", " Dupont  ")
		require.NoError(t, err)
	})

	t.Run("firstName vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		err := svc(d).UpdateMyProfile(ctx, "uid-1", "", "Dupont")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("lastName vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		err := svc(d).UpdateMyProfile(ctx, "uid-1", "Jean", "  ")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("user introuvable → erreur propagée", func(t *testing.T) {
		d := newDeps(t)
		d.writeRepo.On("UpdateName", mock.Anything, "x", "Jean", "Dupont").Return(supportErrors.ErrUserNotFound)

		err := svc(d).UpdateMyProfile(ctx, "x", "Jean", "Dupont")
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

// =============================================================================
// UpdateSupportAgent (admin)
// =============================================================================

func TestSupportService_UpdateSupportAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : email + rôle", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)
		d.readRepo.On("ExistsByEmail", mock.Anything, "new@x.com").Return(false, nil)
		d.writeRepo.On("UpdateEmail", mock.Anything, user.UserID, "new@x.com").Return(nil)
		d.writeRepo.On("UpdateRole", mock.Anything, user.UserID, domain.RoleAdmin).Return(nil)

		err := svc(d).UpdateSupportAgent(ctx, user.UserID, "New@X.com", domain.RoleAdmin)
		require.NoError(t, err)
	})

	t.Run("succès : rôle seul", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)
		d.writeRepo.On("UpdateRole", mock.Anything, user.UserID, domain.RoleAdmin).Return(nil)

		err := svc(d).UpdateSupportAgent(ctx, user.UserID, "", domain.RoleAdmin)
		require.NoError(t, err)
	})

	t.Run("aucun champ → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		err := svc(d).UpdateSupportAgent(ctx, "uid-1", "", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("rôle invalide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		err := svc(d).UpdateSupportAgent(ctx, "uid-1", "", "boss")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("email déjà utilisé → ErrEmailAlreadyExists", func(t *testing.T) {
		d := newDeps(t)
		user := fixtures.NewTestSupportUser()
		d.readRepo.On("GetByID", mock.Anything, user.UserID).Return(user, nil)
		d.readRepo.On("ExistsByEmail", mock.Anything, "taken@x.com").Return(true, nil)

		err := svc(d).UpdateSupportAgent(ctx, user.UserID, "taken@x.com", "")
		assert.ErrorIs(t, err, supportErrors.ErrEmailAlreadyExists)
	})

	t.Run("user introuvable → erreur propagée", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("GetByID", mock.Anything, "x").Return(nil, supportErrors.ErrUserNotFound)

		err := svc(d).UpdateSupportAgent(ctx, "x", "", domain.RoleAdmin)
		assert.ErrorIs(t, err, supportErrors.ErrUserNotFound)
	})
}

// =============================================================================
// CreateSupportAgent
// =============================================================================

func TestSupportService_CreateSupportAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : user créé + email envoyé avec temp password", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("ExistsByEmail", mock.Anything, "new@x.com").Return(false, nil)
		d.writeRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *domain.SupportUser) bool {
			return u.Email == "new@x.com" &&
				u.FirstName == "John" &&
				u.LastName == "Doe" &&
				u.Role == domain.RoleSupport &&
				u.IsActive &&
				u.MustChangePassword &&
				u.PasswordHash != ""
		})).Return(nil)

		id, err := svc(d).CreateSupportAgent(ctx, "New@X.com", "John", "Doe", domain.RoleSupport)
		require.NoError(t, err)
		assert.NotEmpty(t, id)

		waitForEmails(t, d.email, 1)
		assert.Equal(t, "new@x.com", d.email.LastTo())
	})

	t.Run("rôle admin explicite → user créé avec rôle admin", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("ExistsByEmail", mock.Anything, "boss@x.com").Return(false, nil)
		d.writeRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *domain.SupportUser) bool {
			return u.Email == "boss@x.com" && u.Role == domain.RoleAdmin
		})).Return(nil)

		id, err := svc(d).CreateSupportAgent(ctx, "boss@x.com", "Big", "Boss", domain.RoleAdmin)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("rôle invalide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "A", "B", "superuser")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("rôle vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "A", "B", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("email vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).CreateSupportAgent(ctx, "", "A", "B", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("firstName vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "", "B", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("lastName vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "A", "", "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("email déjà pris → ErrEmailAlreadyExists", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("ExistsByEmail", mock.Anything, "taken@x.com").Return(true, nil)

		_, err := svc(d).CreateSupportAgent(ctx, "taken@x.com", "A", "B", domain.RoleSupport)
		assert.ErrorIs(t, err, supportErrors.ErrEmailAlreadyExists)
	})

	t.Run("ExistsByEmail erreur → propagée", func(t *testing.T) {
		d := newDeps(t)
		dbErr := errors.New("db down")
		d.readRepo.On("ExistsByEmail", mock.Anything, "e@x.com").Return(false, dbErr)

		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "A", "B", domain.RoleSupport)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("Create erreur → propagée", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("ExistsByEmail", mock.Anything, "e@x.com").Return(false, nil)
		dbErr := errors.New("db down")
		d.writeRepo.On("Create", mock.Anything, mock.Anything).Return(dbErr)

		_, err := svc(d).CreateSupportAgent(ctx, "e@x.com", "A", "B", domain.RoleSupport)
		assert.ErrorIs(t, err, dbErr)
	})
}

// =============================================================================
// ListSupportAgents
// =============================================================================

func TestSupportService_ListSupportAgents(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : délègue au repo", func(t *testing.T) {
		d := newDeps(t)
		users := []*domain.SupportUser{fixtures.NewTestSupportUser(), fixtures.NewTestSupportUser()}
		d.readRepo.On("List", mock.Anything, 10, 0).Return(users, 2, nil)

		out, total, err := svc(d).ListSupportAgents(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, out, 2)
		assert.Equal(t, 2, total)
	})

	t.Run("repo vide", func(t *testing.T) {
		d := newDeps(t)
		d.readRepo.On("List", mock.Anything, 10, 0).Return([]*domain.SupportUser{}, 0, nil)

		out, total, err := svc(d).ListSupportAgents(ctx, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Zero(t, total)
	})

	t.Run("repo erreur", func(t *testing.T) {
		d := newDeps(t)
		dbErr := errors.New("db down")
		d.readRepo.On("List", mock.Anything, 10, 0).Return(nil, 0, dbErr)

		_, _, err := svc(d).ListSupportAgents(ctx, 10, 0)
		assert.ErrorIs(t, err, dbErr)
	})
}

// =============================================================================
// DeactivateSupportAgent
// =============================================================================

func TestSupportService_DeactivateSupportAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("succès : délègue au repo", func(t *testing.T) {
		d := newDeps(t)
		d.writeRepo.On("Deactivate", mock.Anything, "uid-1").Return(nil)

		err := svc(d).DeactivateSupportAgent(ctx, "uid-1")
		require.NoError(t, err)
	})

	t.Run("userID vide → ErrInvalidInput", func(t *testing.T) {
		d := newDeps(t)
		err := svc(d).DeactivateSupportAgent(ctx, "")
		assert.ErrorIs(t, err, supportErrors.ErrInvalidInput)
	})

	t.Run("repo erreur → propagée", func(t *testing.T) {
		d := newDeps(t)
		dbErr := errors.New("db down")
		d.writeRepo.On("Deactivate", mock.Anything, "uid-1").Return(dbErr)

		err := svc(d).DeactivateSupportAgent(ctx, "uid-1")
		assert.ErrorIs(t, err, dbErr)
	})
}
