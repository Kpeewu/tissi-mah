package fedapay

// =============================================================================
// Types de réponse de l'API FedaPay
// =============================================================================

// Transaction représente une transaction FedaPay.
type Transaction struct {
	ID          int    `json:"id"`
	Klass       string `json:"klass"`
	Reference   string `json:"reference"`
	Amount      int    `json:"amount"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, approved, declined, transferred, refunded, canceled
	Mode        string `json:"mode"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TransactionResponse est la réponse de création d'une transaction.
type TransactionResponse struct {
	V1 *Transaction `json:"v1"`
}

// TokenResponse est la réponse de génération de token pour une transaction.
type TokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

// SendTransactionResponse est la réponse d'envoi direct USSD.
type SendTransactionResponse struct {
	V1 *Transaction `json:"v1"`
}

// PayoutItem représente un payout FedaPay.
type PayoutItem struct {
	ID            int    `json:"id"`
	Klass         string `json:"klass"`
	Reference     string `json:"reference"`
	Amount        int    `json:"amount"`
	Status        string `json:"status"` // pending, started, completed, failed
	Mode          string `json:"mode"`
	FailReason    string `json:"fail_reason"`
	SentAt        string `json:"sent_at"`
	FailedAt      string `json:"failed_at"`
	ScheduledDate string `json:"scheduled_date"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// PayoutResponse est la réponse de création d'un payout.
type PayoutResponse struct {
	V1 *PayoutItem `json:"v1"`
}

// BatchPayoutResponse est la réponse du démarrage batch de payouts.
type BatchPayoutResponse struct {
	V1 []PayoutItem `json:"v1"`
}

// BalanceItem représente un solde FedaPay.
type BalanceItem struct {
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	Mode     string `json:"mode"`
}

// BalanceResponse est la réponse de consultation des soldes.
type BalanceResponse struct {
	V1 []BalanceItem `json:"v1"`
}

// WebhookPayload est le corps brut du webhook FedaPay.
type WebhookPayload struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Entity      WebhookEntity   `json:"entity"`
}

// WebhookEntity est l'entité contenue dans un webhook.
type WebhookEntity struct {
	ID          int    `json:"id"`
	Klass       string `json:"klass"`
	Reference   string `json:"reference"`
	Amount      int    `json:"amount"`
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// APIError représente une erreur de l'API FedaPay.
type APIError struct {
	Message string `json:"message"`
	Errors  map[string][]string `json:"errors"`
}

func (e *APIError) Error() string {
	return e.Message
}

// CustomerPayload représente les données client pour une transaction.
type CustomerPayload struct {
	FirstName   string `json:"firstname"`
	LastName    string `json:"lastname"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number"`
}
