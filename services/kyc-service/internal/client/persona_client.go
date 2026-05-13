package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"go.uber.org/zap"
)

const (
	personaBaseURL    = "https://withpersona.com/api/v1"
	personaAPIVersion = "2023-01-05"
	personaTimeout    = 30 * time.Second
)

// personaClientImpl est le client HTTP vers l'API Persona
type personaClientImpl struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewPersonaClient crée un client HTTP pour l'API Persona
func NewPersonaClient(apiKey string, logger *zap.Logger) PersonaClient {
	return &personaClientImpl{
		apiKey:  apiKey,
		baseURL: personaBaseURL,
		httpClient: &http.Client{
			Timeout: personaTimeout,
		},
		logger: logger,
	}
}

// parseSessionExpiresAt parse session-token-expires-at en RFC3339.
// Persona peut renvoyer une chaîne vide quand la session SDK n'est plus
// nécessaire (ex: docs déjà soumis via SubmitGovernmentID). On fallback
// à now+24h plutôt que d'annuler une requête que Persona a acceptée.
func (c *personaClientImpl) parseSessionExpiresAt(raw, inquiryID string) time.Time {
	if raw == "" {
		c.logger.Warn("persona: empty session-token-expires-at, using default 24h",
			zap.String("inquiryID", inquiryID),
		)
		return time.Now().UTC().Add(24 * time.Hour)
	}
	expiresAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.logger.Warn("persona: invalid session-token-expires-at, using default 24h",
			zap.String("inquiryID", inquiryID),
			zap.String("rawExpiresAt", raw),
			zap.Error(err),
		)
		return time.Now().UTC().Add(24 * time.Hour)
	}
	return expiresAt
}

// =============================================================================
// CreateInquiry
// =============================================================================

// createInquiryRequest est le body JSON envoyé à POST /inquiries.
// meta.auto-create-inquiry-session: true demande à Persona de générer un
// session token immédiatement et de le renvoyer dans meta.session-token.
type createInquiryRequest struct {
	Data createInquiryData `json:"data"`
	Meta createInquiryMeta `json:"meta"`
}

type createInquiryData struct {
	Attributes createInquiryAttributes `json:"attributes"`
}

type createInquiryAttributes struct {
	InquiryTemplateID string `json:"inquiry-template-id"`
	ReferenceID       string `json:"reference-id"`
}

type createInquiryMeta struct {
	AutoCreateInquirySession bool `json:"auto-create-inquiry-session"`
}

// personaInquiryResponse est la réponse JSON de l'API Persona pour une inquiry.
// Le session-token est en top-level meta (convention JSON:API), pas dans
// data.attributes — vrai pour POST /inquiries (avec auto-create-inquiry-session)
// et pour POST /inquiries/{id}/resume.
type personaInquiryResponse struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Status      string `json:"status"`
			TemplateID  string `json:"inquiry-template-id"`
			ReferenceID string `json:"reference-id"`
		} `json:"attributes"`
	} `json:"data"`
	Meta struct {
		SessionToken string `json:"session-token"`
		ExpiresAt    string `json:"session-token-expires-at"`
	} `json:"meta"`
}

func (c *personaClientImpl) CreateInquiry(ctx context.Context, templateID string, referenceID string) (*domain.PersonaInquiry, error) {
	c.logger.Debug("persona: CreateInquiry",
		zap.String("templateID", templateID),
		zap.String("referenceID", referenceID),
	)

	reqBody := createInquiryRequest{
		Data: createInquiryData{
			Attributes: createInquiryAttributes{
				InquiryTemplateID: templateID,
				ReferenceID:       referenceID,
			},
		},
		Meta: createInquiryMeta{
			AutoCreateInquirySession: true,
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("persona: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/inquiries", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("persona: create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("persona: CreateInquiry request failed", zap.Error(err))
		return nil, fmt.Errorf("persona: CreateInquiry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("persona: CreateInquiry unexpected status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("persona: CreateInquiry: unexpected status %d", resp.StatusCode)
	}

	var personaResp personaInquiryResponse
	if err := json.NewDecoder(resp.Body).Decode(&personaResp); err != nil {
		return nil, fmt.Errorf("persona: decode response: %w", err)
	}

	if personaResp.Meta.SessionToken == "" {
		c.logger.Error("persona: empty session-token in CreateInquiry response — vérifier la config du template ou l'API key",
			zap.String("inquiryID", personaResp.Data.ID),
		)
		return nil, fmt.Errorf("persona: CreateInquiry: empty session-token")
	}

	expiresAt := c.parseSessionExpiresAt(personaResp.Meta.ExpiresAt, personaResp.Data.ID)

	c.logger.Info("persona: inquiry created",
		zap.String("inquiryID", personaResp.Data.ID),
		zap.String("templateID", templateID),
	)

	return &domain.PersonaInquiry{
		InquiryID:    personaResp.Data.ID,
		TemplateID:   templateID,
		SessionToken: personaResp.Meta.SessionToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// =============================================================================
// ResumeInquiry
// =============================================================================

func (c *personaClientImpl) ResumeInquiry(ctx context.Context, inquiryID string) (*domain.PersonaSession, error) {
	c.logger.Debug("persona: ResumeInquiry", zap.String("inquiryID", inquiryID))

	url := fmt.Sprintf("%s/inquiries/%s/resume", c.baseURL, inquiryID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("persona: create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("persona: ResumeInquiry request failed", zap.Error(err))
		return nil, fmt.Errorf("persona: ResumeInquiry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("persona: ResumeInquiry unexpected status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("persona: ResumeInquiry: unexpected status %d", resp.StatusCode)
	}

	var personaResp personaInquiryResponse
	if err := json.NewDecoder(resp.Body).Decode(&personaResp); err != nil {
		return nil, fmt.Errorf("persona: decode response: %w", err)
	}

	if personaResp.Meta.SessionToken == "" {
		c.logger.Error("persona: empty session-token in ResumeInquiry response",
			zap.String("inquiryID", inquiryID),
		)
		return nil, fmt.Errorf("persona: ResumeInquiry: empty session-token")
	}

	expiresAt := c.parseSessionExpiresAt(personaResp.Meta.ExpiresAt, inquiryID)

	c.logger.Info("persona: inquiry resumed",
		zap.String("inquiryID", inquiryID),
	)

	return &domain.PersonaSession{
		SessionToken: personaResp.Meta.SessionToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// =============================================================================
// SubmitGovernmentID
// =============================================================================
// Soumet un document d'identité (recto + verso optionnel) à Persona via les
// URLs S3/MinIO de notre file-service. Évite la re-capture côté SDK Android.
//
// API Persona : POST /api/v1/government-id-documents
// https://docs.withpersona.com/reference/create-a-government-id

type submitGovernmentIDRequest struct {
	Data submitGovernmentIDData `json:"data"`
}

type submitGovernmentIDData struct {
	Attributes submitGovernmentIDAttributes `json:"attributes"`
}

type submitGovernmentIDAttributes struct {
	InquiryID     string `json:"inquiry-id"`
	Kind          string `json:"kind,omitempty"`
	FrontPhotoURL string `json:"front-photo-url"`
	BackPhotoURL  string `json:"back-photo-url,omitempty"`
}

func (c *personaClientImpl) SubmitGovernmentID(ctx context.Context, inquiryID, kind, frontURL, backURL string) error {
	c.logger.Debug("persona: SubmitGovernmentID",
		zap.String("inquiryID", inquiryID),
		zap.String("kind", kind),
		zap.Bool("hasBack", backURL != ""),
	)

	if inquiryID == "" {
		return fmt.Errorf("persona: SubmitGovernmentID: inquiryID required")
	}
	if frontURL == "" {
		return fmt.Errorf("persona: SubmitGovernmentID: frontURL required")
	}

	reqBody := submitGovernmentIDRequest{
		Data: submitGovernmentIDData{
			Attributes: submitGovernmentIDAttributes{
				InquiryID:     inquiryID,
				Kind:          kind,
				FrontPhotoURL: frontURL,
				BackPhotoURL:  backURL,
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("persona: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/government-id-documents", bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("persona: create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("persona: SubmitGovernmentID request failed", zap.Error(err))
		return fmt.Errorf("persona: SubmitGovernmentID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("persona: SubmitGovernmentID unexpected status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return fmt.Errorf("persona: SubmitGovernmentID: unexpected status %d", resp.StatusCode)
	}

	c.logger.Info("persona: government-id submitted",
		zap.String("inquiryID", inquiryID),
		zap.String("kind", kind),
	)
	return nil
}

// setHeaders applique les headers communs à toutes les requêtes Persona
func (c *personaClientImpl) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Persona-Version", personaAPIVersion)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}
