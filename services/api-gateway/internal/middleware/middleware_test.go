package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/api-gateway/internal/middleware"
)

// ============================================================
// PanicRecovery
// ============================================================

func TestPanicRecovery(t *testing.T) {
	logger := zap.NewNop()
	handler := middleware.PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal server error")
}

func TestPanicRecovery_NoPanic(t *testing.T) {
	logger := zap.NewNop()
	handler := middleware.PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ============================================================
// RequestID
// ============================================================

func TestRequestID_Generated(t *testing.T) {
	var capturedID string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Header.Get(middleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, capturedID, "UUID should be generated when header absent")
	assert.Equal(t, capturedID, rec.Header().Get(middleware.RequestIDHeader), "response header should match")
}

func TestRequestID_PreservedFromClient(t *testing.T) {
	clientID := "my-client-uuid-1234"
	var capturedID string
	handler := middleware.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.Header.Get(middleware.RequestIDHeader)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(middleware.RequestIDHeader, clientID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, clientID, capturedID, "client-provided ID should be preserved")
	assert.Equal(t, clientID, rec.Header().Get(middleware.RequestIDHeader))
}

// ============================================================
// SecurityHeaders
// ============================================================

func TestSecurityHeaders_WithoutHSTS(t *testing.T) {
	handler := middleware.SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.Equal(t, "0", rec.Header().Get("X-XSS-Protection"))
	assert.Empty(t, rec.Header().Get("Strict-Transport-Security"), "HSTS disabled")
}

func TestSecurityHeaders_WithHSTS(t *testing.T) {
	handler := middleware.SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
}

// ============================================================
// BodySizeLimit
// ============================================================

func TestBodySizeLimit_UnderLimit(t *testing.T) {
	handler := middleware.BodySizeLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body) //nolint:errcheck
	}))

	req := httptest.NewRequest("POST", "/test", strings.NewReader("small"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// ============================================================
// AppID
// ============================================================

func setupAppID() http.Handler {
	logger := zap.NewNop()
	mobileIDs := middleware.ParseAppIDs("mobile-uuid-1,mobile-uuid-2")
	supportIDs := middleware.ParseAppIDs("support-uuid-1")
	return middleware.AppID(
		mobileIDs,
		supportIDs,
		func(path string) bool { return path == "/api/v1/user/me" },
		func(path string) bool { return path == "/api/v1/support/me" },
		logger,
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestAppID_MissingHeader_ProtectedRoute(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing X-App-ID")
}

func TestAppID_InvalidID_ProtectedRoute(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	req.Header.Set(middleware.AppIDHeader, "bogus-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid X-App-ID")
}

func TestAppID_ValidMobileID(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	req.Header.Set(middleware.AppIDHeader, "mobile-uuid-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAppID_ValidMobileID_SecondInCSV(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	req.Header.Set(middleware.AppIDHeader, "mobile-uuid-2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAppID_MobileID_OnSupportRoute_Rejected(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/support/me", nil)
	req.Header.Set(middleware.AppIDHeader, "mobile-uuid-1") // mobile ID sur route support → rejeté
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAppID_SupportID_OnSupportRoute_OK(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/support/me", nil)
	req.Header.Set(middleware.AppIDHeader, "support-uuid-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAppID_PublicRoute_Passthrough(t *testing.T) {
	handler := setupAppID()
	req := httptest.NewRequest("GET", "/api/v1/auth/health", nil) // route publique, pas dans isProtected
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestParseAppIDs_CSV(t *testing.T) {
	ids := middleware.ParseAppIDs("id-1, id-2 , id-3,")
	require.Len(t, ids, 3)
	_, ok1 := ids["id-1"]
	_, ok2 := ids["id-2"]
	_, ok3 := ids["id-3"]
	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.True(t, ok3)
}
