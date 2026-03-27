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

// GetTransactionToken génère un token pour une transaction existante.
func (c *Client) GetTransactionToken(transactionID int) (*TokenResponse, error) {
	path := fmt.Sprintf("/v1/transactions/%d/token", transactionID)

	body, statusCode, err := c.doRequest("POST", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get transaction token: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, parseError(body)
	}

	var resp TokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	return &resp, nil
}

// SendTransaction envoie une transaction directement en USSD (moov_tg ou togocel).
func (c *Client) SendTransaction(transactionID int, mode string, phoneNumber string) (*Transaction, error) {
	path := fmt.Sprintf("/v1/transactions/%d/%s", transactionID, mode)
	payload := map[string]interface{}{
		"phone_number": map[string]string{
			"number":       phoneNumber,
			"country_code": "TG",
		},
	}

	body, statusCode, err := c.doRequest("PUT", path, payload)
	if err != nil {
		return nil, fmt.Errorf("send transaction: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: send transaction failed",
			zap.Int("transactionID", transactionID),
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
		zap.Int("transactionID", resp.V1.ID),
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
