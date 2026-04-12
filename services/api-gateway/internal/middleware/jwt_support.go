package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// SupportUIDHeader / SupportRoleHeader sont les headers HTTP transmis aux services internes
// (convertis en metadata gRPC par grpc-gateway).
const (
	SupportUIDHeader  = "x-support-uid"
	SupportRoleHeader = "x-support-role"
)

// supportClaims doit refléter celles signées par support-service.
type supportClaims struct {
	Role               string `json:"role"`
	MustChangePassword bool   `json:"mcp"`
	jwt.RegisteredClaims
}

// JWTSupport vérifie le JWT support-service (HS256) sur les routes /api/v1/support/*
// listées dans SupportProtectedRoutes. Le secret partagé doit matcher celui de support-service.
func JWTSupport(secret string, isProtected ProtectedRoutes, logger *zap.Logger) func(http.Handler) http.Handler {
	secretBytes := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
				return
			}

			parsed, err := jwt.ParseWithClaims(token, &supportClaims{}, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return secretBytes, nil
			})
			if err != nil || !parsed.Valid {
				logger.Warn("support jwt validation failed",
					zap.String("path", r.URL.Path),
					zap.Error(err),
				)
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired support token")
				return
			}
			c, ok := parsed.Claims.(*supportClaims)
			if !ok || c.Subject == "" {
				writeJSONError(w, http.StatusUnauthorized, "invalid support token claims")
				return
			}

			r.Header.Set(SupportUIDHeader, c.Subject)
			r.Header.Set(SupportRoleHeader, c.Role)
			next.ServeHTTP(w, r)
		})
	}
}
