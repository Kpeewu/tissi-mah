package interfaces

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
)

// RefundRepositoryWrite définit les opérations d'écriture sur les remboursements.
type RefundRepositoryWrite interface {
	CreateRefund(ctx context.Context, refund *domain.Refund) error
	UpdateRefundStatus(ctx context.Context, refundID string, status domain.RefundStatus) error
}
