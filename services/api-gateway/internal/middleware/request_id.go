package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

// RequestIDHeader est le nom du header HTTP portant l'identifiant de requête.
const RequestIDHeader = "X-Request-ID"

// RequestID génère un UUID v4 si le header X-Request-ID est absent de la
// requête entrante, puis le propage :
//   - sur la request (lu ensuite par le metadataAnnotator gRPC de mux.go pour
//     transiter jusqu'aux services downstream)
//   - sur la response (visible par le client pour le debug)
//
// Si le client fournit déjà un X-Request-ID, il est conservé tel quel
// (confiance : le client peut pré-générer son ID pour corrélation locale).
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(RequestIDHeader)
			if rid == "" {
				rid = uuid.NewString()
				r.Header.Set(RequestIDHeader, rid)
			}
			w.Header().Set(RequestIDHeader, rid)
			next.ServeHTTP(w, r)
		})
	}
}
