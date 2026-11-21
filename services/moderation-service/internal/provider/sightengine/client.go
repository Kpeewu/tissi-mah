package sightengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	"go.uber.org/zap"
)

const (
	sightengineURL = "https://api.sightengine.com/1.0/check.json"
	// Modèles activés : nudité explicite, contenu offensant, gore.
	models     = "nudity-2.0,offensive,gore"
	httpTimeout = 5 * time.Second
)

// Client implémente provider.ImageModerator via l'API Sightengine.
type Client struct {
	apiUser    string
	apiSecret  string
	httpClient *http.Client
	logger     *zap.Logger
}

func NewClient(apiUser, apiSecret string, logger *zap.Logger) *Client {
	return &Client{
		apiUser:    apiUser,
		apiSecret:  apiSecret,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
	}
}

type sightengineResponse struct {
	Status string `json:"status"`
	Nudity struct {
		SexualActivity  float32 `json:"sexual_activity"`
		SexualDisplay   float32 `json:"sexual_display"`
		Erotica         float32 `json:"erotica"`
		VerySuggestive  float32 `json:"very_suggestive"`
	} `json:"nudity"`
	Offensive struct {
		Prob float32 `json:"prob"`
	} `json:"offensive"`
	Gore struct {
		Prob float32 `json:"prob"`
	} `json:"gore"`
}

// ModerateImage envoie l'image à Sightengine et retourne le score le plus élevé
// parmi les catégories nudité, offensive et gore.
// Retourne (0, NONE, nil) si les credentials sont absents ou si l'API est indisponible.
func (c *Client) ModerateImage(ctx context.Context, imageBytes []byte, mimeType string) (float32, domain.Category, error) {
	if c.apiUser == "" || c.apiSecret == "" {
		return 0, domain.CategoryNone, nil
	}

	// Construire le body multipart/form-data
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("models", models); err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: write models field: %w", err)
	}
	if err := mw.WriteField("api_user", c.apiUser); err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: write api_user field: %w", err)
	}
	if err := mw.WriteField("api_secret", c.apiSecret); err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: write api_secret field: %w", err)
	}

	// Déterminer l'extension à partir du MIME type pour nommer le fichier
	ext := extensionFromMime(mimeType)
	fw, err := mw.CreateFormFile("media", "image"+ext)
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: create form file: %w", err)
	}
	if _, err := fw.Write(imageBytes); err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: write image: %w", err)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sightengineURL, &buf)
	if err != nil {
		return 0, domain.CategoryNone, fmt.Errorf("sightengine: request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("sightengine API unavailable", zap.Error(err))
		return 0, domain.CategoryNone, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("sightengine API non-200", zap.Int("status", resp.StatusCode))
		return 0, domain.CategoryNone, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, domain.CategoryNone, nil
	}

	var result sightengineResponse
	if err := json.Unmarshal(data, &result); err != nil || result.Status != "success" {
		c.logger.Warn("sightengine: unexpected response", zap.String("status", result.Status))
		return 0, domain.CategoryNone, nil
	}

	return highestScore(result)
}

// highestScore détermine le score le plus élevé et la catégorie associée
// en appliquant les seuils définis par Sightengine.
func highestScore(r sightengineResponse) (float32, domain.Category, error) {
	type candidate struct {
		score    float32
		category domain.Category
	}

	candidates := []candidate{
		{r.Nudity.SexualActivity, domain.CategoryNSFW},
		{r.Nudity.SexualDisplay, domain.CategoryNSFW},
		{r.Nudity.Erotica, domain.CategoryNSFW},
		{r.Nudity.VerySuggestive, domain.CategoryNSFW},
		{r.Gore.Prob, domain.CategoryGore},
		{r.Offensive.Prob, domain.CategoryHateSpeech},
	}

	maxScore := float32(0)
	category := domain.CategoryNone
	for _, c := range candidates {
		if c.score > maxScore {
			maxScore = c.score
			category = c.category
		}
	}

	return maxScore, category, nil
}

func extensionFromMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic", "image/heif":
		return ".heic"
	default:
		return ".jpg"
	}
}
