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

func TestSubmitGovernmentID(t *testing.T) {
	t.Run("succès - 201 Created avec front + back", func(t *testing.T) {
		var capturedMethod, capturedPath, capturedAuth, capturedVersion, capturedContentType string
		var capturedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedPath = r.URL.Path
			capturedAuth = r.Header.Get("Authorization")
			capturedVersion = r.Header.Get("Persona-Version")
			capturedContentType = r.Header.Get("Content-Type")
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"type":"government-id","id":"gid_xxx"}}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")

		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "identification_card",
			"https://minio.local/bucket/front.jpg",
			"https://minio.local/bucket/back.jpg",
		)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/government-ids", capturedPath)
		assert.Equal(t, "Bearer test-api-key", capturedAuth)
		assert.Equal(t, personaAPIVersion, capturedVersion)
		assert.Equal(t, "application/json", capturedContentType)

		// Vérifier le payload JSON:API
		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		data, ok := body["data"].(map[string]any)
		require.True(t, ok, "data field missing")
		assert.Equal(t, "government-id", data["type"])

		attrs, ok := data["attributes"].(map[string]any)
		require.True(t, ok, "attributes field missing")
		assert.Equal(t, "identification_card", attrs["kind"])
		assert.Equal(t, "https://minio.local/bucket/front.jpg", attrs["front-photo-url"])
		assert.Equal(t, "https://minio.local/bucket/back.jpg", attrs["back-photo-url"])

		rels, ok := data["relationships"].(map[string]any)
		require.True(t, ok, "relationships field missing")
		inqRel, ok := rels["inquiry"].(map[string]any)
		require.True(t, ok, "relationships.inquiry missing")
		inqData, ok := inqRel["data"].(map[string]any)
		require.True(t, ok, "relationships.inquiry.data missing")
		assert.Equal(t, "inquiry", inqData["type"])
		assert.Equal(t, "inq_abc", inqData["id"])
	})

	t.Run("succès - 200 OK accepté aussi", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "passport",
			"https://minio.local/bucket/front.jpg", "")
		require.NoError(t, err)
	})

	t.Run("succès - sans backURL, back-photo-url absent du JSON", func(t *testing.T) {
		var capturedBody []byte

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "passport",
			"https://minio.local/bucket/front.jpg", "")
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
		_, hasBack := attrs["back-photo-url"]
		assert.False(t, hasBack, "back-photo-url should be omitted when empty")
	})

	t.Run("erreur - inquiryID vide, aucun appel HTTP", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "", "identification_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inquiryID required")
		assert.False(t, called)
	})

	t.Run("erreur - frontURL vide, aucun appel HTTP", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "identification_card", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "frontURL required")
		assert.False(t, called)
	})

	t.Run("erreur - HTTP 400 retourné par Persona", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"title":"invalid front-photo-url"}]}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "identification_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 400")
	})

	t.Run("erreur - HTTP 500 retourné par Persona", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "identification_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 500")
	})

	t.Run("erreur - réseau injoignable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "identification_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SubmitGovernmentID")
	})

	t.Run("succès - kind vide envoyé sans le champ (omitempty)", func(t *testing.T) {
		var capturedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "",
			"https://minio.local/bucket/front.jpg", "")
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		attrs := body["data"].(map[string]any)["attributes"].(map[string]any)
		_, hasKind := attrs["kind"]
		assert.False(t, hasKind, "kind should be omitted when empty")
	})
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
