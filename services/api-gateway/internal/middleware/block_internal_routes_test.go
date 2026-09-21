package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/middleware"
)

// serveThroughBlock envoie une requête à travers BlockInternalRoutes et indique
// si elle a atteint le handler suivant.
func serveThroughBlock(t *testing.T, method, target string) (reached bool, status int) {
	t.Helper()
	handler := middleware.BlockInternalRoutes(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return reached, rec.Code
}

func TestBlockInternalRoutes(t *testing.T) {
	blocked := []string{
		"/api/v1/payment/internal/releasePayment",
		"/api/v1/payment/internal/requestRefund",
		"/api/v1/booking/internal/cancelBookingsForTrip",
		"/api/v1/trip/internal/updateAvailableSeats",
		// Tentatives de contournement
		"//api/v1/payment/internal/releasePayment",
		"/api/v1/payment/./internal/releasePayment",
		"/api/v1/booking/x/../internal/failPayment",
		"/api/v1/payment/INTERNAL/releasePayment",
		"/api/v1/payment/internal%2FreleasePayment",
		"/api/v1/payment/internal/",
	}
	for _, target := range blocked {
		t.Run("bloquée "+target, func(t *testing.T) {
			reached, status := serveThroughBlock(t, http.MethodPost, target)
			assert.False(t, reached, "la requête ne doit pas atteindre le mux")
			assert.Equal(t, http.StatusNotFound, status)
		})
	}

	allowed := []string{
		"/api/v1/payment/createPayment",
		"/api/v1/payment/webhooks/fedapay",
		"/api/v1/booking/getDriverBookings",
		"/api/v1/trip/passenger/getTripDetails",
		"/api/v1/internalNotes", // "internal" en préfixe d'un segment, pas un segment entier
	}
	for _, target := range allowed {
		t.Run("autorisée "+target, func(t *testing.T) {
			reached, status := serveThroughBlock(t, http.MethodGet, target)
			assert.True(t, reached)
			assert.Equal(t, http.StatusOK, status)
		})
	}
}

// TestBlockInternalRoutes_CoversEveryProtoBinding relit les protos de la gateway :
// toute route HTTP dont le chemin contient un segment `internal` doit être bloquée,
// et aucune autre ne doit l'être. Une RPC interne ajoutée plus tard avec une
// annotation google.api.http est ainsi couverte sans modifier ce test.
func TestBlockInternalRoutes_CoversEveryProtoBinding(t *testing.T) {
	protos, err := filepath.Glob(filepath.Join("..", "..", "proto", "*.proto"))
	require.NoError(t, err)
	require.NotEmpty(t, protos, "aucun proto trouvé : le test ne vérifierait rien")

	binding := regexp.MustCompile(`\b(get|post|put|patch|delete)\s*:\s*"([^"]+)"`)
	internalCount := 0
	for _, file := range protos {
		content, err := os.ReadFile(file) //nolint:gosec // chemins issus d'un glob sur les protos du dépôt
		require.NoError(t, err)
		for _, m := range binding.FindAllStringSubmatch(string(content), -1) {
			method, route := strings.ToUpper(m[1]), m[2]
			isInternal := strings.Contains(route, "/internal/")
			if isInternal {
				internalCount++
			}
			reached, _ := serveThroughBlock(t, method, route)
			assert.Equal(t, !isInternal, reached, "%s %s (%s)", method, route, filepath.Base(file))
		}
	}
	assert.Positive(t, internalCount, "les protos déclarent des routes internes : le test doit en trouver")
}
