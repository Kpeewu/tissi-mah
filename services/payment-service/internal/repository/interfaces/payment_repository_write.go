package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PaymentRepositoryWrite définit les opérations d'écriture sur les paiements.
type PaymentRepositoryWrite interface {
	CreatePayment(ctx context.Context, payment *domain.Payment) error
	UpdatePaymentStatus(ctx context.Context, paymentID string, status domain.PaymentStatus, externalTransactionID string) error
	MarkPaymentFailed(ctx context.Context, paymentID string, reason string) error
	SaveWebhookEvent(ctx context.Context, event *domain.WebhookEvent) error
}
