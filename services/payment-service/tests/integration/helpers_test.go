package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/implementations"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/repository/interfaces"
	"go.uber.org/zap"
)

// cleanupPaymentTables vide toutes les tables (TRUNCATE CASCADE)
func cleanupPaymentTables(t *testing.T, ctx context.Context) {
	t.Helper()

	_, err := testPool.Exec(ctx, "TRUNCATE TABLE payments, refunds, payouts, webhook_events, payout_batches CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup payment tables: %v", err)
	}
}

func newTestPaymentReadRepository() repoInterfaces.PaymentRepositoryRead {
	return implementations.NewPaymentReadRepository(testPool, zap.NewNop())
}

func newTestPaymentWriteRepository() repoInterfaces.PaymentRepositoryWrite {
	return implementations.NewPaymentWriteRepository(testPool, zap.NewNop())
}

func newTestRefundReadRepository() repoInterfaces.RefundRepositoryRead {
	return implementations.NewRefundReadRepository(testPool, zap.NewNop())
}

func newTestRefundWriteRepository() repoInterfaces.RefundRepositoryWrite {
	return implementations.NewRefundWriteRepository(testPool, zap.NewNop())
}

func newTestPayoutReadRepository() repoInterfaces.PayoutRepositoryRead {
	return implementations.NewPayoutReadRepository(testPool, zap.NewNop())
}

func newTestPayoutWriteRepository() repoInterfaces.PayoutRepositoryWrite {
	return implementations.NewPayoutWriteRepository(testPool, zap.NewNop())
}
