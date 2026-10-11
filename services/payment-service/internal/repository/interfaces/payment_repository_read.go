package interfaces

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// PaymentRepositoryRead définit les opérations de lecture sur les paiements.
type PaymentRepositoryRead interface {
	GetByID(ctx context.Context, paymentID string) (*domain.Payment, error)
	GetByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error)
	GetByExternalTransactionID(ctx context.Context, externalID string) (*domain.Payment, error)
	GetWebhookEvent(ctx context.Context, fedapayEventID string) (*domain.WebhookEvent, error)
	// GetExpiredPendingPayments retourne les paiements pending dont expires_at est dépassé (max 100).
	GetExpiredPendingPayments(ctx context.Context, now time.Time) ([]*domain.Payment, error)

	// GetPendingByBookingID retourne le paiement pending d'une réservation (ErrorPaymentNotFound sinon).
	GetPendingByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error)

	// HasActivePayment vérifie s'il existe un paiement actif (pending ou held) pour un booking.
	HasActivePayment(ctx context.Context, bookingID string) (bool, error)
}
