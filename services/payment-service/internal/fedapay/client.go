package fedapay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client encapsule toute l'interaction avec l'API FedaPay.
type Client struct {
	httpClient    *http.Client
	apiURL        string
	apiKey        string
	webhookSecret string
	logger        *zap.Logger
	sem           chan struct{} // sémaphore pour limiter les appels concurrents
}

// NewClient crée un nouveau client FedaPay.
func NewClient(apiURL, apiKey, webhookSecret string, maxConcurrent int, logger *zap.Logger) *Client {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				MaxConnsPerHost:     50,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		apiURL:        apiURL,
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		logger:        logger,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// doRequest exécute une requête HTTP avec les headers FedaPay.
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, int, error) {
	// Limiter les appels concurrents vers FedaPay
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	var reqBody io.Reader
	var reqBodyStr string
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		reqBodyStr = string(jsonBody)
		reqBody = bytes.NewReader(jsonBody)
	}

	url := c.apiURL + path
	c.logger.Debug("fedapay: request",
		zap.String("method", method),
		zap.String("url", url),
		zap.String("body", reqBodyStr),
	)

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	c.logger.Debug("fedapay: response",
		zap.String("method", method),
		zap.String("url", url),
		zap.Int("statusCode", resp.StatusCode),
		zap.String("body", string(respBody)),
	)

	return respBody, resp.StatusCode, nil
}

// parseError extrait l'erreur API d'une réponse.
func parseError(body []byte) error {
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("fedapay error: %s", string(body))
	}
	if apiErr.Message != "" {
		return &apiErr
	}
	return fmt.Errorf("fedapay error: %s", string(body))
}
