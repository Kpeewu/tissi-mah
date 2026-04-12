package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims représente les claims de l'access token support-service.
type Claims struct {
	Role               string `json:"role"`
	MustChangePassword bool   `json:"mcp"`
	jwt.RegisteredClaims
}

// JWTSigner signe et vérifie les access tokens HS256.
type JWTSigner struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

func NewJWTSigner(secret string, ttlHours int) *JWTSigner {
	return &JWTSigner{
		secret: []byte(secret),
		ttl:    time.Duration(ttlHours) * time.Hour,
		issuer: "support-service",
	}
}

// Sign retourne (token, expiresAt unix).
func (s *JWTSigner) Sign(userID, role string, mustChange bool) (string, int64, error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	c := Claims{
		Role:               role,
		MustChangePassword: mustChange,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, exp.Unix(), nil
}

// Verify parse et valide le token, retourne les claims.
func (s *JWTSigner) Verify(raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
