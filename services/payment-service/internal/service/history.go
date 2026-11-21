package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
)

// historyOriginKey est la clé de contexte de l'origine (acteur + canal) des entrées d'historique.
type historyOriginKey struct{}

// historyOrigin décrit qui (initiatedBy) et par quel canal (source) une transition est déclenchée.
type historyOrigin struct {
	initiatedBy string
	source      string
}

// withHistoryOrigin pose l'origine des entrées d'historique écrites pendant le traitement.
// Appelé une fois par point d'entrée (RPC, worker) pour éviter de propager l'origine en paramètre.
func withHistoryOrigin(ctx context.Context, initiatedBy, source string) context.Context {
	return context.WithValue(ctx, historyOriginKey{}, historyOrigin{initiatedBy: initiatedBy, source: source})
}

// historyOriginFrom retourne l'origine posée par withHistoryOrigin (system / unknown par défaut).
func historyOriginFrom(ctx context.Context) historyOrigin {
	if origin, ok := ctx.Value(historyOriginKey{}).(historyOrigin); ok {
		return origin
	}
	return historyOrigin{initiatedBy: domain.HistoryInitiatedBySystem, source: domain.HistorySourceUnknown}
}

// writePaymentHistory écrit une entrée dans payment_status_history de manière non bloquante.
// À appeler uniquement après le succès de la transition correspondante.
func (s *paymentServiceImpl) writePaymentHistory(ctx context.Context, paymentID, status, providerStatus, notes string) {
	if s.paymentHistoryRepo == nil {
		return
	}
	origin := historyOriginFrom(ctx)
	entry := &domain.PaymentStatusHistory{
		HistoryID:      uuid.New().String(),
		PaymentID:      paymentID,
		Status:         status,
		ProviderStatus: providerStatus,
		InitiatedBy:    origin.initiatedBy,
		Source:         origin.source,
		OccurredAt:     time.Now().UTC(),
		Notes:          notes,
	}
	if err := s.paymentHistoryRepo.CreateHistoryEntry(ctx, entry); err != nil {
		s.logger.Error("write payment history failed",
			zap.Error(err),
			zap.String("paymentID", paymentID),
			zap.String("status", status),
		)
	}
}

// writeRefundHistory écrit une entrée dans refund_status_history de manière non bloquante.
// À appeler uniquement après le succès de la transition correspondante.
func (s *paymentServiceImpl) writeRefundHistory(ctx context.Context, refundID, status string, retryCount int16, notes string) {
	if s.refundHistoryRepo == nil {
		return
	}
	origin := historyOriginFrom(ctx)
	entry := &domain.RefundStatusHistory{
		HistoryID:   uuid.New().String(),
		RefundID:    refundID,
		Status:      status,
		RetryCount:  retryCount,
		InitiatedBy: origin.initiatedBy,
		Source:      origin.source,
		OccurredAt:  time.Now().UTC(),
		Notes:       notes,
	}
	if err := s.refundHistoryRepo.CreateHistoryEntry(ctx, entry); err != nil {
		s.logger.Error("write refund history failed",
			zap.Error(err),
			zap.String("refundID", refundID),
			zap.String("status", status),
		)
	}
}

// writePayoutHistory écrit une entrée dans payout_status_history de manière non bloquante.
func (s *paymentServiceImpl) writePayoutHistory(ctx context.Context, payoutID, status, initiatedBy, supportUserID, supportFirstName, supportLastName, notes string) {
	if s.payoutHistoryRepo == nil {
		return
	}
	entry := &domain.PayoutStatusHistory{
		HistoryID:        uuid.New().String(),
		PayoutID:         payoutID,
		Status:           status,
		InitiatedBy:      initiatedBy,
		SupportUserID:    supportUserID,
		SupportFirstName: supportFirstName,
		SupportLastName:  supportLastName,
		OccurredAt:       time.Now().UTC(),
		Notes:            notes,
	}
	if err := s.payoutHistoryRepo.CreateHistoryEntry(ctx, entry); err != nil {
		s.logger.Error("write payout history failed",
			zap.Error(err),
			zap.String("payoutID", payoutID),
			zap.String("status", status),
		)
	}
}

// =============================================================================
// Consultation (support)
// =============================================================================

func (s *paymentServiceImpl) GetPaymentHistory(ctx context.Context, paymentID string, bookingID string) ([]*serviceInterfaces.PaymentHistoryEntryResult, error) {
	if paymentID == "" && bookingID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}
	if s.paymentHistoryRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	var entries []*domain.PaymentStatusHistory
	var err error
	if paymentID != "" {
		entries, err = s.paymentHistoryRepo.ListByPaymentID(ctx, paymentID)
	} else {
		entries, err = s.paymentHistoryRepo.ListByBookingID(ctx, bookingID)
	}
	if err != nil {
		return nil, err
	}

	results := make([]*serviceInterfaces.PaymentHistoryEntryResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, &serviceInterfaces.PaymentHistoryEntryResult{
			HistoryID:      e.HistoryID,
			PaymentID:      e.PaymentID,
			Status:         e.Status,
			ProviderStatus: e.ProviderStatus,
			InitiatedBy:    e.InitiatedBy,
			Source:         e.Source,
			OccurredAt:     e.OccurredAt.UTC().Format(time.RFC3339),
			Notes:          e.Notes,
		})
	}
	return results, nil
}

func (s *paymentServiceImpl) GetRefundHistory(ctx context.Context, refundID string, bookingID string) ([]*serviceInterfaces.RefundHistoryEntryResult, error) {
	if refundID == "" && bookingID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}
	if s.refundHistoryRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	var entries []*domain.RefundStatusHistory
	var err error
	if refundID != "" {
		entries, err = s.refundHistoryRepo.ListByRefundID(ctx, refundID)
	} else {
		entries, err = s.refundHistoryRepo.ListByBookingID(ctx, bookingID)
	}
	if err != nil {
		return nil, err
	}

	results := make([]*serviceInterfaces.RefundHistoryEntryResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, &serviceInterfaces.RefundHistoryEntryResult{
			HistoryID:   e.HistoryID,
			RefundID:    e.RefundID,
			Status:      e.Status,
			RetryCount:  int(e.RetryCount),
			InitiatedBy: e.InitiatedBy,
			Source:      e.Source,
			OccurredAt:  e.OccurredAt.UTC().Format(time.RFC3339),
			Notes:       e.Notes,
		})
	}
	return results, nil
}

func (s *paymentServiceImpl) GetPayoutHistory(ctx context.Context, payoutID string) ([]*serviceInterfaces.PayoutHistoryEntryResult, error) {
	if payoutID == "" {
		return nil, paymentErrors.ErrorInvalidInput
	}
	if s.payoutHistoryRepo == nil {
		return nil, paymentErrors.ErrorInternalServer
	}

	entries, err := s.payoutHistoryRepo.ListByPayoutID(ctx, payoutID)
	if err != nil {
		return nil, err
	}

	results := make([]*serviceInterfaces.PayoutHistoryEntryResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, &serviceInterfaces.PayoutHistoryEntryResult{
			HistoryID:        e.HistoryID,
			PayoutID:         e.PayoutID,
			Status:           e.Status,
			InitiatedBy:      e.InitiatedBy,
			SupportUserID:    e.SupportUserID,
			SupportFirstName: e.SupportFirstName,
			SupportLastName:  e.SupportLastName,
			OccurredAt:       e.OccurredAt.UTC().Format(time.RFC3339),
			Notes:            e.Notes,
		})
	}
	return results, nil
}
