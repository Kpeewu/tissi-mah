package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	firebaseValidator "github.com/Kpeewu/tissi-mah/services/api-gateway/pkg/firebase"
	"go.uber.org/zap"
)

// FirebaseUIDHeader est le header HTTP utilisé pour transmettre le Firebase UID
// aux services internes via grpc-gateway (converti en metadata gRPC).
const FirebaseUIDHeader = "x-firebase-uid"

// ProtectedRoutes est une fonction qui retourne true si la route requiert un JWT
type ProtectedRoutes func(path string) bool

// JWTFirebase retourne un middleware HTTP qui :
//  1. Vérifie si la route est protégée (nécessite un JWT)
//  2. Extrait le Bearer token du header Authorization
//  3. Valide le token avec Firebase Admin SDK
//  4. Injecte le Firebase UID dans le header x-firebase-uid
//     (grpc-gateway le transmettra en metadata gRPC aux services)
func JWTFirebase(validator *firebaseValidator.JWTValidator, isProtected ProtectedRoutes, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Les routes non protégées passent sans JWT
			if !isProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Extraire le Bearer token du header Authorization
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

			// Valider le token Firebase et extraire le UID
			uid, err := validator.VerifyToken(r.Context(), token)
			if err != nil {
				logger.Warn("firebase token validation failed",
					zap.String("path", r.URL.Path),
					zap.Error(err),
				)
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Injecter le Firebase UID pour grpc-gateway
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
