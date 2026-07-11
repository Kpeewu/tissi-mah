package middleware

import (
	"net/http"

	grpcutil "github.com/Kpeewu/tissi-mah/pkg/grpcutil"
)

// StripInboundAuthHeaders supprime, en bordure de gateway, les headers d'identité
// que seul l'api-gateway a le droit de positionner à partir d'un token validé.
//
// Sans ce strip, un client peut envoyer `x-firebase-uid: <uid-victime>` : le
// metadataAnnotator du mux le recopie en metadata gRPC et le signe avec
// INTERNAL_HMAC_SECRET → les services internes voient une signature valide et
// font confiance à l'UID forgé (usurpation d'identité). Le middleware doit
// s'exécuter AVANT JWTFirebase / JWTSupport (qui reposent ces headers après
// validation) et donc avant l'annotator.
//
// x-request-id n'est pas touché : il est légitimement fourni/propagé par le
// middleware RequestID.
func StripInboundAuthHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(FirebaseUIDHeader)
		r.Header.Del(SupportUIDHeader)
		r.Header.Del(SupportRoleHeader)
		r.Header.Del(grpcutil.MetadataUIDSig)
		next.ServeHTTP(w, r)
	})
}
