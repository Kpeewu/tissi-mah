package fedapay

import (
	"encoding/json"
	"fmt"
)

// GetBalances retourne les soldes disponibles du compte FedaPay.
func (c *Client) GetBalances() ([]BalanceItem, error) {
	body, statusCode, err := c.doRequest("GET", "/v1/balances", nil)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, parseError(body)
	}

	var resp BalanceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal balance response: %w", err)
	}

	return resp.V1, nil
}

// GetXOFBalance retourne le solde XOF disponible.
func (c *Client) GetXOFBalance() (int, error) {
	balances, err := c.GetBalances()
	if err != nil {
		return 0, err
	}

	for _, b := range balances {
		if b.Currency == "XOF" {
			return b.Amount, nil
		}
	}

	return 0, nil
}
