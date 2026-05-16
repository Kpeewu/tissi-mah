package perspective

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	"go.uber.org/zap"
)

const (
	perspectiveURL = "https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze"
	httpTimeout    = 3 * time.Second
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

func NewClient(apiKey string, logger *zap.Logger) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
	}
}

type analyzeRequest struct {
	Comment          map[string]string            `json:"comment"`
	RequestedAttributes map[string]map[string]any `json:"requestedAttributes"`
	Languages        []string                     `json:"languages"`
}

type analyzeResponse struct {
	AttributeScores map[string]struct {
		SummaryScore struct {
			Value float32 `json:"value"`
		} `json:"summaryScore"`
	} `json:"attributeScores"`
}

// ModerateText appelle l'API Google Perspective et retourne le score de toxicité le plus élevé.
func (c *Client) ModerateText(ctx context.Context, text string) (float32, domain.Category, error) {
	if c.apiKey == "" {
		return 0, domain.CategoryNone, nil
	}

	body := analyzeRequest{
		Comment:  map[string]string{"text": text},
		Languages: []string{"fr", "en", "de", "it", "es"},
		RequestedAttributes: map[string]map[string]any{
			"TOXICITY":         {},
			"SEVERE_TOXICITY":  {},
			"INSULT":           {},
			"PROFANITY":        {},
			"IDENTITY_ATTACK":  {},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("perspective: marshal: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", perspectiveURL, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("perspective: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("perspective API unavailable", zap.Error(err))
		return 0, domain.CategoryNone, nil // fail-open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("perspective API returned non-200", zap.Int("status", resp.StatusCode))
		return 0, domain.CategoryNone, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, domain.CategoryNone, nil
	}

	var result analyzeResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, domain.CategoryNone, nil
	}

	// Sélectionner le score le plus élevé et la catégorie associée
	maxScore := float32(0)
	category := domain.CategoryNone
	categoryMap := map[string]domain.Category{
		"TOXICITY":        domain.CategoryObscene,
		"SEVERE_TOXICITY": domain.CategoryHateSpeech,
		"INSULT":          domain.CategoryInsults,
		"PROFANITY":       domain.CategoryObscene,
		"IDENTITY_ATTACK": domain.CategoryHateSpeech,
	}

	for attr, scores := range result.AttributeScores {
		if scores.SummaryScore.Value > maxScore {
			maxScore = scores.SummaryScore.Value
			if cat, ok := categoryMap[attr]; ok {
				category = cat
			}
		}
	}

	return maxScore, category, nil
}
