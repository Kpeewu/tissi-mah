package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
			_, _ = w.Write([]byte(`{"data":{"id":"gid_xxx"}}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")

		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "id_card",
			"https://minio.local/bucket/front.jpg",
			"https://minio.local/bucket/back.jpg",
		)
		require.NoError(t, err)

		assert.Equal(t, http.MethodPost, capturedMethod)
		assert.Equal(t, "/government-id-documents", capturedPath)
		assert.Equal(t, "Bearer test-api-key", capturedAuth)
		assert.Equal(t, personaAPIVersion, capturedVersion)
		assert.Equal(t, "application/json", capturedContentType)

		// Vérifier le format JSON-API du body
		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		data, ok := body["data"].(map[string]any)
		require.True(t, ok, "data field missing")
		attrs, ok := data["attributes"].(map[string]any)
		require.True(t, ok, "attributes field missing")
		assert.Equal(t, "inq_abc", attrs["inquiry-id"])
		assert.Equal(t, "id_card", attrs["kind"])
		assert.Equal(t, "https://minio.local/bucket/front.jpg", attrs["front-photo-url"])
		assert.Equal(t, "https://minio.local/bucket/back.jpg", attrs["back-photo-url"])
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

	t.Run("succès - sans backURL, le champ back-photo-url est omis du JSON", func(t *testing.T) {
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
		assert.Equal(t, "https://minio.local/bucket/front.jpg", attrs["front-photo-url"])
	})

	t.Run("erreur - inquiryID vide, aucun appel HTTP", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "", "id_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inquiryID required")
		assert.False(t, called, "no HTTP request should be made")
	})

	t.Run("erreur - frontURL vide, aucun appel HTTP", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "id_card", "", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "frontURL required")
		assert.False(t, called, "no HTTP request should be made")
	})

	t.Run("erreur - HTTP 400 retourné par Persona", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"title":"invalid front-photo-url"}]}`))
		}))
		defer server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "id_card",
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
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "id_card",
			"https://minio.local/bucket/front.jpg", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status 500")
	})

	t.Run("erreur - réseau injoignable", func(t *testing.T) {
		// Server fermé immédiatement → la requête échouera au niveau transport
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		server.Close()

		c := newTestPersonaClient(server, "test-api-key")
		err := c.SubmitGovernmentID(context.Background(), "inq_abc", "id_card",
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
