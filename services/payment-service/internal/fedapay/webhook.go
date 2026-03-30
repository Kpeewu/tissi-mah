package fedapay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// VerifyWebhookSignature vérifie la signature HMAC-SHA256 d'un webhook FedaPay.
// Le header X-FEDAPAY-SIGNATURE a le format "t={timestamp},s={hex_signature}".
// Le HMAC est calculé sur "{timestamp}.{rawPayload}".
func (c *Client) VerifyWebhookSignature(signature string, rawPayload []byte) bool {
	// Parser t= et s= depuis le header
	var timestamp, sig string
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp = part[2:]
		} else if strings.HasPrefix(part, "s=") {
			sig = part[2:]
		}
	}

	if timestamp == "" || sig == "" {
		return false
	}

	// Calculer HMAC-SHA256 de "{timestamp}.{rawPayload}"
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawPayload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expectedMAC))
}

// ParseWebhookEvent parse le payload brut d'un webhook FedaPay.
func ParseWebhookEvent(rawPayload []byte) (*WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, fmt.Errorf("parse webhook payload: %w", err)
	}
	return &payload, nil
}
