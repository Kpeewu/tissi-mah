package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/cache"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type bookingServiceImpl struct {
	readRepo      repoInterfaces.BookingRepositoryRead
	writeRepo     repoInterfaces.BookingRepositoryWrite
	tripClient    client.TripClient
	userClient    client.UserClient
	paymentClient client.PaymentClient
	cache         *cache.BookingCache
	serviceFee    int // pourcentage
	logger        *zap.Logger
}

func NewBookingService(
	readRepo repoInterfaces.BookingRepositoryRead,
	writeRepo repoInterfaces.BookingRepositoryWrite,
	tripClient client.TripClient,
	userClient client.UserClient,
	paymentClient client.PaymentClient,
	bookingCache *cache.BookingCache,
	serviceFeePercent int,
	logger *zap.Logger,
) serviceInterfaces.BookingService {
	return &bookingServiceImpl{
		readRepo:      readRepo,
		writeRepo:     writeRepo,
		tripClient:    tripClient,
		userClient:    userClient,
		paymentClient: paymentClient,
		cache:         bookingCache,
		serviceFee:    serviceFeePercent,
		logger:        logger,
	}
}

// =============================================================================
// CreateBooking
// =============================================================================

func (s *bookingServiceImpl) CreateBooking(ctx context.Context, input *serviceInterfaces.CreateBookingInput) (*serviceInterfaces.CreateBookingResult, error) {
	s.logger.Debug("service: CreateBooking called",
		zap.String("passengerID", input.PassengerID),
		zap.String("tripID", input.TripID),
		zap.Int("seatsBooked", input.SeatsBooked),
	)

	// 1. Validation input
	if err := s.validateCreateInput(input); err != nil {
		return nil, err
	}

	// 2. Récupérer les détails du trajet
	tripDetails, err := s.tripClient.GetTripDetails(ctx, input.TripID)
	if err != nil {
		s.logger.Error("service: GetTripDetails failed", zap.Error(err))
		return nil, bookingErrors.ErrorInternalServer
	}
	if tripDetails == nil {
		return nil, bookingErrors.ErrorTripNotFound
	}

	// Vérifier que le trajet est disponible
	if tripDetails.Status != "scheduled" && tripDetails.Status != "inProgress" {
		return nil, bookingErrors.ErrorTripNotAvailable
	}

	// 3. Vérifier que le passager n'est pas le conducteur
	if input.PassengerID == tripDetails.DriverID {
		return nil, bookingErrors.ErrorPassengerIsDriver
	}

	// 4. Vérifier que le passager existe
	exists, err := s.userClient.UserExists(ctx, input.PassengerID)
	if err != nil {
		s.logger.Error("service: UserExists failed", zap.Error(err))
		return nil, bookingErrors.ErrorInternalServer
	}
	if !exists {
		return nil, bookingErrors.ErrorPassengerNotFound
	}

	// 5. Vérifier pas de double réservation
	hasActive, err := s.readRepo.HasActiveBooking(ctx, input.PassengerID, input.TripID)
	if err != nil {
		return nil, err
	}
	if hasActive {
		return nil, bookingErrors.ErrorDuplicateBooking
	}

	// 6. Déterminer les sequencer orders des waypoints pickup et dropoff
	pickupOrder, dropoffOrder := resolveWaypointOrders(tripDetails.Waypoints, input.PickupWaypointID, input.DropoffWaypointID)
	if pickupOrder == -1 || dropoffOrder == -1 {
		return nil, bookingErrors.ErrorInvalidWaypoints
	}
	maxOrder := getMaxWaypointOrder(tripDetails.Waypoints)

	// 7. Réservation atomique Redis des places par segment
	if s.cache != nil {
		// Initialiser les compteurs par segment si absents
		if err := s.cache.InitSegmentSeatCounters(ctx, input.TripID, tripDetails.TotalSeats, maxOrder); err != nil {
			s.logger.Warn("service: InitSegmentSeatCounters failed, continuing without cache", zap.Error(err))
		} else {
			if err := s.cache.ReserveSegmentSeats(ctx, input.TripID, pickupOrder, dropoffOrder, input.SeatsBooked); err != nil {
				return nil, bookingErrors.ErrorNoSeatsAvailable
			}
		}
	} else if tripDetails.AvailableSeats < input.SeatsBooked {
		return nil, bookingErrors.ErrorNoSeatsAvailable
	}

	// 8. Calculer les prix depuis les waypoints (server-side)
	pricePerSeat, err := s.calculatePriceFromWaypoints(tripDetails.Waypoints, input.PickupWaypointID, input.DropoffWaypointID)
	if err != nil {
		s.logger.Error("service: calculatePriceFromWaypoints failed", zap.Error(err))
		return nil, err
	}
	subtotal := input.SeatsBooked * pricePerSeat
	serviceFee := subtotal * s.serviceFee / 100
	totalAmount := subtotal + serviceFee

	// 8. Générer l'ID et la référence
	bookingID := uuid.New().String()
	bookingRef := generateBookingReference()

	// 9. Déterminer le statut initial
	paymentMethod := domain.PaymentMethod(input.PaymentMethod)
	var initialStatus domain.BookingStatus
	var approvedAt *time.Time

	if paymentMethod == domain.PaymentCash {
		if tripDetails.AutoApproveEnabled {
			initialStatus = domain.BookingStatusApproved
			now := time.Now()
			approvedAt = &now
		} else {
			initialStatus = domain.BookingStatusPendingApproval
		}
	} else {
		initialStatus = domain.BookingStatusPaymentPending
	}

	// 10. Construire le domaine
	booking := &domain.Booking{
		BookingID:         bookingID,
		BookingReference:  bookingRef,
		TripID:            input.TripID,
		PassengerID:       input.PassengerID,
		DriverID:          tripDetails.DriverID,
		PickupWaypointID:      input.PickupWaypointID,
		DropoffWaypointID:     input.DropoffWaypointID,
		PickupSequencerOrder:  int16(pickupOrder),
		DropoffSequencerOrder: int16(dropoffOrder),
		SeatsBooked:       int16(input.SeatsBooked),
		PricePerSeat:      pricePerSeat,
		Subtotal:          subtotal,
		ServiceFee:        serviceFee,
		TotalAmount:       totalAmount,
		PaymentMethod:     paymentMethod,
		Status:            initialStatus,
		ApprovedAt:        approvedAt,
	}

	// Construire les segments avec prix calculé côté serveur
	segments := make([]*domain.Segment, 0, len(input.Segments))
	for _, seg := range input.Segments {
		pickupTime := parseTimeOptional(seg.PickupScheduledAt)
		dropoffTime := parseTimeOptional(seg.DropoffScheduledAt)
		distMeters := seg.SegmentDistanceMeters
		durMinutes := seg.SegmentDurationMinutes

		// Calculer le prix du segment depuis les waypoints
		segPrice, err := s.calculatePriceFromWaypoints(tripDetails.Waypoints, seg.PickupWaypointID, seg.DropoffWaypointID)
		if err != nil {
			s.logger.Error("service: calculatePriceFromWaypoints failed for segment",
				zap.String("pickupWP", seg.PickupWaypointID),
				zap.String("dropoffWP", seg.DropoffWaypointID),
				zap.Error(err),
			)
			return nil, err
		}

		segments = append(segments, &domain.Segment{
			SegmentID:              uuid.New().String(),
			BookingID:              bookingID,
			PickupWaypointID:       seg.PickupWaypointID,
			DropoffWaypointID:      seg.DropoffWaypointID,
			PickupLocationName:     seg.PickupLocationName,
			PickupCity:             seg.PickupCity,
			PickupLat:              seg.PickupLat,
			PickupLng:              seg.PickupLng,
			PickupScheduledAt:      pickupTime,
			DropoffLocationName:    seg.DropoffLocationName,
			DropoffCity:            seg.DropoffCity,
			DropoffLat:             seg.DropoffLat,
			DropoffLng:             seg.DropoffLng,
			DropoffScheduledAt:     dropoffTime,
			SegmentDistanceMeters:  &distMeters,
			SegmentDurationMinutes: &durMinutes,
			SegmentPrice:           &segPrice,
		})
	}

	// Entrée d'historique
	history := &domain.StatusHistoryEntry{
		HistoryID:      uuid.New().String(),
		BookingID:      bookingID,
		PreviousStatus: string(domain.BookingStatusCreated),
		NewStatus:      string(initialStatus),
		ChangedBy:      input.PassengerID,
		ChangedByType:  "passenger",
	}

	// 11. Persister
	if err := s.writeRepo.Create(ctx, booking, segments, history); err != nil {
		// Compensation Redis en cas d'échec DB
		if s.cache != nil {
			_ = s.cache.RestoreSegmentSeats(ctx, input.TripID, pickupOrder, dropoffOrder, input.SeatsBooked)
		}
		return nil, err
	}

	// 12. Invalider le cache
	if s.cache != nil {
		s.cache.InvalidatePassengerBookings(ctx, input.PassengerID)
		s.cache.InvalidateDriverTripBookings(ctx, tripDetails.DriverID, input.TripID)
	}

	return &serviceInterfaces.CreateBookingResult{
		BookingID:        bookingID,
		BookingReference: bookingRef,
		Status:           string(initialStatus),
		TotalAmount:      totalAmount,
	}, nil
}

// =============================================================================
// GetBookingDetails
// =============================================================================

func (s *bookingServiceImpl) GetBookingDetails(ctx context.Context, input *serviceInterfaces.GetBookingDetailsInput) (*serviceInterfaces.BookingDetailResult, error) {
	if input.BookingID == "" {
		return nil, bookingErrors.ErrorInvalidInput
	}

	booking, segments, history, err := s.readRepo.GetByIDWithDetails(ctx, input.BookingID)
	if err != nil {
		return nil, err
	}

	// Vérification d'autorisation : l'utilisateur doit être le passager ou le conducteur
	// Skippée pour les appels inter-services (UserID vide)
	if input.UserID != "" && booking.PassengerID != input.UserID && booking.DriverID != input.UserID {
		return nil, bookingErrors.ErrorUnauthorized
	}

	return s.mapBookingToDetailResult(booking, segments, history), nil
}

// =============================================================================
// GetPassengerBookings
// =============================================================================

func (s *bookingServiceImpl) GetPassengerBookings(ctx context.Context, input *serviceInterfaces.GetPassengerBookingsInput) ([]*serviceInterfaces.BookingPreviewResult, error) {
	if input.PassengerID == "" {
		return nil, bookingErrors.ErrorInvalidInput
	}

	// Cache check
	if s.cache != nil {
		cached, err := s.cache.GetPassengerBookings(ctx, input.PassengerID, input.PageIndex, input.StatusFilter)
		if err == nil && cached != nil {
			return s.mapPreviewsToResults(cached), nil
		}
	}

	previews, err := s.readRepo.GetPassengerBookings(ctx, input.PassengerID, input.PageIndex, input.StatusFilter)
	if err != nil {
		return nil, err
	}

	// Cache set
	if s.cache != nil {
		_ = s.cache.SetPassengerBookings(ctx, input.PassengerID, input.PageIndex, input.StatusFilter, previews)
	}

	return s.mapPreviewsToResults(previews), nil
}

// =============================================================================
// GetDriverTripBookings
// =============================================================================

func (s *bookingServiceImpl) GetDriverTripBookings(ctx context.Context, input *serviceInterfaces.GetDriverTripBookingsInput) ([]*serviceInterfaces.BookingPreviewResult, error) {
	if input.DriverID == "" || input.TripID == "" {
		return nil, bookingErrors.ErrorInvalidInput
	}

	previews, err := s.readRepo.GetDriverTripBookings(ctx, input.DriverID, input.TripID, input.PageIndex)
	if err != nil {
		return nil, err
	}

	return s.mapPreviewsToResults(previews), nil
}

// =============================================================================
// ApproveBooking
// =============================================================================

func (s *bookingServiceImpl) ApproveBooking(ctx context.Context, input *serviceInterfaces.ApproveBookingInput) error {
	if input.DriverID == "" || input.BookingID == "" {
		return bookingErrors.ErrorInvalidInput
	}

	if err := s.writeRepo.Approve(ctx, input.BookingID, input.DriverID); err != nil {
		return err
	}

	s.invalidateBookingCaches(ctx, input.BookingID)
	return nil
}

// =============================================================================
// RejectBooking
// =============================================================================

func (s *bookingServiceImpl) RejectBooking(ctx context.Context, input *serviceInterfaces.RejectBookingInput) error {
	if input.DriverID == "" || input.BookingID == "" {
		return bookingErrors.ErrorInvalidInput
	}

	// Récupérer le booking pour connaître le tripID et les places
	booking, err := s.readRepo.GetByID(ctx, input.BookingID)
	if err != nil {
		return err
	}

	if err := s.writeRepo.Reject(ctx, input.BookingID, input.DriverID, input.Reason); err != nil {
		return err
	}

	// Restaurer les places Redis
	if s.cache != nil {
		_ = s.cache.RestoreSegmentSeats(ctx, booking.TripID, int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked))
	}

	s.invalidateBookingCaches(ctx, input.BookingID)

	// Demander le remboursement au payment-service (fire-and-forget)
	if s.shouldRequestRefund(booking) {
		go s.requestRefundAsync(booking, "bookingRejected", time.Now().UTC())
	}

	return nil
}

// =============================================================================
// CancelBooking
// =============================================================================

func (s *bookingServiceImpl) CancelBooking(ctx context.Context, input *serviceInterfaces.CancelBookingInput) error {
	if input.UserID == "" || input.BookingID == "" {
		return bookingErrors.ErrorInvalidInput
	}

	// Récupérer le booking pour connaître le tripID et les places
	booking, err := s.readRepo.GetByID(ctx, input.BookingID)
	if err != nil {
		return err
	}

	if err := s.writeRepo.Cancel(ctx, input.BookingID, input.UserID, input.Reason); err != nil {
		return err
	}

	// Restaurer les places Redis
	if s.cache != nil {
		_ = s.cache.RestoreSegmentSeats(ctx, booking.TripID, int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked))
	}

	s.invalidateBookingCaches(ctx, input.BookingID)

	// Demander le remboursement au payment-service (fire-and-forget)
	if s.shouldRequestRefund(booking) {
		reason := "cancelledByPassenger"
		if input.UserID == booking.DriverID {
			reason = "cancelledByDriver"
		}
		go s.requestRefundAsync(booking, reason, time.Now().UTC())
	}

	return nil
}

// =============================================================================
// StartBookingsForWaypoint
// =============================================================================

func (s *bookingServiceImpl) StartBookingsForWaypoint(ctx context.Context, input *serviceInterfaces.StartBookingsForWaypointInput) (int, error) {
	if input.TripID == "" || input.WaypointID == "" {
		return 0, bookingErrors.ErrorInvalidInput
	}

	count, err := s.writeRepo.StartBookingsForWaypoint(ctx, input.TripID, input.WaypointID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// =============================================================================
// CompleteBookingsForWaypoint
// =============================================================================

func (s *bookingServiceImpl) CompleteBookingsForWaypoint(ctx context.Context, input *serviceInterfaces.CompleteBookingsForWaypointInput) (int, error) {
	if input.TripID == "" || input.WaypointID == "" {
		return 0, bookingErrors.ErrorInvalidInput
	}

	count, err := s.writeRepo.CompleteBookingsForWaypoint(ctx, input.TripID, input.WaypointID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// =============================================================================
// ReportNoShow
// =============================================================================

func (s *bookingServiceImpl) ReportNoShow(ctx context.Context, input *serviceInterfaces.ReportNoShowInput) error {
	if input.BookingID == "" || input.ReporterID == "" || input.NoShowType == "" {
		return bookingErrors.ErrorInvalidInput
	}

	if input.NoShowType != "driver" && input.NoShowType != "passenger" {
		return bookingErrors.ErrorInvalidInput
	}

	// Récupérer le booking pour restaurer les places
	booking, err := s.readRepo.GetByID(ctx, input.BookingID)
	if err != nil {
		return err
	}

	if err := s.writeRepo.ReportNoShow(ctx, input.BookingID, input.ReporterID, input.NoShowType, input.Description); err != nil {
		return err
	}

	// Restaurer les places Redis
	if s.cache != nil {
		_ = s.cache.RestoreSegmentSeats(ctx, booking.TripID, int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked))
	}

	s.invalidateBookingCaches(ctx, input.BookingID)

	// Demander le remboursement au payment-service (fire-and-forget)
	if s.shouldRequestRefund(booking) {
		reason := "noShowPassenger"
		if input.NoShowType == "driver" {
			reason = "noShowDriver"
		}
		go s.requestRefundAsync(booking, reason, time.Now().UTC())
	}

	return nil
}

// =============================================================================
// ConfirmPayment
// =============================================================================

func (s *bookingServiceImpl) ConfirmPayment(ctx context.Context, input *serviceInterfaces.ConfirmPaymentInput) error {
	if input.BookingID == "" || input.TransactionID == "" {
		return bookingErrors.ErrorInvalidInput
	}

	if err := s.writeRepo.ConfirmPayment(ctx, input.BookingID, input.TransactionID); err != nil {
		return err
	}

	s.invalidateBookingCaches(ctx, input.BookingID)
	return nil
}

// =============================================================================
// FailPayment
// =============================================================================

func (s *bookingServiceImpl) FailPayment(ctx context.Context, input *serviceInterfaces.FailPaymentInput) error {
	if input.BookingID == "" {
		return bookingErrors.ErrorInvalidInput
	}

	// Récupérer le booking pour connaître le tripID et les places
	booking, err := s.readRepo.GetByID(ctx, input.BookingID)
	if err != nil {
		return err
	}

	if err := s.writeRepo.FailPayment(ctx, input.BookingID, input.Reason); err != nil {
		return err
	}

	// Restaurer les places Redis
	if s.cache != nil {
		_ = s.cache.RestoreSegmentSeats(ctx, booking.TripID, int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked))
	}

	s.invalidateBookingCaches(ctx, input.BookingID)
	return nil
}

// =============================================================================
// CancelBookingsForWaypoint
// =============================================================================

func (s *bookingServiceImpl) CancelBookingsForWaypoint(ctx context.Context, input *serviceInterfaces.CancelBookingsForWaypointInput) (int, error) {
	if input.TripID == "" || input.WaypointID == "" {
		return 0, bookingErrors.ErrorInvalidInput
	}

	cancelledBookings, err := s.writeRepo.CancelBookingsForWaypoint(ctx, input.TripID, input.WaypointID)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	for _, booking := range cancelledBookings {
		// Restaurer les places Redis par segment
		if s.cache != nil {
			_ = s.cache.RestoreSegmentSeats(ctx, booking.TripID, int(booking.PickupSequencerOrder), int(booking.DropoffSequencerOrder), int(booking.SeatsBooked))
		}

		s.invalidateBookingCaches(ctx, booking.BookingID)

		// Demander le remboursement au payment-service (fire-and-forget)
		if s.shouldRequestRefund(booking) {
			go s.requestRefundAsync(booking, "waypointCancelled", now)
		}
	}

	return len(cancelledBookings), nil
}

// =============================================================================
// Helpers
// =============================================================================

// calculatePriceFromWaypoints calcule le prix entre deux waypoints comme la somme
// des price_from_previous des waypoints dont sequencer_order > pickup et <= dropoff.
func (s *bookingServiceImpl) calculatePriceFromWaypoints(waypoints []client.TripWaypoint, pickupWaypointID, dropoffWaypointID string) (int, error) {
	pickupOrder := -1
	dropoffOrder := -1

	for _, wp := range waypoints {
		if wp.WaypointID == pickupWaypointID {
			pickupOrder = wp.SequencerOrder
		}
		if wp.WaypointID == dropoffWaypointID {
			dropoffOrder = wp.SequencerOrder
		}
	}

	if pickupOrder == -1 || dropoffOrder == -1 {
		return 0, bookingErrors.ErrorInvalidWaypoints
	}
	if pickupOrder >= dropoffOrder {
		return 0, bookingErrors.ErrorInvalidWaypoints
	}

	price := 0
	for _, wp := range waypoints {
		if wp.SequencerOrder > pickupOrder && wp.SequencerOrder <= dropoffOrder {
			price += wp.PriceFromPrevious
		}
	}

	return price, nil
}

func (s *bookingServiceImpl) validateCreateInput(input *serviceInterfaces.CreateBookingInput) error {
	if input.PassengerID == "" {
		return bookingErrors.ErrorInvalidInput
	}
	if input.TripID == "" {
		return bookingErrors.ErrorInvalidInput
	}
	if input.PickupWaypointID == "" || input.DropoffWaypointID == "" {
		return bookingErrors.ErrorInvalidInput
	}
	if input.SeatsBooked < 1 {
		return bookingErrors.ErrorInvalidInput
	}

	validMethods := map[string]bool{
		string(domain.PaymentMobileMoney): true,
		string(domain.PaymentCard):        true,
		string(domain.PaymentPaypal):      true,
		string(domain.PaymentCash):        true,
	}
	if !validMethods[input.PaymentMethod] {
		return bookingErrors.ErrorInvalidInput
	}

	if len(input.Segments) == 0 {
		return bookingErrors.ErrorInvalidInput
	}

	return nil
}

// resolveWaypointOrders retourne les sequencer orders des waypoints pickup et dropoff.
// Retourne -1, -1 si un waypoint n'est pas trouvé.
func resolveWaypointOrders(waypoints []client.TripWaypoint, pickupWaypointID, dropoffWaypointID string) (int, int) {
	pickupOrder := -1
	dropoffOrder := -1
	for _, wp := range waypoints {
		if wp.WaypointID == pickupWaypointID {
			pickupOrder = wp.SequencerOrder
		}
		if wp.WaypointID == dropoffWaypointID {
			dropoffOrder = wp.SequencerOrder
		}
	}
	return pickupOrder, dropoffOrder
}

// getMaxWaypointOrder retourne le plus grand sequencer order parmi les waypoints.
func getMaxWaypointOrder(waypoints []client.TripWaypoint) int {
	max := 0
	for _, wp := range waypoints {
		if wp.SequencerOrder > max {
			max = wp.SequencerOrder
		}
	}
	return max
}

func (s *bookingServiceImpl) invalidateBookingCaches(ctx context.Context, bookingID string) {
	if s.cache == nil {
		return
	}

	booking, err := s.readRepo.GetByID(ctx, bookingID)
	if err != nil {
		return
	}

	s.cache.InvalidateBooking(ctx, bookingID)
	s.cache.InvalidatePassengerBookings(ctx, booking.PassengerID)
	s.cache.InvalidateDriverTripBookings(ctx, booking.DriverID, booking.TripID)
}

func (s *bookingServiceImpl) mapBookingToDetailResult(booking *domain.Booking, segments []*domain.Segment, history []*domain.StatusHistoryEntry) *serviceInterfaces.BookingDetailResult {
	result := &serviceInterfaces.BookingDetailResult{
		BookingID:          booking.BookingID,
		BookingReference:   booking.BookingReference,
		TripID:             booking.TripID,
		PassengerID:        booking.PassengerID,
		DriverID:           booking.DriverID,
		PickupWaypointID:   booking.PickupWaypointID,
		DropoffWaypointID:  booking.DropoffWaypointID,
		SeatsBooked:        int(booking.SeatsBooked),
		PricePerSeat:       booking.PricePerSeat,
		Subtotal:           booking.Subtotal,
		ServiceFee:         booking.ServiceFee,
		TotalAmount:        booking.TotalAmount,
		PaymentMethod:      string(booking.PaymentMethod),
		Status:             string(booking.Status),
		PaymentCompletedAt: formatTimeOptional(booking.PaymentCompletedAt),
		ApprovedAt:         formatTimeOptional(booking.ApprovedAt),
		RejectedAt:         formatTimeOptional(booking.RejectedAt),
		CancelledAt:        formatTimeOptional(booking.CancelledAt),
		CompletedAt:        formatTimeOptional(booking.CompletedAt),
		CancellerID:        derefString(booking.CancellerID),
		CancellationReason: derefString(booking.CancellationReason),
		NoShowType:         derefString(booking.NoShowType),
		NoShowReportedBy:   derefString(booking.NoShowReportedBy),
		NoShowReportedAt:   formatTimeOptional(booking.NoShowReportedAt),
		NoShowDescription:  derefString(booking.NoShowDescription),
		CreatedAt:          booking.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          booking.UpdatedAt.Format(time.RFC3339),
	}

	for _, seg := range segments {
		result.Segments = append(result.Segments, serviceInterfaces.SegmentDetailResult{
			SegmentID:              seg.SegmentID,
			PickupWaypointID:       seg.PickupWaypointID,
			DropoffWaypointID:      seg.DropoffWaypointID,
			PickupLocationName:     seg.PickupLocationName,
			PickupCity:             seg.PickupCity,
			PickupLat:              seg.PickupLat,
			PickupLng:              seg.PickupLng,
			PickupScheduledAt:      formatTimeOptional(seg.PickupScheduledAt),
			PickupActualAt:         formatTimeOptional(seg.PickupActualAt),
			DropoffLocationName:    seg.DropoffLocationName,
			DropoffCity:            seg.DropoffCity,
			DropoffLat:             seg.DropoffLat,
			DropoffLng:             seg.DropoffLng,
			DropoffScheduledAt:     formatTimeOptional(seg.DropoffScheduledAt),
			DropoffActualAt:        formatTimeOptional(seg.DropoffActualAt),
			SegmentDistanceMeters:  derefInt(seg.SegmentDistanceMeters),
			SegmentDurationMinutes: derefInt(seg.SegmentDurationMinutes),
			SegmentPrice:           derefInt(seg.SegmentPrice),
		})
	}

	for _, h := range history {
		result.History = append(result.History, serviceInterfaces.StatusHistoryResult{
			HistoryID:      h.HistoryID,
			PreviousStatus: h.PreviousStatus,
			NewStatus:      h.NewStatus,
			ChangedBy:      h.ChangedBy,
			ChangedByType:  h.ChangedByType,
			ChangeReason:   derefString(h.ChangeReason),
			Metadata:       derefString(h.Metadata),
			CreatedAt:      h.CreatedAt.Format(time.RFC3339),
		})
	}

	return result
}

func (s *bookingServiceImpl) mapPreviewsToResults(previews []*domain.BookingPreview) []*serviceInterfaces.BookingPreviewResult {
	results := make([]*serviceInterfaces.BookingPreviewResult, 0, len(previews))
	for _, p := range previews {
		results = append(results, &serviceInterfaces.BookingPreviewResult{
			BookingID:           p.BookingID,
			BookingReference:    p.BookingReference,
			TripID:              p.TripID,
			Status:              string(p.Status),
			SeatsBooked:         int(p.SeatsBooked),
			TotalAmount:         p.TotalAmount,
			PickupLocationName:  p.PickupLocationName,
			DropoffLocationName: p.DropoffLocationName,
			DepartureDate:       p.DepartureDatetime.Format("2006-01-02"),
			DepartureTime:       p.DepartureDatetime.Format("15:04"),
		})
	}
	return results
}

// generateBookingReference génère une référence unique RES-YYYYMMDD-XXXXXX.
func generateBookingReference() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	b.WriteString("RES-")
	b.WriteString(time.Now().UTC().Format("20060102"))
	b.WriteByte('-')
	for range 6 {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b.WriteByte(chars[n.Int64()])
	}
	return b.String()
}

func parseTimeOptional(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func formatTimeOptional(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// =============================================================================
// Payment integration helpers
// =============================================================================

// shouldRequestRefund vérifie si le booking nécessite un remboursement via payment-service.
func (s *bookingServiceImpl) shouldRequestRefund(booking *domain.Booking) bool {
	return s.paymentClient != nil &&
		booking.PaymentMethod != domain.PaymentCash &&
		booking.PaymentCompletedAt != nil
}

// requestRefundAsync appelle payment-service en fire-and-forget (goroutine).
func (s *bookingServiceImpl) requestRefundAsync(booking *domain.Booking, reason string, cancelledAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Récupérer la date de départ depuis trips-service
	departureDatetime := s.getDepartureDatetime(ctx, booking)

	input := &client.RefundInput{
		BookingID:         booking.BookingID,
		RefundReason:      reason,
		OriginalAmount:    booking.Subtotal,
		ServiceFee:        booking.ServiceFee,
		DepartureDatetime: departureDatetime,
		ApprovedAt:        formatTimeOptional(booking.ApprovedAt),
		CancelledAt:       cancelledAt.Format(time.RFC3339),
	}

	if err := s.paymentClient.RequestRefund(ctx, input); err != nil {
		s.logger.Error("requestRefundAsync: payment-service call failed",
			zap.String("bookingID", booking.BookingID),
			zap.String("reason", reason),
			zap.Error(err),
		)
	} else {
		s.logger.Info("refund requested successfully",
			zap.String("bookingID", booking.BookingID),
			zap.String("reason", reason),
		)
	}
}

// getDepartureDatetime récupère la date de départ du waypoint de pickup.
func (s *bookingServiceImpl) getDepartureDatetime(ctx context.Context, booking *domain.Booking) string {
	tripDetails, err := s.tripClient.GetTripDetails(ctx, booking.TripID)
	if err != nil || tripDetails == nil {
		s.logger.Warn("getDepartureDatetime: cannot fetch trip details",
			zap.String("tripID", booking.TripID), zap.Error(err),
		)
		return ""
	}

	for _, wp := range tripDetails.Waypoints {
		if wp.WaypointID == booking.PickupWaypointID {
			return wp.ScheduledPickupDatetime
		}
	}

	s.logger.Warn("getDepartureDatetime: pickup waypoint not found",
		zap.String("tripID", booking.TripID),
		zap.String("pickupWaypointID", booking.PickupWaypointID),
	)
	return ""
}

// =============================================================================
// Reconciliation (appelé par le job de réconciliation)
// =============================================================================

// ReconcileSeats recalcule les compteurs Redis et synchronise la DB trips.
func (s *bookingServiceImpl) ReconcileSeats(ctx context.Context) {
	s.logger.Debug("reconciliation: starting seat reconciliation")

	tripIDs, err := s.readRepo.GetActiveTripsWithBookings(ctx)
	if err != nil {
		s.logger.Error("reconciliation: GetActiveTripsWithBookings failed", zap.Error(err))
		return
	}

	for _, tripID := range tripIDs {
		s.reconcileTrip(ctx, tripID)
	}

	s.logger.Debug("reconciliation: completed", zap.Int("tripsProcessed", len(tripIDs)))
}

func (s *bookingServiceImpl) reconcileTrip(ctx context.Context, tripID string) {
	// Récupérer les détails du trajet depuis trips-service
	tripDetails, err := s.tripClient.GetTripDetails(ctx, tripID)
	if err != nil || tripDetails == nil {
		s.logger.Warn("reconciliation: skip trip (cannot fetch details)", zap.String("tripID", tripID), zap.Error(err))
		return
	}

	maxOrder := getMaxWaypointOrder(tripDetails.Waypoints)
	if maxOrder <= 1 {
		return
	}

	// Réconcilier chaque segment (leg)
	minAvailable := tripDetails.TotalSeats
	for order := 1; order < maxOrder; order++ {
		occupancy, err := s.readRepo.GetSegmentOccupancy(ctx, tripID, order)
		if err != nil {
			s.logger.Warn("reconciliation: skip segment", zap.String("tripID", tripID), zap.Int("order", order), zap.Error(err))
			continue
		}

		expectedAvailable := tripDetails.TotalSeats - occupancy
		if expectedAvailable < minAvailable {
			minAvailable = expectedAvailable
		}

		// Mettre à jour le compteur Redis par segment
		if s.cache != nil {
			_ = s.cache.SetSegmentSeatCounter(ctx, tripID, order, expectedAvailable)
		}
	}

	// Mettre à jour la DB trips avec le minimum (pour affichage global)
	if tripDetails.AvailableSeats != minAvailable {
		s.logger.Info("reconciliation: updating trips DB",
			zap.String("tripID", tripID),
			zap.Int("dbCurrent", tripDetails.AvailableSeats),
			zap.Int("expected", minAvailable),
		)
		if err := s.tripClient.UpdateAvailableSeats(ctx, tripID, minAvailable); err != nil {
			s.logger.Error("reconciliation: UpdateAvailableSeats failed",
				zap.String("tripID", tripID), zap.Error(err))
		}
	}
}

// StartReconciliationJob lance le job de réconciliation en arrière-plan.
func StartReconciliationJob(ctx context.Context, svc *bookingServiceImpl, intervalSeconds int, logger *zap.Logger) {
	interval := time.Duration(intervalSeconds) * time.Second
	logger.Info("reconciliation job started", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("reconciliation job stopped")
			return
		case <-ticker.C:
			svc.ReconcileSeats(ctx)
		}
	}
}

// AsImpl retourne l'implémentation concrète pour le job de réconciliation.
func AsImpl(svc serviceInterfaces.BookingService) *bookingServiceImpl {
	impl, _ := svc.(*bookingServiceImpl)
	return impl
}

// GetServiceFeePercent est un helper utilisé dans le handler pour les calculs.
func GetServiceFeePercent(svc serviceInterfaces.BookingService) int {
	impl, ok := svc.(*bookingServiceImpl)
	if !ok {
		return 10
	}
	return impl.serviceFee
}
