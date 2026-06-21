package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/redis/go-redis/v9"
)

const resetTokenBytes = 32

// ResetStore gère les tokens de réinitialisation de mot de passe (Redis, usage unique).
type ResetStore struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewResetStore(r *redis.Client, ttl time.Duration) *ResetStore {
	return &ResetStore{redis: r, ttl: ttl}
}

func (s *ResetStore) key(hash string) string { return "support:pwreset:" + hash }

// Issue génère un token aléatoire, stocke son hash → userID avec TTL, et retourne le token brut.
func (s *ResetStore) Issue(ctx context.Context, userID string) (string, error) {
	b := make([]byte, resetTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash := base64.RawURLEncoding.EncodeToString(sum[:])

	if err := s.redis.Set(ctx, s.key(hash), userID, s.ttl).Err(); err != nil {
		return "", err
	}
	return raw, nil
}

// Consume valide un token brut et le supprime (usage unique). Retourne le userID associé.
// ErrResetTokenInvalid si le token est absent, expiré ou déjà utilisé.
func (s *ResetStore) Consume(ctx context.Context, raw string) (string, error) {
	if raw == "" {
		return "", supportErrors.ErrResetTokenInvalid
	}
	sum := sha256.Sum256([]byte(raw))
	hash := base64.RawURLEncoding.EncodeToString(sum[:])

	userID, err := s.redis.GetDel(ctx, s.key(hash)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", supportErrors.ErrResetTokenInvalid
		}
		return "", err
	}
	if userID == "" {
		return "", supportErrors.ErrResetTokenInvalid
	}
	return userID, nil
}
