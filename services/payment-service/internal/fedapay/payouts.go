package fedapay

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// CreatePayout crée un nouveau payout FedaPay vers un numéro Mobile Money.
func (c *Client) CreatePayout(amount int, mode string, phoneNumber string, firstName string, lastName string) (*PayoutItem, error) {
	payload := map[string]interface{}{
		"amount":   amount,
		"currency": map[string]string{"iso": "XOF"},
		"mode":     mode,
		"customer": map[string]interface{}{
			"firstname": firstName,
			"lastname":  lastName,
			"phone_number": map[string]string{
				"number":       phoneNumber,
				"country_code": "TG",
			},
		},
	}

	body, statusCode, err := c.doRequest("POST", "/v1/payouts", payload)
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: create payout failed",
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return nil, parseError(body)
	}

	var resp PayoutResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal payout response: %w", err)
	}

	if resp.V1 == nil {
		return nil, fmt.Errorf("fedapay: empty payout response")
	}

	c.logger.Info("fedapay: payout created",
		zap.Int("payoutID", resp.V1.ID),
		zap.String("reference", resp.V1.Reference),
	)

	return resp.V1, nil
}

// StartPayouts démarre un batch de payouts (PUT /v1/payouts/start).
func (c *Client) StartPayouts(payoutIDs []int) ([]PayoutItem, error) {
	payload := map[string]interface{}{
		"payouts": payoutIDs,
	}

	body, statusCode, err := c.doRequest("PUT", "/v1/payouts/start", payload)
	if err != nil {
		return nil, fmt.Errorf("start payouts: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: start payouts failed",
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return nil, parseError(body)
	}

	var resp BatchPayoutResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal batch payout response: %w", err)
	}

	c.logger.Info("fedapay: payouts batch started",
		zap.Int("count", len(resp.V1)),
	)

	return resp.V1, nil
}

// GetPayout récupère les détails d'un payout.
func (c *Client) GetPayout(payoutID int) (*PayoutItem, error) {
	path := fmt.Sprintf("/v1/payouts/%d", payoutID)

	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get payout: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, parseError(body)
	}

	var resp PayoutResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal payout response: %w", err)
	}

	if resp.V1 == nil {
		return nil, fmt.Errorf("fedapay: empty payout response")
	}

	return resp.V1, nil
}
