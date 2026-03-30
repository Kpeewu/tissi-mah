package fedapay

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// CreateTransaction crée une nouvelle transaction FedaPay.
func (c *Client) CreateTransaction(amount int, description string, customer CustomerPayload) (*Transaction, error) {
	payload := map[string]interface{}{
		"description": description,
		"amount":      amount,
		"currency":    map[string]string{"iso": "XOF"},
		"customer":    customer,
	}

	body, statusCode, err := c.doRequest("POST", "/v1/transactions", payload)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: create transaction failed",
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return nil, parseError(body)
	}

	var resp TransactionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal transaction response: %w", err)
	}

	if resp.V1 == nil {
		return nil, fmt.Errorf("fedapay: empty transaction response")
	}

	c.logger.Info("fedapay: transaction created",
		zap.Int("transactionID", resp.V1.ID),
		zap.String("reference", resp.V1.Reference),
	)

	return resp.V1, nil
}

// GetTransactionToken génère un token d'envoi pour une transaction existante.
// Ce token est nécessaire pour appeler SendTransaction.
func (c *Client) GetTransactionToken(transactionID int) (*TokenResponse, error) {
	path := fmt.Sprintf("/v1/transactions/%d/token", transactionID)

	body, statusCode, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get transaction token: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: get transaction token failed",
			zap.Int("transactionID", transactionID),
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return nil, parseError(body)
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	c.logger.Info("fedapay: transaction token generated",
		zap.Int("transactionID", transactionID),
	)

	return &resp, nil
}

// SendTransaction envoie une transaction directement en USSD (moov_tg, togocel ou momo_test).
// Le token doit être obtenu via GetTransactionToken avant cet appel.
func (c *Client) SendTransaction(paymentToken string, mode string, phoneNumber string) (*PaymentIntent, error) {
	path := fmt.Sprintf("/v1/%s", mode)
	payload := map[string]interface{}{
		"token": paymentToken,
		"phone_number": map[string]string{
			"number":  phoneNumber,
			"country": "tg",
		},
	}

	body, statusCode, err := c.doRequest("POST", path, payload)
	if err != nil {
		return nil, fmt.Errorf("send transaction: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: send transaction failed",
			zap.String("mode", mode),
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return nil, parseError(body)
	}

	var resp SendTransactionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal send response: %w", err)
	}

	if resp.V1 == nil {
		return nil, fmt.Errorf("fedapay: empty send transaction response")
	}

	c.logger.Info("fedapay: transaction sent via USSD",
		zap.Int("paymentIntentID", resp.V1.ID),
		zap.String("mode", mode),
		zap.String("status", resp.V1.Status),
	)

	return resp.V1, nil
}

// GetTransaction récupère les détails d'une transaction.
func (c *Client) GetTransaction(transactionID int) (*Transaction, error) {
	path := fmt.Sprintf("/v1/transactions/%d", transactionID)

	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, parseError(body)
	}

	var resp TransactionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal transaction response: %w", err)
	}

	if resp.V1 == nil {
		return nil, fmt.Errorf("fedapay: empty transaction response")
	}

	return resp.V1, nil
}
