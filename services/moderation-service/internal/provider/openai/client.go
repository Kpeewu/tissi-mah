package openai

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
	moderationURL = "https://api.openai.com/v1/moderations"
	httpTimeout   = 3 * time.Second
	model         = "omni-moderation-latest"
)

// categoryMap mappe les catégories OpenAI vers les catégories domaine.
var categoryMap = map[string]domain.Category{
	"sexual":                  domain.CategoryNSFW,
	"sexual/minors":           domain.CategoryCSAM,
	"harassment":              domain.CategoryInsults,
	"harassment/threatening":  domain.CategoryInsults,
	"hate":                    domain.CategoryHateSpeech,
	"hate/threatening":        domain.CategoryHateSpeech,
	"illicit":                 domain.CategoryObscene,
	"illicit/violent":         domain.CategoryGore,
	"violence":                domain.CategoryGore,
	"violence/graphic":        domain.CategoryGore,
	"self-harm":               domain.CategoryGore,
	"self-harm/intent":        domain.CategoryGore,
	"self-harm/instructions":  domain.CategoryGore,
}

// Client implémente provider.TextModerator via l'API OpenAI Moderation.
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

type moderationRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type moderationResponse struct {
	Results []struct {
		Flagged         bool               `json:"flagged"`
		CategoryScores  map[string]float32 `json:"category_scores"`
	} `json:"results"`
}

// ModerateText appelle l'API OpenAI Moderation et retourne le score le plus élevé
// parmi toutes les catégories ainsi que la catégorie associée.
// Retourne (0, NONE, nil) si la clé API est absente ou si l'API est indisponible (fail-open).
func (c *Client) ModerateText(ctx context.Context, text string) (float32, domain.Category, error) {
	if c.apiKey == "" {
		return 0, domain.CategoryNone, nil
	}

	body := moderationRequest{Input: text, Model: model}
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("openai: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, moderationURL, bytes.NewReader(payload))
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("openai: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("openai moderation API unavailable", zap.Error(err))
		return 0, domain.CategoryNone, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("openai moderation API non-200", zap.Int("status", resp.StatusCode))
		return 0, domain.CategoryNone, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, domain.CategoryNone, nil
	}

	var result moderationResponse
	if err := json.Unmarshal(data, &result); err != nil || len(result.Results) == 0 {
		return 0, domain.CategoryNone, nil
	}

	// Sélectionner le score le plus élevé parmi toutes les catégories
	maxScore := float32(0)
	category := domain.CategoryNone
	for name, score := range result.Results[0].CategoryScores {
		if score > maxScore {
			maxScore = score
			if cat, ok := categoryMap[name]; ok {
				category = cat
			}
		}
	}

	return maxScore, category, nil
}
