package middleware

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// AppIDHeader est le header HTTP que l'app mobile (ou le back-office support)
// doit envoyer à chaque requête protégée.
const AppIDHeader = "X-App-ID"

// AppID enforce la présence d'un header X-App-ID valide sur les routes
// protégées. Il y a 2 zones distinctes avec leurs propres whitelists :
//
//   - Routes **mobile** (Firebase JWT) : validé contre mobileIDs
//   - Routes **support** (JWT support) : validé contre supportIDs
//
// Les routes publiques (health, webhooks, /api/v1/auth/checkPhoneNumber, etc.)
// passent sans vérification.
//
// Les IDs sont des UUID v4 bundlés dans le build de l'app mobile et du
// back-office web. Valeur CSV pour permettre la rotation sans downtime
// (ajouter le nouvel ID, déployer, retirer l'ancien après la release).
//
// Limite connue : un UUID bundlé dans une APK peut être extrait par
// décompilation. Ce mécanisme est efficace contre les bots opportunistes
// mais pas contre un attaquant ciblé. Pour aller plus loin : Play Integrity
// (Android) et App Attest (iOS).
func AppID(
	mobileIDs map[string]struct{},
	supportIDs map[string]struct{},
	isProtected ProtectedRoutes,
	isSupportProtected ProtectedRoutes,
	logger *zap.Logger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			isMobileRoute := isProtected(path)
			isSupportRoute := isSupportProtected(path)

			// Routes publiques (health, webhooks, internes) → pas de check
			if !isMobileRoute && !isSupportRoute {
				next.ServeHTTP(w, r)
				return
			}

			id := strings.TrimSpace(r.Header.Get(AppIDHeader))
			if id == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing X-App-ID header")
				return
			}

			// Choisir la whitelist selon la zone de la route
			allowed := mobileIDs
			if isSupportRoute {
				allowed = supportIDs
			}
			if _, ok := allowed[id]; !ok {
				logger.Warn("invalid X-App-ID rejected",
					zap.String("path", path),
					zap.String("ip", extractIP(r)),
					zap.String("requestID", r.Header.Get(RequestIDHeader)),
				)
				writeJSONError(w, http.StatusUnauthorized, "invalid X-App-ID")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ParseAppIDs convertit une chaîne CSV d'App-IDs en set (map[string]struct{}).
// Ignore les entrées vides après trim. Facilite la rotation sans downtime :
// mettre "old-id,new-id" pendant la fenêtre de migration.
func ParseAppIDs(csv string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, id := range strings.Split(csv, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}
