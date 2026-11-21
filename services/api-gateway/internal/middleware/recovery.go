package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// PanicRecovery intercepte les panics dans les handlers HTTP, log la stack
// trace complète et renvoie un 500 JSON proprement au lieu de planter le
// process. Doit être le premier middleware dans la chaîne.
func PanicRecovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered in api-gateway",
						zap.Any("panic", rec),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("requestID", r.Header.Get(RequestIDHeader)),
						zap.ByteString("stack", debug.Stack()),
					)
					writeJSONError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
