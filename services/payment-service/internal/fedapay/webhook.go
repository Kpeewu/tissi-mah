package fedapay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// VerifyWebhookSignature vérifie la signature HMAC-SHA256 d'un webhook FedaPay.
func (c *Client) VerifyWebhookSignature(signature string, rawPayload []byte) bool {
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(rawPayload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// ParseWebhookEvent parse le payload brut d'un webhook FedaPay.
func ParseWebhookEvent(rawPayload []byte) (*WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}
	return &payload, nil
}
