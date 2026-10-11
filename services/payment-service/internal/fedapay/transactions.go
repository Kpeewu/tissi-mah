package fedapay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.uber.org/zap"
)

// CreateTransactionParams regroupe les paramètres de création d'une collecte avec redirection.
type CreateTransactionParams struct {
	Amount            int
	Description       string
	CallbackURL       string            // URL de retour après paiement (FedaPay ajoute ?id=&status=)
	MerchantReference string            // référence unique côté TissiMah (PaymentReference)
	Customer          *CustomerPayload  // optionnel — pas de numéro : l'utilisateur le choisit sur la page FedaPay
	CustomMetadata    map[string]string // optionnel — jamais de données sensibles
}

// CreateTransaction crée une nouvelle transaction FedaPay (collecte avec redirection).
func (c *Client) CreateTransaction(params CreateTransactionParams) (*Transaction, error) {
	payload := map[string]interface{}{
		"description": params.Description,
		"amount":      params.Amount,
		"currency":    map[string]string{"iso": "XOF"},
	}
	if params.CallbackURL != "" {
		payload["callback_url"] = params.CallbackURL
	}
	if params.MerchantReference != "" {
		payload["merchant_reference"] = params.MerchantReference
	}
	if params.Customer != nil {
		payload["customer"] = params.Customer
	}
	if len(params.CustomMetadata) > 0 {
		payload["custom_metadata"] = params.CustomMetadata
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
		zap.String("merchantReference", params.MerchantReference),
	)

	return resp.V1, nil
}

// GetTransactionToken génère le token et le lien de paiement (page FedaPay) d'une transaction.
// Le client mobile est redirigé vers TokenResponse.URL.
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

// CancelTransaction supprime une transaction en attente sur FedaPay (DELETE).
// Une fois supprimée, la page de paiement n'est plus utilisable.
// Doit être appelé uniquement après avoir vérifié via GetTransaction que la transaction n'est pas approuvée.
func (c *Client) CancelTransaction(transactionID int) error {
	path := fmt.Sprintf("/v1/transactions/%d", transactionID)

	body, statusCode, err := c.doRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("cancel transaction: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return ErrTransactionNotFound
	}
	if statusCode < 200 || statusCode >= 300 {
		c.logger.Error("fedapay: cancel transaction failed",
			zap.Int("transactionID", transactionID),
			zap.Int("statusCode", statusCode),
			zap.String("body", string(body)),
		)
		return parseError(body)
	}

	c.logger.Info("fedapay: transaction cancelled", zap.Int("transactionID", transactionID))
	return nil
}

// GetTransaction récupère les détails d'une transaction.
func (c *Client) GetTransaction(transactionID int) (*Transaction, error) {
	path := fmt.Sprintf("/v1/transactions/%d", transactionID)

	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, ErrTransactionNotFound
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

// GetTransactionByMerchantReference récupère une transaction via la référence marchande TissiMah.
// Filet de sécurité quand l'ID FedaPay n'a pas pu être persisté.
func (c *Client) GetTransactionByMerchantReference(reference string) (*Transaction, error) {
	path := fmt.Sprintf("/v1/transactions/merchant/%s", url.PathEscape(reference))

	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get transaction by merchant reference: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, ErrTransactionNotFound
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
