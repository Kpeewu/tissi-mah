package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/otp"
	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStore(otpTTL, resend, fail time.Duration) *otp.Store {
	testMiniSrv.FlushAll()
	return otp.NewStore(testRedis, otpTTL, resend, fail)
}

func TestOTPStore_SessionLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newStore(5*time.Minute, time.Minute, time.Hour)

	sess := &otp.Session{Email: "a@x.com", UserID: "uid", Role: "support", CodeHash: "h"}
	require.NoError(t, st.SaveSession(ctx, "s1", sess))

	got, err := st.GetSession(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, "a@x.com", got.Email)

	require.NoError(t, st.DeleteSession(ctx, "s1"))
	_, err = st.GetSession(ctx, "s1")
	assert.ErrorIs(t, err, supportErrors.ErrOTPSessionNotFound)
}

func TestOTPStore_SessionTTLExpires(t *testing.T) {
	ctx := context.Background()
	st := newStore(2*time.Second, time.Minute, time.Hour)

	require.NoError(t, st.SaveSession(ctx, "s1", &otp.Session{Email: "a@x.com"}))

	// miniredis ne vieillit pas tout seul : on avance l'horloge.
	testMiniSrv.FastForward(3 * time.Second)

	_, err := st.GetSession(ctx, "s1")
	assert.ErrorIs(t, err, supportErrors.ErrOTPSessionNotFound)
}

func TestOTPStore_IncrSessionAttempts(t *testing.T) {
	ctx := context.Background()
	st := newStore(5*time.Minute, time.Minute, time.Hour)

	sess := &otp.Session{Email: "a@x.com", Attempts: 0}
	require.NoError(t, st.SaveSession(ctx, "s1", sess))

	require.NoError(t, st.IncrSessionAttempts(ctx, "s1", sess))
	require.NoError(t, st.IncrSessionAttempts(ctx, "s1", sess))

	got, err := st.GetSession(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, 2, got.Attempts)
}

func TestOTPStore_FailCounter(t *testing.T) {
	ctx := context.Background()
	st := newStore(5*time.Minute, time.Minute, time.Hour)

	for i := int64(1); i <= 4; i++ {
		n, err := st.IncrFail(ctx, "brute@x.com")
		require.NoError(t, err)
		assert.Equal(t, i, n)
	}

	locked, err := st.IsLocked(ctx, "brute@x.com", 5)
	require.NoError(t, err)
	assert.False(t, locked, "4 < threshold 5")

	_, _ = st.IncrFail(ctx, "brute@x.com")
	locked, err = st.IsLocked(ctx, "brute@x.com", 5)
	require.NoError(t, err)
	assert.True(t, locked)

	// Reset
	require.NoError(t, st.ResetFail(ctx, "brute@x.com"))
	locked, err = st.IsLocked(ctx, "brute@x.com", 5)
	require.NoError(t, err)
	assert.False(t, locked)
}

func TestOTPStore_FailCounter_Window(t *testing.T) {
	ctx := context.Background()
	st := newStore(5*time.Minute, time.Minute, 2*time.Second)

	for i := 0; i < 5; i++ {
		_, _ = st.IncrFail(ctx, "x@x.com")
	}
	locked, _ := st.IsLocked(ctx, "x@x.com", 5)
	assert.True(t, locked)

	// Fenêtre expirée → compteur effacé.
	testMiniSrv.FastForward(3 * time.Second)
	locked, _ = st.IsLocked(ctx, "x@x.com", 5)
	assert.False(t, locked)
}

func TestOTPStore_ResendCooldown(t *testing.T) {
	ctx := context.Background()
	st := newStore(5*time.Minute, 2*time.Second, time.Hour)

	require.NoError(t, st.MarkResendCooldown(ctx, "a@x.com"))

	err := st.MarkResendCooldown(ctx, "a@x.com")
	assert.ErrorIs(t, err, supportErrors.ErrResendCooldown)

	// Fast-forward passé le cooldown.
	testMiniSrv.FastForward(3 * time.Second)
	require.NoError(t, st.MarkResendCooldown(ctx, "a@x.com"))

	// ClearResendCooldown permet immédiatement un nouveau marquage.
	require.NoError(t, st.ClearResendCooldown(ctx, "a@x.com"))
	require.NoError(t, st.MarkResendCooldown(ctx, "a@x.com"))
}
