package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/fixtures"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/implementations"
)

func paymentHistoryEntry(paymentID, status string, occurredAt time.Time) *domain.PaymentStatusHistory {
	return &domain.PaymentStatusHistory{
		HistoryID:   uuid.New().String(),
		PaymentID:   paymentID,
		Status:      status,
		InitiatedBy: domain.HistoryInitiatedBySystem,
		Source:      domain.HistorySourceFedaPayWebhook,
		OccurredAt:  occurredAt,
	}
}

func TestPaymentHistoryRepository(t *testing.T) {
	ctx := context.Background()
	historyRepo := implementations.NewPaymentHistoryRepository(testPool, zap.NewNop())
	paymentWriteRepo := newTestPaymentWriteRepository()

	t.Run("entrée complète puis relue, champs optionnels NULL en DB", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(fixtures.WithPaymentID("pay-h-1"))))

		occurredAt := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, &domain.PaymentStatusHistory{
			HistoryID: uuid.New().String(), PaymentID: "pay-h-1", Status: "pending", ProviderStatus: "pending",
			InitiatedBy: domain.HistoryInitiatedByPassenger, Source: domain.HistorySourceCreatePayment,
			OccurredAt: occurredAt, Notes: "Transaction FedaPay 101 créée",
		}))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-h-1", "booking_synced", occurredAt.Add(time.Minute))))

		var providerStatus, notes *string
		require.NoError(t, testPool.QueryRow(ctx,
			`SELECT provider_status, notes FROM payment_status_history WHERE payment_id = $1 AND status = 'booking_synced'`,
			"pay-h-1",
		).Scan(&providerStatus, &notes))
		assert.Nil(t, providerStatus)
		assert.Nil(t, notes)

		entries, err := historyRepo.ListByPaymentID(ctx, "pay-h-1")
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "pending", entries[0].Status)
		assert.Equal(t, "pending", entries[0].ProviderStatus)
		assert.Equal(t, domain.HistoryInitiatedByPassenger, entries[0].InitiatedBy)
		assert.Equal(t, domain.HistorySourceCreatePayment, entries[0].Source)
		assert.True(t, occurredAt.Equal(entries[0].OccurredAt))
		assert.Equal(t, "Transaction FedaPay 101 créée", entries[0].Notes)
		assert.Equal(t, "booking_synced", entries[1].Status)
		assert.Empty(t, entries[1].ProviderStatus)
		assert.Empty(t, entries[1].Notes)
	})

	t.Run("ordre chronologique indépendant de l'ordre d'insertion", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(fixtures.WithPaymentID("pay-h-2"))))

		base := time.Now().UTC()
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-h-2", "refunded", base.Add(2*time.Second))))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-h-2", "pending", base)))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-h-2", "held", base.Add(time.Second))))

		entries, err := historyRepo.ListByPaymentID(ctx, "pay-h-2")
		require.NoError(t, err)
		statuses := make([]string, 0, len(entries))
		for _, e := range entries {
			statuses = append(statuses, e.Status)
		}
		assert.Equal(t, []string{"pending", "held", "refunded"}, statuses)
	})

	t.Run("par réservation : tous les paiements de la réservation, pas les autres", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-old"), fixtures.WithPaymentBookingID("bk-h"),
			fixtures.WithPaymentStatus(domain.PaymentStatusFailed),
		)))
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-new"), fixtures.WithPaymentBookingID("bk-h"),
		)))
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID("pay-other"), fixtures.WithPaymentBookingID("bk-other"),
		)))

		base := time.Now().UTC()
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-old", "failed", base)))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-new", "pending", base.Add(time.Second))))
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-other", "pending", base)))

		entries, err := historyRepo.ListByBookingID(ctx, "bk-h")
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "pay-old", entries[0].PaymentID)
		assert.Equal(t, "pay-new", entries[1].PaymentID)
	})

	t.Run("aucune entrée : liste vide non nil", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		entries, err := historyRepo.ListByBookingID(ctx, "bk-none")
		require.NoError(t, err)
		assert.NotNil(t, entries)
		assert.Empty(t, entries)
	})

	t.Run("FK violation si paiement inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		err := historyRepo.CreateHistoryEntry(ctx, paymentHistoryEntry("pay-nonexistent", "pending", time.Now().UTC()))
		assert.Error(t, err)
	})
}

func TestRefundHistoryRepository(t *testing.T) {
	ctx := context.Background()
	historyRepo := implementations.NewRefundHistoryRepository(testPool, zap.NewNop())
	paymentWriteRepo := newTestPaymentWriteRepository()
	refundWriteRepo := newTestRefundWriteRepository()

	seedRefund := func(t *testing.T, paymentID, refundID, bookingID string) {
		t.Helper()
		require.NoError(t, paymentWriteRepo.CreatePayment(ctx, fixtures.NewTestPayment(
			fixtures.WithPaymentID(paymentID), fixtures.WithPaymentBookingID(bookingID),
			fixtures.WithPaymentStatus(domain.PaymentStatusRefunded),
		)))
		require.NoError(t, refundWriteRepo.CreateRefund(ctx, fixtures.NewTestRefund(
			fixtures.WithRefundID(refundID), fixtures.WithRefundPaymentID(paymentID), fixtures.WithRefundBookingID(bookingID),
		)))
	}

	t.Run("insertion, relecture par refund et par réservation", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)
		seedRefund(t, "pay-r-1", "ref-r-1", "bk-r")
		seedRefund(t, "pay-r-2", "ref-r-2", "bk-r-other")

		base := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
		for i, e := range []struct {
			status string
			retry  int16
		}{{"pending", 0}, {"processing", 0}, {domain.RefundEventRetryScheduled, 1}} {
			require.NoError(t, historyRepo.CreateHistoryEntry(ctx, &domain.RefundStatusHistory{
				HistoryID: uuid.New().String(), RefundID: "ref-r-1", Status: e.status, RetryCount: e.retry,
				InitiatedBy: domain.HistoryInitiatedBySystem, Source: domain.HistorySourceRefundWorker,
				OccurredAt: base.Add(time.Duration(i) * time.Second), Notes: "note " + e.status,
			}))
		}
		require.NoError(t, historyRepo.CreateHistoryEntry(ctx, &domain.RefundStatusHistory{
			HistoryID: uuid.New().String(), RefundID: "ref-r-2", Status: "pending",
			InitiatedBy: domain.HistoryInitiatedByBookingService, Source: domain.HistorySourceBookingService,
			OccurredAt: base,
		}))

		byRefund, err := historyRepo.ListByRefundID(ctx, "ref-r-1")
		require.NoError(t, err)
		require.Len(t, byRefund, 3)
		assert.Equal(t, domain.RefundEventRetryScheduled, byRefund[2].Status)
		assert.Equal(t, int16(1), byRefund[2].RetryCount)
		assert.Equal(t, domain.HistorySourceRefundWorker, byRefund[2].Source)
		assert.Equal(t, "note retry_scheduled", byRefund[2].Notes)

		byBooking, err := historyRepo.ListByBookingID(ctx, "bk-r")
		require.NoError(t, err)
		assert.Len(t, byBooking, 3)
	})

	t.Run("FK violation si refund inexistant", func(t *testing.T) {
		cleanupPaymentTables(t, ctx)

		err := historyRepo.CreateHistoryEntry(ctx, &domain.RefundStatusHistory{
			HistoryID: uuid.New().String(), RefundID: "ref-nonexistent", Status: "pending",
			InitiatedBy: domain.HistoryInitiatedBySystem, Source: domain.HistorySourceUnknown, OccurredAt: time.Now().UTC(),
		})
		assert.Error(t, err)
	})
}
