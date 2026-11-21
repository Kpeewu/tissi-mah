package middleware

import "net/http"

// DefaultBodyMaxBytes est la limite par défaut (10 Mo).
const DefaultBodyMaxBytes int64 = 10 << 20 // 10 485 760 bytes

// BodySizeLimit limite la taille du body HTTP pour prévenir les attaques
// DoS par body surdimensionné. Les requêtes dépassant la limite reçoivent
// un 413 Request Entity Too Large.
//
// Implémentation : http.MaxBytesReader — la lecture s'arrête et renvoie une
// erreur dès que la limite est dépassée ; les octets excédentaires ne
// transitent pas en mémoire.
func BodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
