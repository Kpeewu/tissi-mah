package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newTestPersonaClient construit un personaClientImpl pointant vers un httptest.Server
// pour intercepter les requêtes sortantes. baseURL = server.URL (sans suffixe).
func newTestPersonaClient(server *httptest.Server, apiKey string) *personaClientImpl {
	return &personaClientImpl{
		apiKey:     apiKey,
		baseURL:    server.URL,
		httpClient: server.Client(),
		logger:     zap.NewNop(),
	}
}

func TestCreateInquiry(t *testing.T) {
	t.Run("succès - meta.auto-create-inquiry-session true et token parsé depuis meta", func(t *testing.T) {
		var capturedPath, capturedAuth, capturedVersion string
		var capturedBody []byte

		expiresAt := "2026-05-09T16:54:09Z"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			capturedAuth = r.Header.Get("Authorization")
			capturedVersion = r.Header.Get("Persona-Version")
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"data":{
					"id":"inq_abc",
					"attributes":{
						"status":"created",
						"inquiry-template-id":"itmpl_xyz",
						"reference-id":"user_123"
					}
				},
				"meta":{
					"session-token":"sess_top_secret",
					"session-token-expires-at":"` + expiresAt + `"
				}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		inquiry, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.NoError(t, err)

		assert.Equal(t, "/inquiries", capturedPath)
		assert.Equal(t, "Bearer test-api-key", capturedAuth)
		assert.Equal(t, personaAPIVersion, capturedVersion)

		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		meta, ok := body["meta"].(map[string]any)
		require.True(t, ok, "request body must contain meta block")
		assert.Equal(t, true, meta["auto-create-inquiry-session"])
		attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
		assert.Equal(t, "itmpl_xyz", attrs["inquiry-template-id"])
		assert.Equal(t, "user_123", attrs["reference-id"])

		assert.Equal(t, "inq_abc", inquiry.InquiryID)
		assert.Equal(t, "itmpl_xyz", inquiry.TemplateID)
		assert.Equal(t, "sess_top_secret", inquiry.SessionToken)
		expected, _ := time.Parse(time.RFC3339, expiresAt)
		assert.True(t, inquiry.ExpiresAt.Equal(expected))
	})

	t.Run("erreur - meta.session-token vide", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_abc","attributes":{}},
				"meta":{"session-token":"","session-token-expires-at":"2026-05-09T16:54:09Z"}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		_, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty session-token")
	})

	t.Run("erreur - meta absent (ancien comportement Persona sans auto-create)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"id":"inq_abc","attributes":{}}}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		_, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty session-token")
	})

	t.Run("fallback - session-token-expires-at non parsable utilise +24h", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_abc","attributes":{}},
				"meta":{"session-token":"sess_top_secret","session-token-expires-at":"not-a-date"}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		inquiry, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.NoError(t, err)
		require.NotNil(t, inquiry)
		assert.Equal(t, "sess_top_secret", inquiry.SessionToken)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), inquiry.ExpiresAt, time.Minute)
	})

	t.Run("fallback - session-token-expires-at vide utilise +24h", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_AqzrEzT4LoG4wBApi2SLd1CHsxJAeX","attributes":{}},
				"meta":{"session-token":"sess_top_secret","session-token-expires-at":""}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		inquiry, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.NoError(t, err)
		require.NotNil(t, inquiry)
		assert.Equal(t, "inq_AqzrEzT4LoG4wBApi2SLd1CHsxJAeX", inquiry.InquiryID)
		assert.Equal(t, "sess_top_secret", inquiry.SessionToken)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), inquiry.ExpiresAt, time.Minute)
	})

	t.Run("erreur - HTTP 401 retourné par Persona", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"title":"unauthorized"}]}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		_, err := c.CreateInquiry(context.Background(), "itmpl_xyz", "user_123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 401")
	})
}

func TestResumeInquiry(t *testing.T) {
	t.Run("succès - token parsé depuis meta", func(t *testing.T) {
		var capturedMethod, capturedPath string
		expiresAt := "2026-05-09T16:54:09Z"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_abc","attributes":{"status":"pending"}},
				"meta":{"session-token":"sess_resumed","session-token-expires-at":"` + expiresAt + `"}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		session, err := c.ResumeInquiry(context.Background(), "inq_abc")
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/inquiries/inq_abc/resume", capturedPath)
		assert.Equal(t, "sess_resumed", session.SessionToken)
		expected, _ := time.Parse(time.RFC3339, expiresAt)
		assert.True(t, session.ExpiresAt.Equal(expected))
	})

	t.Run("erreur - meta.session-token vide", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"id":"inq_abc","attributes":{}},"meta":{"session-token":""}}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		_, err := c.ResumeInquiry(context.Background(), "inq_abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty session-token")
	})

	t.Run("erreur - HTTP 410 inquiry non resumable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGone)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		_, err := c.ResumeInquiry(context.Background(), "inq_abc")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 410")
	})

	t.Run("fallback - session-token-expires-at non parsable utilise +24h", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_abc","attributes":{}},
				"meta":{"session-token":"sess_resumed","session-token-expires-at":"not-a-date"}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		session, err := c.ResumeInquiry(context.Background(), "inq_abc")
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "sess_resumed", session.SessionToken)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), session.ExpiresAt, time.Minute)
	})

	t.Run("fallback - session-token-expires-at vide utilise +24h", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data":{"id":"inq_abc","attributes":{}},
				"meta":{"session-token":"sess_resumed","session-token-expires-at":""}
			}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		session, err := c.ResumeInquiry(context.Background(), "inq_abc")
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "sess_resumed", session.SessionToken)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), session.ExpiresAt, time.Minute)
	})
}
