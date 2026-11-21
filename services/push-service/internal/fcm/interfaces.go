package fcm

import "context"

// FCMClient définit l'interface pour l'envoi de notifications push via FCM.
type FCMClient interface {
	Send(ctx context.Context, title, body, token string, data map[string]string) error
	SendMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) ([]SendResult, error)
	Close() error
}

// SendResult contient le résultat d'envoi pour un token individuel.
type SendResult struct {
	Token     string
	Success   bool
	ErrorCode string // "UNREGISTERED", "INVALID_ARGUMENT", etc.
}
