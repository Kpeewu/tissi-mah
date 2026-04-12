package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	supportErrors "github.com/Kpeewu/tissi-mah/services/support-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const refreshTokenBytes = 32

// RefreshRecord est le payload stocké en Redis pour un refresh token actif.
type RefreshRecord struct {
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	FamilyID  string `json:"family_id"`
	CreatedAt int64  `json:"created_at"`
}

// RefreshStore gère le cycle de vie des refresh tokens (Redis).
type RefreshStore struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewRefreshStore(r *redis.Client, ttlHours int) *RefreshStore {
	return &RefreshStore{
		redis: r,
		ttl:   time.Duration(ttlHours) * time.Hour,
	}
}

func (s *RefreshStore) tokenKey(id string) string  { return "support:refresh:" + id }
func (s *RefreshStore) familyKey(id string) string { return "support:refresh_family:" + id }

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateRawToken() (string, error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Issue crée un nouveau refresh token pour une famille donnée. Si familyID est vide, en crée une nouvelle.
// Retourne (rawToken, familyID, expiresAt unix).
func (s *RefreshStore) Issue(ctx context.Context, userID, role, familyID string) (string, string, int64, error) {
	if familyID == "" {
		familyID = uuid.NewString()
	}
	raw, err := generateRawToken()
	if err != nil {
		return "", "", 0, err
	}
	id := hashToken(raw)
	rec := RefreshRecord{
		UserID:    userID,
		Role:      role,
		FamilyID:  familyID,
		CreatedAt: time.Now().Unix(),
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return "", "", 0, err
	}

	pipe := s.redis.TxPipeline()
	pipe.Set(ctx, s.tokenKey(id), payload, s.ttl)
	pipe.SAdd(ctx, s.familyKey(familyID), id)
	pipe.Expire(ctx, s.familyKey(familyID), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", "", 0, err
	}

	return raw, familyID, time.Now().Add(s.ttl).Unix(), nil
}

// Verify retourne le record si le token est valide. Si le token n'existe plus mais que l'ID
// est encore référencé dans une famille active → réutilisation détectée → famille révoquée.
func (s *RefreshStore) Verify(ctx context.Context, raw string) (*RefreshRecord, error) {
	id := hashToken(raw)
	val, err := s.redis.Get(ctx, s.tokenKey(id)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Détection de réutilisation : on cherche dans toutes les familles connues.
			s.tryDetectReuse(ctx, id)
			return nil, supportErrors.ErrRefreshInvalid
		}
		return nil, err
	}
	var rec RefreshRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// Rotate supprime l'ancien token et émet un nouveau dans la même famille.
func (s *RefreshStore) Rotate(ctx context.Context, oldRaw string, rec *RefreshRecord) (string, int64, error) {
	oldID := hashToken(oldRaw)
	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, s.tokenKey(oldID))
	pipe.SRem(ctx, s.familyKey(rec.FamilyID), oldID)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", 0, err
	}
	newRaw, _, exp, err := s.Issue(ctx, rec.UserID, rec.Role, rec.FamilyID)
	if err != nil {
		return "", 0, err
	}
	return newRaw, exp, nil
}

// Revoke supprime un seul refresh token (logout simple).
func (s *RefreshStore) Revoke(ctx context.Context, raw string) error {
	id := hashToken(raw)
	val, err := s.redis.Get(ctx, s.tokenKey(id)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return err
	}
	var rec RefreshRecord
	if err := json.Unmarshal([]byte(val), &rec); err == nil {
		s.redis.SRem(ctx, s.familyKey(rec.FamilyID), id)
	}
	return s.redis.Del(ctx, s.tokenKey(id)).Err()
}

// RevokeFamily invalide tous les refresh tokens d'une famille (compromission ou logout full devices).
func (s *RefreshStore) RevokeFamily(ctx context.Context, familyID string) error {
	ids, err := s.redis.SMembers(ctx, s.familyKey(familyID)).Result()
	if err != nil {
		return err
	}
	pipe := s.redis.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, s.tokenKey(id))
	}
	pipe.Del(ctx, s.familyKey(familyID))
	_, err = pipe.Exec(ctx)
	return err
}

// tryDetectReuse parcourt les familles existantes pour voir si l'ID rotaté est encore référencé.
// Si oui, la famille entière est révoquée (présomption de compromission).
func (s *RefreshStore) tryDetectReuse(ctx context.Context, tokenID string) {
	iter := s.redis.Scan(ctx, 0, "support:refresh_family:*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		exists, err := s.redis.SIsMember(ctx, key, tokenID).Result()
		if err == nil && exists {
			familyID := key[len("support:refresh_family:"):]
			_ = s.RevokeFamily(ctx, familyID)
			return
		}
	}
}
