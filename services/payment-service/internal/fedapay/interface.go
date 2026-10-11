package fedapay

// FedaPayClient définit les opérations FedaPay utilisées par le service et les workers.
// Implémentée par *Client ; permet l'injection d'un mock dans les tests.
type FedaPayClient interface {
	// Collectes (paiement avec redirection)
	CreateTransaction(params CreateTransactionParams) (*Transaction, error)
	GetTransactionToken(transactionID int) (*TokenResponse, error)
	GetTransaction(transactionID int) (*Transaction, error)
	GetTransactionByMerchantReference(reference string) (*Transaction, error)
	CancelTransaction(transactionID int) error

	// Payouts (remboursements passagers et versements chauffeurs)
	CreatePayout(amount int, mode string, phoneNumber string, firstName string, lastName string) (*PayoutItem, error)
	StartPayouts(payoutIDs []int) ([]PayoutItem, error)
	GetPayout(payoutID int) (*PayoutItem, error)

	// Webhooks
	VerifyWebhookSignature(signature string, rawPayload []byte) bool
}

// Vérification à la compilation.
var _ FedaPayClient = (*Client)(nil)
