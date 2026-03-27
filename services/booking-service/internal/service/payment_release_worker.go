package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// StartPaymentReleaseJob lance le worker de libération des paiements en arrière-plan.
// Il vérifie périodiquement les bookings complétés dont le délai de contestation est dépassé
// et appelle payment-service pour libérer les paiements (held → released).
func StartPaymentReleaseJob(ctx context.Context, svc *bookingServiceImpl, intervalSeconds int, contestationDelaySeconds int, logger *zap.Logger) {
	interval := time.Duration(intervalSeconds) * time.Second
	contestationDelay := time.Duration(contestationDelaySeconds) * time.Second

	logger.Info("payment release worker started",
		zap.Duration("interval", interval),
		zap.Duration("contestationDelay", contestationDelay),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("payment release worker stopped")
			return
		case <-ticker.C:
			svc.processPaymentReleases(ctx, contestationDelay)
		}
	}
}

// processPaymentReleases traite les bookings complétés dont le délai de contestation est dépassé.
func (s *bookingServiceImpl) processPaymentReleases(ctx context.Context, contestationDelay time.Duration) {
	if s.paymentClient == nil {
		return
	}

	cutoff := time.Now().UTC().Add(-contestationDelay)

	bookings, err := s.readRepo.GetCompletedBookingsPendingRelease(ctx, cutoff)
	if err != nil {
		s.logger.Error("payment release: failed to get pending bookings", zap.Error(err))
		return
	}

	if len(bookings) == 0 {
		s.logger.Debug("payment release: no bookings pending release")
		return
	}

	s.logger.Info("payment release: processing bookings", zap.Int("count", len(bookings)))

	for _, booking := range bookings {
		if err := s.paymentClient.ReleasePayment(ctx, booking.BookingID); err != nil {
			s.logger.Error("payment release: ReleasePayment failed",
				zap.String("bookingID", booking.BookingID),
				zap.Error(err),
			)
			continue
		}

		if err := s.writeRepo.MarkPaymentReleased(ctx, booking.BookingID); err != nil {
			s.logger.Error("payment release: MarkPaymentReleased failed",
				zap.String("bookingID", booking.BookingID),
				zap.Error(err),
			)
			continue
		}

		s.logger.Info("payment released for booking",
			zap.String("bookingID", booking.BookingID),
			zap.String("tripID", booking.TripID),
		)
	}
}
