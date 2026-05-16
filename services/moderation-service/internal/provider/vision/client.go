package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	"go.uber.org/zap"
)

const (
	visionURL   = "https://vision.googleapis.com/v1/images:annotate"
	httpTimeout = 5 * time.Second
)

// SafeSearch likelihood scores (Google Vision API).
// VERY_LIKELY et LIKELY déclenchent un BLOCKED.
var blockedLikelihoods = map[string]bool{
	"LIKELY":      true,
	"VERY_LIKELY": true,
}

type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

func NewClient(googleAppCreds string, logger *zap.Logger) *Client {
	// Positionner GOOGLE_APPLICATION_CREDENTIALS si fourni comme chemin de fichier
	if googleAppCreds != "" && googleAppCreds[0] == '/' {
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", googleAppCreds) //nolint:errcheck
	}
	// Pour la simplicité, on utilise l'API key si disponible (chemin alternatif)
	return &Client{
		apiKey:     googleAppCreds,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
	}
}

type annotateRequest struct {
	Requests []imageRequest `json:"requests"`
}

type imageRequest struct {
	Image    map[string]string `json:"image"`
	Features []map[string]any  `json:"features"`
}

type annotateResponse struct {
	Responses []struct {
		SafeSearchAnnotation struct {
			Adult    string `json:"adult"`
			Violence string `json:"violence"`
			Racy     string `json:"racy"`
			Medical  string `json:"medical"`
		} `json:"safeSearchAnnotation"`
	} `json:"responses"`
}

// ModerateImage appelle Google Cloud Vision SafeSearch et retourne un score + catégorie.
func (c *Client) ModerateImage(ctx context.Context, imageBytes []byte, _ string) (float32, domain.Category, error) {
	if c.apiKey == "" {
		return 0, domain.CategoryNone, nil
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)
	body := annotateRequest{
		Requests: []imageRequest{
			{
				Image: map[string]string{"content": encoded},
				Features: []map[string]any{
					{"type": "SAFE_SEARCH_DETECTION", "maxResults": 1},
				},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("vision: marshal: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", visionURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("vision: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("vision API unavailable", zap.Error(err))
		return 0, domain.CategoryNone, nil // fail-open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("vision API returned non-200", zap.Int("status", resp.StatusCode))
		return 0, domain.CategoryNone, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, domain.CategoryNone, nil
	}

	var result annotateResponse
	if err := json.Unmarshal(data, &result); err != nil || len(result.Responses) == 0 {
		return 0, domain.CategoryNone, nil
	}

	ssa := result.Responses[0].SafeSearchAnnotation

	if blockedLikelihoods[ssa.Adult] {
		return 0.95, domain.CategoryNSFW, nil
	}
	if blockedLikelihoods[ssa.Violence] {
		return 0.95, domain.CategoryGore, nil
	}
	// RACY = fortement suggestif mais pas explicitement obscène → FLAGGED
	if ssa.Racy == "VERY_LIKELY" {
		return 0.75, domain.CategoryNSFW, nil
	}

	return 0, domain.CategoryNone, nil
}
