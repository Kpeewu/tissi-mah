package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
)

// paymentPendingSweepBatchSize borne le nombre de réservations traitées par tick.
const paymentPendingSweepBatchSize = 100

// StartPaymentPendingSweeper lance le worker qui libère les places des réservations restées
// en paymentPending au-delà de la fenêtre de paiement. Il couvre les cas que le worker
// d'expiration de payment-service ne voit pas : réservation sans paiement (createPayment
// jamais appelé) ou paiement approuvé dont la confirmation n'a pas atteint booking-service.
func StartPaymentPendingSweeper(ctx context.Context, svc *bookingServiceImpl, intervalSeconds int, timeoutMinutes int, logger *zap.Logger) {
	interval := time.Duration(intervalSeconds) * time.Second
	timeout := time.Duration(timeoutMinutes) * time.Minute

	logger.Info("payment pending sweeper started",
		zap.Duration("interval", interval),
		zap.Duration("timeout", timeout),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("payment pending sweeper stopped")
			return
		case <-ticker.C:
			svc.SweepStalePaymentPendingBookings(ctx, timeout)
		}
	}
}

// SweepStalePaymentPendingBookings traite les réservations paymentPending créées il y a plus de timeout :
//   - aucun paiement, ou paiement failed → FailPayment (places libérées)
//   - paiement held                      → ConfirmPayment (rattrapage d'une confirmation perdue)
//   - paiement pending                   → rien : le worker de payment-service supprime la transaction
func (s *bookingServiceImpl) SweepStalePaymentPendingBookings(ctx context.Context, timeout time.Duration) {
	if s.paymentClient == nil {
		return
	}

	cutoff := time.Now().UTC().Add(-timeout)
	bookingIDs, err := s.readRepo.GetStalePaymentPendingBookingIDs(ctx, cutoff, paymentPendingSweepBatchSize)
	if err != nil {
		s.logger.Error("payment pending sweeper: query failed", zap.Error(err))
		return
	}
	if len(bookingIDs) == 0 {
		return
	}

	s.logger.Info("payment pending sweeper: processing bookings", zap.Int("count", len(bookingIDs)))

	for _, bookingID := range bookingIDs {
		paymentStatus, err := s.paymentClient.GetPaymentStatusByBooking(ctx, bookingID)
		if err != nil {
			s.logger.Error("payment pending sweeper: get payment status failed",
				zap.String("bookingID", bookingID), zap.Error(err))
			continue
		}

		switch paymentStatus {
		case "", "failed":
			if err := s.FailPayment(ctx, &serviceInterfaces.FailPaymentInput{
				BookingID: bookingID,
				Reason:    "paymentTimeout",
			}); err != nil {
				s.logger.Error("payment pending sweeper: FailPayment failed",
					zap.String("bookingID", bookingID), zap.Error(err))
				continue
			}
			s.logger.Info("payment pending sweeper: booking released", zap.String("bookingID", bookingID))

		case "held":
			if _, err := s.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
				BookingID:     bookingID,
				TransactionID: "sweeper",
			}); err != nil && !errors.Is(err, bookingErrors.ErrorPaymentAlreadyConfirmed) {
				s.logger.Error("payment pending sweeper: ConfirmPayment failed",
					zap.String("bookingID", bookingID), zap.Error(err))
				continue
			}
			s.logger.Info("payment pending sweeper: booking confirmed (missed confirmation)", zap.String("bookingID", bookingID))

		default:
			// pending : la transaction FedaPay est gérée par le worker d'expiration de payment-service
		}
	}
}
