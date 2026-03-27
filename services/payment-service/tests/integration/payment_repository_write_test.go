package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CreatePayment
// =============================================================================

func TestPaymentWriteRepository_CreatePayment(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()

	t.Run("cree un paiement avec succes", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-create-1"),
			fixtures.WithPaymentBookingID("booking-create-1"),
			fixtures.WithPaymentAmount(7500),
		)

		err := writeRepo.CreatePayment(ctx, payment)
		require.NoError(t, err)

		// Verifier en lecture
		result, err := readRepo.GetByID(ctx, "pay-create-1")
		require.NoError(t, err)
		assert.Equal(t, 7500, result.Amount)
		assert.Equal(t, domain.PaymentStatusPending, result.Status)
	})
}

// =============================================================================
// UpdatePaymentStatus
// =============================================================================

func TestPaymentWriteRepository_UpdatePaymentStatus(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()

	t.Run("met a jour le statut pending → held avec completed_at", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-update-1"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		err := writeRepo.UpdatePaymentStatus(ctx, "pay-update-1", domain.PaymentStatusHeld, "ext-123")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "pay-update-1")
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusHeld, result.Status)
		assert.NotNil(t, result.CompletedAt, "CompletedAt doit etre set pour held")
	})

	t.Run("met a jour le statut held → released", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		now := time.Now().UTC()
		payment := fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-update-2"),
			fixtures.WithPaymentStatus(domain.PaymentStatusHeld),
			fixtures.WithPaymentCompletedAt(now),
		)
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		err := writeRepo.UpdatePaymentStatus(ctx, "pay-update-2", domain.PaymentStatusReleased, "")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "pay-update-2")
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusReleased, result.Status)
	})

	t.Run("retourne erreur si paiement inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		err := writeRepo.UpdatePaymentStatus(ctx, "pay-inexistant", domain.PaymentStatusHeld, "")
		assert.ErrorIs(t, err, paymentErrors.ErrorPaymentNotFound)
	})
}

// =============================================================================
// MarkPaymentFailed
// =============================================================================

func TestPaymentWriteRepository_MarkPaymentFailed(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()
	readRepo := newTestPaymentReadRepository()

	t.Run("marque un paiement comme echoue", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		payment := fixtures.NewTestPayment(fixtures.WithPaymentID("pay-fail-1"))
		require.NoError(t, fixtures.InsertPayment(ctx, testPool, payment))

		err := writeRepo.MarkPaymentFailed(ctx, "pay-fail-1", "FedaPay: transaction.declined")
		require.NoError(t, err)

		result, err := readRepo.GetByID(ctx, "pay-fail-1")
		require.NoError(t, err)
		assert.Equal(t, domain.PaymentStatusFailed, result.Status)
		assert.NotNil(t, result.FailedAt)
		require.NotNil(t, result.FailureReason)
		assert.Equal(t, "FedaPay: transaction.declined", *result.FailureReason)
	})
}

// =============================================================================
// SaveWebhookEvent (idempotence)
// =============================================================================

func TestPaymentWriteRepository_SaveWebhookEvent(t *testing.T) {
	ctx := context.Background()
	writeRepo := newTestPaymentWriteRepository()

	t.Run("sauvegarde un evenement webhook", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		now := time.Now().UTC()
		event := &domain.WebhookEvent{
			EventID:        uuid.New().String(),
			FedapayEventID: "fedapay-evt-1",
			EventType:      "transaction.approved",
			Payload:        `{"test": true}`,
			ProcessedAt:    &now,
		}

		err := writeRepo.SaveWebhookEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("retourne ErrorDuplicateWebhookEvent si doublon", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		now := time.Now().UTC()
		event := &domain.WebhookEvent{
			EventID:        uuid.New().String(),
			FedapayEventID: "fedapay-evt-dup",
			EventType:      "transaction.approved",
			Payload:        `{"test": true}`,
			ProcessedAt:    &now,
		}

		err := writeRepo.SaveWebhookEvent(ctx, event)
		require.NoError(t, err)

		// Deuxieme insertion avec meme fedapay_event_id
		event2 := &domain.WebhookEvent{
			EventID:        uuid.New().String(),
			FedapayEventID: "fedapay-evt-dup",
			EventType:      "transaction.approved",
			Payload:        `{"test": true}`,
			ProcessedAt:    &now,
		}

		err = writeRepo.SaveWebhookEvent(ctx, event2)
		assert.ErrorIs(t, err, paymentErrors.ErrorDuplicateWebhookEvent)
	})
}
