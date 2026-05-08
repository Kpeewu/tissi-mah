package middleware

import "net/http"

// SecurityHeaders ajoute les en-têtes HTTP défensifs standard sur toutes
// les responses. Référence : OWASP Secure Headers Project.
//
// HSTS est conditionnel (enableHSTS=true uniquement en prod/staging où TLS
// est terminé en amont par nginx). En vps-dev le TLS est géré différemment.
//
// Note : pas de Content-Security-Policy — l'api-gateway ne sert pas de HTML,
// une CSP serait une fausse confiance et risquerait de bloquer les clients.
func SecurityHeaders(enableHSTS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Empêche le MIME-type sniffing (attaques drive-by)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			// Empêche l'intégration dans une iframe (clickjacking)
			w.Header().Set("X-Frame-Options", "DENY")
			// Contrôle les informations de referrer transmises aux tiers
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Désactivé intentionnellement (XSS auditor legacy; la CSP est la bonne approche)
			w.Header().Set("X-XSS-Protection", "0")
			if enableHSTS {
				// max-age 1 an; inclut tous les sous-domaines
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
