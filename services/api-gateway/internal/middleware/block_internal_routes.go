package middleware

import (
	"net/http"
	"path"
	"strings"
)

// internalSegment est le segment de chemin qui marque les RPC réservées aux
// appels entre services (ex. /api/v1/payment/internal/releasePayment).
const internalSegment = "internal"

// BlockInternalRoutes refuse, en bordure de gateway, toute requête HTTP visant une
// route `/internal/`.
//
// Ces RPC (versement au chauffeur, remboursement, annulation en masse des
// réservations, mise à jour des places d'un trajet…) portent une annotation
// `google.api.http` dans les protos, donc grpc-gateway les expose. Mais elles ne
// figurent dans aucune liste de routes protégées : sans ce middleware, elles
// étaient appelables par n'importe qui, sans token ni X-App-ID. Les services se
// les appellent entre eux directement en gRPC ; aucun client HTTP légitime ne
// passe par la gateway pour les atteindre.
//
// La réponse est un 404 et non un 403, pour ne pas confirmer l'existence de la
// route. Le chemin passe par path.Clean puis est comparé segment par segment,
// sur la forme décodée comme sur la forme brute, pour que `//`, `./`, `..` ou
// un encodage `%2F` ne permettent pas de contourner le filtre.
func BlockInternalRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasInternalSegment(r.URL.Path) || hasInternalSegment(r.URL.RawPath) {
			writeJSONError(w, http.StatusNotFound, "Not Found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hasInternalSegment(p string) bool {
	if p == "" {
		return false
	}
	for _, segment := range strings.Split(path.Clean(p), "/") {
		if strings.EqualFold(segment, internalSegment) {
			return true
		}
	}
	return false
}
