package otp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// Session représente l'état d'un OTP en cours de vérification.
type Session struct {
	Email     string `json:"email"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	CodeHash  string `json:"code_hash"`
	Attempts  int    `json:"attempts"`
	CreatedAt int64  `json:"created_at"`
}

// Store encapsule les opérations Redis liées aux OTP, à la limite d'échecs et au cooldown resend.
type Store struct {
	redis             *redis.Client
	otpTTL            time.Duration
	resendCooldown    time.Duration
	failWindow        time.Duration
}

func NewStore(r *redis.Client, otpTTL, resendCooldown, failWindow time.Duration) *Store {
	return &Store{
		redis:          r,
		otpTTL:         otpTTL,
		resendCooldown: resendCooldown,
		failWindow:     failWindow,
	}
}

func (s *Store) sessionKey(id string) string  { return "support:otp:" + id }
func (s *Store) failKey(email string) string  { return "support:fail:" + email }
func (s *Store) resendKey(email string) string { return "support:resend:" + email }

// SaveSession persiste/écrase une session OTP avec un TTL frais.
func (s *Store) SaveSession(ctx context.Context, sessionID string, sess *Session) error {
	b, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, s.sessionKey(sessionID), b, s.otpTTL).Err()
}

// GetSession récupère une session OTP. Retourne ErrOTPSessionNotFound si elle a expiré ou n'existe pas.
func (s *Store) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	val, err := s.redis.Get(ctx, s.sessionKey(sessionID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, supportErrors.ErrOTPSessionNotFound
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal([]byte(val), &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteSession supprime une session OTP (succès ou trop d'essais).
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	return s.redis.Del(ctx, s.sessionKey(sessionID)).Err()
}

// IncrFail incrémente le compteur d'échecs d'auth pour un email avec une fenêtre de 24h.
func (s *Store) IncrFail(ctx context.Context, email string) (int64, error) {
	key := s.failKey(email)
	pipe := s.redis.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, s.failWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// IsLocked retourne true si le compte est verrouillé (trop d'échecs).
func (s *Store) IsLocked(ctx context.Context, email string, threshold int) (bool, error) {
	val, err := s.redis.Get(ctx, s.failKey(email)).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return val >= threshold, nil
}

// ResetFail efface le compteur d'échecs après un login + OTP réussi.
func (s *Store) ResetFail(ctx context.Context, email string) error {
	return s.redis.Del(ctx, s.failKey(email)).Err()
}

// MarkResendCooldown pose un cooldown pour les demandes de resend OTP.
// Retourne ErrResendCooldown si le cooldown est déjà actif.
func (s *Store) MarkResendCooldown(ctx context.Context, email string) error {
	ok, err := s.redis.SetNX(ctx, s.resendKey(email), 1, s.resendCooldown).Result()
	if err != nil {
		return err
	}
	if !ok {
		return supportErrors.ErrResendCooldown
	}
	return nil
}

// ClearResendCooldown supprime le cooldown resend (après VerifyOTP réussi).
func (s *Store) ClearResendCooldown(ctx context.Context, email string) error {
	return s.redis.Del(ctx, s.resendKey(email)).Err()
}

// IncrSessionAttempts charge la session, incrémente le compteur d'essais, ré-écrit avec TTL refresh.
func (s *Store) IncrSessionAttempts(ctx context.Context, sessionID string, sess *Session) error {
	sess.Attempts++
	return s.SaveSession(ctx, sessionID, sess)
}

// FailWindowSeconds expose le TTL pour information.
func (s *Store) FailWindowSeconds() int { return int(s.failWindow.Seconds()) }

func (s *Store) String() string {
	return fmt.Sprintf("otp.Store(otp=%s, resend=%s, fail=%s)", s.otpTTL, s.resendCooldown, s.failWindow)
}
