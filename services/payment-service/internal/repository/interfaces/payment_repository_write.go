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

	// UpdateProviderStatus enregistre le dernier statut FedaPay observé (sans changer le statut interne).
	UpdateProviderStatus(ctx context.Context, paymentID string, providerStatus string) error

	// UpdatePaymentURL met à jour le lien de paiement FedaPay d'un paiement pending.
	UpdatePaymentURL(ctx context.Context, paymentID string, paymentURL string) error

	// MarkFailedPaymentHeld fait passer un paiement failed → held (approbation FedaPay tardive revérifiée).
	MarkFailedPaymentHeld(ctx context.Context, paymentID string) error

	// MarkBookingSynced enregistre que le paiement held est rattaché à sa réservation (idempotent).
	MarkBookingSynced(ctx context.Context, paymentID string) error

	// IncrementBookingSyncAttempts incrémente le compteur de reprises de rattachement échouées
	// et retourne sa nouvelle valeur.
	IncrementBookingSyncAttempts(ctx context.Context, paymentID string) (int, error)

	// AnonymizePassengerPhoneNumbers efface passenger_phone_number pour les paiements
	// dont le booking_id figure dans la liste. Utilisé lors de la suppression de compte.
	AnonymizePassengerPhoneNumbers(ctx context.Context, bookingIDs []string) error
}
