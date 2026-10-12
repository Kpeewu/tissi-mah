package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
)

const (
	// refundRequestBatchSize borne le nombre de demandes rejouées par tick.
	refundRequestBatchSize = 100
	// refundRequestGracePeriod laisse à la goroutine lancée lors de la transition le temps d'aboutir.
	refundRequestGracePeriod = 2 * time.Minute
	// maxRefundRequestAttempts : au-delà de ce nombre de refus définitifs de payment-service,
	// la demande n'est plus rejouée (traitement manuel par le support).
	maxRefundRequestAttempts = 5
	// refundRequestTimeout borne un appel (trips-service + payment-service).
	refundRequestTimeout = 10 * time.Second
)

// errDepartureUnknown : la date de départ, nécessaire à la règle des 24h d'une annulation passager,
// n'a pas pu être lue auprès de trips-service. Transitoire : la demande est rejouée.
var errDepartureUnknown = errors.New("departure datetime unavailable")

// StartRefundRequestWorker lance le worker qui rejoue les demandes de remboursement non acquittées
// par payment-service (indisponibilité, timeout, redémarrage du pod pendant la goroutine).
func StartRefundRequestWorker(ctx context.Context, svc *bookingServiceImpl, intervalSeconds int, logger *zap.Logger) {
	interval := time.Duration(intervalSeconds) * time.Second
	logger.Info("refund request worker started", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("refund request worker stopped")
			return
		case <-ticker.C:
			svc.ProcessRefundRequests(ctx)
		}
	}
}

// ProcessRefundRequests rejoue un lot de demandes de remboursement dues et non acquittées.
func (s *bookingServiceImpl) ProcessRefundRequests(ctx context.Context) {
	if s.paymentClient == nil {
		return
	}

	updatedBefore := time.Now().UTC().Add(-refundRequestGracePeriod)
	bookings, err := s.readRepo.GetBookingsPendingRefundRequest(
		ctx, updatedBefore, maxRefundRequestAttempts, refundRequestBatchSize, domain.NoShowContestWindow,
	)
	if err != nil {
		s.logger.Error("refund request worker: query failed", zap.Error(err))
		return
	}
	if len(bookings) == 0 {
		return
	}

	s.logger.Info("refund request worker: replaying refund requests", zap.Int("count", len(bookings)))
	for _, booking := range bookings {
		s.processRefundRequest(ctx, booking)
	}
}

// requestRefundAsync transmet le remboursement dû d'une réservation à payment-service (goroutine
// lancée après la transition). En cas d'échec, le worker rejoue la demande.
func (s *bookingServiceImpl) requestRefundAsync(bookingID string) {
	ctx, cancel := context.WithTimeout(context.Background(), refundRequestTimeout)
	defer cancel()

	// Relire la réservation : motif et horodatages tels que committés par la transition
	booking, err := s.readRepo.GetByID(ctx, bookingID)
	if err != nil {
		s.logger.Error("requestRefundAsync: get booking failed, the worker will retry",
			zap.String("bookingID", bookingID), zap.Error(err))
		return
	}
	if booking.RefundReason == nil || booking.RefundRequestedAt != nil {
		return
	}

	s.processRefundRequest(ctx, booking)
}

// processRefundRequest envoie la demande et enregistre son issue : acquittement, refus définitif
// compté, ou échec transitoire laissé au prochain tick du worker.
func (s *bookingServiceImpl) processRefundRequest(ctx context.Context, booking *domain.Booking) {
	reason := *booking.RefundReason

	err := s.requestRefund(ctx, booking)
	if err == nil {
		if err := s.writeRepo.MarkRefundRequested(ctx, booking.BookingID); err != nil {
			// payment-service est idempotent : un rejeu retournera le même remboursement
			s.logger.Error("refund requested but not marked, it will be replayed",
				zap.String("bookingID", booking.BookingID), zap.Error(err))
			return
		}
		s.logger.Info("refund requested successfully",
			zap.String("bookingID", booking.BookingID), zap.String("reason", reason))
		return
	}

	if !isPermanentRefundError(err) {
		s.logger.Warn("refund request failed, will retry",
			zap.String("bookingID", booking.BookingID), zap.String("reason", reason), zap.Error(err))
		return
	}

	attempts, incErr := s.writeRepo.IncrementRefundRequestAttempts(ctx, booking.BookingID)
	if incErr != nil {
		s.logger.Error("increment refund request attempts failed",
			zap.String("bookingID", booking.BookingID), zap.Error(incErr))
		return
	}
	if attempts >= maxRefundRequestAttempts {
		s.logger.Error("refund request abandoned after repeated rejections: manual handling required",
			zap.String("bookingID", booking.BookingID),
			zap.String("reason", reason),
			zap.Int("attempts", attempts),
			zap.Error(err),
		)
		return
	}
	s.logger.Warn("refund request rejected by payment-service, will retry",
		zap.String("bookingID", booking.BookingID),
		zap.String("reason", reason),
		zap.Int("attempts", attempts),
		zap.Error(err),
	)
}

// requestRefund construit la demande depuis la réservation et l'envoie à payment-service.
func (s *bookingServiceImpl) requestRefund(ctx context.Context, booking *domain.Booking) error {
	reason := *booking.RefundReason

	// Le départ n'est exigé que pour la règle des 24h d'une annulation passager
	departureDatetime := s.getDepartureDatetime(ctx, booking)
	if departureDatetime == "" && reason == domain.RefundReasonCancelledByPassenger {
		return errDepartureUnknown
	}

	return s.paymentClient.RequestRefund(ctx, &client.RefundInput{
		BookingID:         booking.BookingID,
		RefundReason:      reason,
		OriginalAmount:    booking.Subtotal,
		ServiceFee:        booking.ServiceFee,
		DepartureDatetime: departureDatetime,
		ApprovedAt:        formatTimeOptional(booking.ApprovedAt),
		CancelledAt:       refundEventAt(booking).Format(time.RFC3339),
	})
}

// refundEventAt retourne la date de la transition à l'origine du remboursement.
func refundEventAt(booking *domain.Booking) time.Time {
	var at *time.Time
	switch booking.Status {
	case domain.BookingStatusRejected:
		at = booking.RejectedAt
	case domain.BookingStatusNoShow:
		at = booking.NoShowReportedAt
	default:
		at = booking.CancelledAt
	}
	if at == nil {
		return time.Now().UTC()
	}
	return at.UTC()
}

// isPermanentRefundError indique un refus de payment-service qu'un rejeu immédiat ne corrigera pas
// (motif inconnu, paiement introuvable ou dans un état non remboursable).
func isPermanentRefundError(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.NotFound, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
