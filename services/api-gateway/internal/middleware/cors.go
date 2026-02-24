package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig contient la configuration CORS
type CORSConfig struct {
	AllowedOrigins []string
	Environment    string
}

// CORS retourne un middleware HTTP qui gère les headers Cross-Origin.
// Reproduit le comportement du plugin cors-global de Kong.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowedOrigins := cfg.AllowedOrigins

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Déterminer si l'origine est autorisée
			allowed := false
			if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				allowed = true
			} else if origin != "" {
				for _, o := range allowedOrigins {
					if strings.EqualFold(o, origin) {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Vary", "Origin")
						allowed = true
						break
					}
				}
			}

			if !allowed {
				// Si pas d'origin ou origin non autorisée, on continue sans headers CORS
				next.ServeHTTP(w, r)
				return
			}

			// Headers CORS communs
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", strings.Join([]string{
				"X-Request-ID",
				"X-RateLimit-Limit-Minute",
				"X-RateLimit-Remaining-Minute",
				"X-RateLimit-Limit-Hour",
				"X-RateLimit-Remaining-Hour",
				"Content-Disposition",
			}, ", "))

			// Preflight (OPTIONS)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
					"Accept",
					"Accept-Language",
					"Accept-Encoding",
					"Authorization",
					"Content-Type",
					"Content-Length",
					"Origin",
					"X-Request-ID",
					"X-Requested-With",
					"X-Device-ID",
					"X-App-Version",
					"X-Platform",
					"X-Timezone",
					"Cache-Control",
					"Pragma",
				}, ", "))
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
