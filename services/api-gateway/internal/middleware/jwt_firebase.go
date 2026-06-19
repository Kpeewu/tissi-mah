package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	firebaseValidator "github.com/Kpeewu/tissi-mah/services/api-gateway/pkg/firebase"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// FirebaseUIDHeader est le header HTTP utilisé pour transmettre le Firebase UID
// aux services internes via grpc-gateway (converti en metadata gRPC).
const FirebaseUIDHeader = "x-firebase-uid"

// suspensionKeyPrefix est le préfixe des clés Redis écrites par auth-service lors d'une suspension.
const suspensionKeyPrefix = "suspended:"

// ProtectedRoutes est une fonction qui retourne true si la route requiert un JWT
type ProtectedRoutes func(path string) bool

// JWTFirebase retourne un middleware HTTP qui :
//  1. Vérifie si la route est protégée (nécessite un JWT)
//  2. Extrait le Bearer token du header Authorization
//  3. Valide le token avec Firebase Admin SDK
//  4. Vérifie que le compte n'est pas suspendu/banni (via Redis si configuré)
//  5. Injecte le Firebase UID dans le header x-firebase-uid
func JWTFirebase(validator *firebaseValidator.JWTValidator, isProtected ProtectedRoutes, suspensionRedis *redis.Client, logger *zap.Logger) func(http.Handler) http.Handler {
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

			uid, err := validator.VerifyToken(r.Context(), token)
			if err != nil {
				logger.Warn("firebase token validation failed",
					zap.String("path", r.URL.Path),
					zap.Error(err),
				)
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Vérifier la suspension via Redis (écrit par auth-service à chaque suspension)
			if suspensionRedis != nil {
				checkCtx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
				val, redisErr := suspensionRedis.Get(checkCtx, suspensionKeyPrefix+uid).Result()
				cancel()
				if redisErr == nil {
					// Clé présente → compte suspendu ou banni
					if val == "banned" {
						logger.Warn("blocked request: account banned",
							zap.String("uid", uid),
							zap.String("path", r.URL.Path),
						)
						writeJSONError(w, http.StatusForbidden, "account_banned")
					} else {
						logger.Warn("blocked request: account suspended",
							zap.String("uid", uid),
							zap.String("path", r.URL.Path),
						)
						writeJSONError(w, http.StatusForbidden, "account_suspended")
					}
					return
				}
				// redis.Nil = clé absente (cas normal), autres erreurs = Redis down → fail-open
			}

			r.Header.Set(FirebaseUIDHeader, uid)
			logger.Debug("authenticated request",
				zap.String("uid", uid),
				zap.String("path", r.URL.Path),
			)
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError écrit une réponse d'erreur JSON
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"message": message}) //nolint:errcheck
}
