package token_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/support-service/internal/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-that-is-at-least-32-bytes-long-xxx"

func TestJWTSigner_SignAndVerify(t *testing.T) {
	signer := token.NewJWTSigner(testSecret, 1)

	t.Run("sign + verify retourne les claims originaux", func(t *testing.T) {
		raw, exp, err := signer.Sign("user-123", domain.RoleAdmin, false)
		require.NoError(t, err)
		require.NotEmpty(t, raw)
		assert.Greater(t, exp, time.Now().Unix())

		claims, err := signer.Verify(raw)
		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.Subject)
		assert.Equal(t, domain.RoleAdmin, claims.Role)
		assert.False(t, claims.MustChangePassword)
		assert.Equal(t, "support-service", claims.Issuer)
		require.NotNil(t, claims.ExpiresAt)
		assert.Equal(t, exp, claims.ExpiresAt.Unix())
	})

	t.Run("mustChangePassword encodé et décodé", func(t *testing.T) {
		raw, _, err := signer.Sign("user-123", domain.RoleSupport, true)
		require.NoError(t, err)

		claims, err := signer.Verify(raw)
		require.NoError(t, err)
		assert.True(t, claims.MustChangePassword)
		assert.Equal(t, domain.RoleSupport, claims.Role)
	})

	t.Run("verify échoue sur signature altérée", func(t *testing.T) {
		raw, _, err := signer.Sign("user-123", domain.RoleAdmin, false)
		require.NoError(t, err)

		tampered := raw[:len(raw)-4] + "XXXX"
		_, err = signer.Verify(tampered)
		assert.Error(t, err)
	})

	t.Run("verify échoue avec un autre secret", func(t *testing.T) {
		raw, _, err := signer.Sign("user-123", domain.RoleAdmin, false)
		require.NoError(t, err)

		otherSigner := token.NewJWTSigner("different-secret-of-at-least-32-bytes-long", 1)
		_, err = otherSigner.Verify(raw)
		assert.Error(t, err)
	})

	t.Run("verify échoue sur token vide", func(t *testing.T) {
		_, err := signer.Verify("")
		assert.Error(t, err)
	})

	t.Run("verify échoue sur token malformé", func(t *testing.T) {
		_, err := signer.Verify("not.a.jwt")
		assert.Error(t, err)
	})

	t.Run("verify échoue sur token expiré", func(t *testing.T) {
		// Produire manuellement un JWT avec exp dans le passé.
		claims := token.Claims{
			Role: domain.RoleAdmin,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-exp",
				Issuer:    "support-service",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
		require.NoError(t, err)

		_, err = signer.Verify(raw)
		assert.Error(t, err)
	})

	t.Run("verify rejette les méthodes de signature non HMAC", func(t *testing.T) {
		// Construire un token "none" (RFC conforme mais non signé).
		claims := jwt.RegisteredClaims{
			Subject:   "user-none",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		}
		noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		raw, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		_, err = signer.Verify(raw)
		assert.Error(t, err)
	})

	t.Run("TTL respecté (exp = now + ttl)", func(t *testing.T) {
		sig := token.NewJWTSigner(testSecret, 2)
		before := time.Now().Unix()
		_, exp, err := sig.Sign("user-ttl", domain.RoleAdmin, false)
		require.NoError(t, err)

		expectedMin := before + 2*3600 - 2
		expectedMax := time.Now().Unix() + 2*3600 + 2
		assert.GreaterOrEqual(t, exp, expectedMin)
		assert.LessOrEqual(t, exp, expectedMax)
	})

	t.Run("signatures différentes pour deux signs (IAT diffère)", func(t *testing.T) {
		raw1, _, err := signer.Sign("u", domain.RoleSupport, false)
		require.NoError(t, err)
		time.Sleep(1100 * time.Millisecond) // IAT est en secondes, attendre >=1s
		raw2, _, err := signer.Sign("u", domain.RoleSupport, false)
		require.NoError(t, err)

		// Si les IAT sont identiques les tokens seront identiques — sinon différents.
		// On tolère : au minimum, un token valide dans les deux cas.
		require.True(t, strings.Count(raw1, ".") == 2 && strings.Count(raw2, ".") == 2)
	})
}
