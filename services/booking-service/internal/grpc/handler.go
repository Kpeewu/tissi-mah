package grpc

import (
	"context"
	"errors"
	"time"

	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/booking-service/internal/service/interfaces"
	bookingErrors "github.com/Kpeewu/tissi-mah/services/booking-service/pkg/errors"
	bookingpb "github.com/Kpeewu/tissi-mah/services/booking-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// BookingHandler implémente bookingpb.BookingServiceServer.
type BookingHandler struct {
	bookingpb.UnimplementedBookingServiceServer
	service serviceInterfaces.BookingService
	logger  *zap.Logger
}

func NewBookingHandler(service serviceInterfaces.BookingService, logger *zap.Logger) *BookingHandler {
	return &BookingHandler{service: service, logger: logger}
}

// CreateBooking crée une nouvelle réservation.
func (h *BookingHandler) CreateBooking(ctx context.Context, req *bookingpb.CreateBookingRequest) (*bookingpb.CreateBookingResponse, error) {
	h.logger.Debug("handler: CreateBooking called",
		zap.String("passengerID", req.PassengerId),
		zap.String("tripID", req.TripId),
	)

	segments := make([]serviceInterfaces.SegmentInput, 0, len(req.Segments))
	for _, s := range req.Segments {
		segments = append(segments, serviceInterfaces.SegmentInput{
			PickupWaypointID:       s.PickupWaypointId,
			DropoffWaypointID:      s.DropoffWaypointId,
			PickupLocationName:     s.PickupLocationName,
			PickupCity:             s.PickupCity,
			PickupLat:              s.PickupLat,
			PickupLng:              s.PickupLng,
			PickupScheduledAt:      s.PickupScheduledAt,
			DropoffLocationName:    s.DropoffLocationName,
			DropoffCity:            s.DropoffCity,
			DropoffLat:             s.DropoffLat,
			DropoffLng:             s.DropoffLng,
			DropoffScheduledAt:     s.DropoffScheduledAt,
			SegmentDistanceMeters:  int(s.SegmentDistanceMeters),
			SegmentDurationMinutes: int(s.SegmentDurationMinutes),
			SegmentPrice:           int(s.SegmentPrice),
		})
	}

	input := &serviceInterfaces.CreateBookingInput{
		PassengerID:        req.PassengerId,
		TripID:             req.TripId,
		PickupWaypointID:   req.PickupWaypointId,
		DropoffWaypointID:  req.DropoffWaypointId,
		SeatsBooked:        int(req.SeatsBooked),
		PaymentMethod:      req.PaymentMethod,
		Segments:           segments,
		PassengerMessage:   req.PassengerMessage,
		ExtraMinutesDetour: int(req.ExtraMinutesDetour),
	}

	result, err := h.service.CreateBooking(ctx, input)
	if err != nil {
		h.logger.Error("handler: CreateBooking failed", zap.Error(err))
		return &bookingpb.CreateBookingResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.CreateBookingResponse{
		BookingId:        result.BookingID,
		BookingReference: result.BookingReference,
		Status:           result.Status,
		TotalAmount:      int32(result.TotalAmount),
	}, nil
}

// GetBookingDetails retourne les détails complets d'une réservation.
func (h *BookingHandler) GetBookingDetails(ctx context.Context, req *bookingpb.GetBookingDetailsRequest) (*bookingpb.GetBookingDetailsResponse, error) {
	h.logger.Debug("handler: GetBookingDetails called", zap.String("bookingID", req.BookingId))

	result, err := h.service.GetBookingDetails(ctx, &serviceInterfaces.GetBookingDetailsInput{
		BookingID: req.BookingId,
		UserID:    req.UserId,
	})
	if err != nil {
		return &bookingpb.GetBookingDetailsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.GetBookingDetailsResponse{
		Booking: toProtoBookingDetail(result),
	}, nil
}

// toProtoBookingDetail convertit un BookingDetailResult en message proto BookingDetail.
func toProtoBookingDetail(result *serviceInterfaces.BookingDetailResult) *bookingpb.BookingDetail {
	pbSegments := make([]*bookingpb.SegmentDetail, 0, len(result.Segments))
	for _, s := range result.Segments {
		pbSegments = append(pbSegments, &bookingpb.SegmentDetail{
			SegmentId:              s.SegmentID,
			PickupWaypointId:       s.PickupWaypointID,
			DropoffWaypointId:      s.DropoffWaypointID,
			PickupLocationName:     s.PickupLocationName,
			PickupCity:             s.PickupCity,
			PickupLat:              s.PickupLat,
			PickupLng:              s.PickupLng,
			PickupScheduledAt:      s.PickupScheduledAt,
			PickupActualAt:         s.PickupActualAt,
			DropoffLocationName:    s.DropoffLocationName,
			DropoffCity:            s.DropoffCity,
			DropoffLat:             s.DropoffLat,
			DropoffLng:             s.DropoffLng,
			DropoffScheduledAt:     s.DropoffScheduledAt,
			DropoffActualAt:        s.DropoffActualAt,
			SegmentDistanceMeters:  int32(s.SegmentDistanceMeters),
			SegmentDurationMinutes: int32(s.SegmentDurationMinutes),
			SegmentPrice:           int32(s.SegmentPrice),
		})
	}

	pbHistory := make([]*bookingpb.StatusHistoryEntry, 0, len(result.History))
	for _, h := range result.History {
		pbHistory = append(pbHistory, &bookingpb.StatusHistoryEntry{
			HistoryId:      h.HistoryID,
			PreviousStatus: h.PreviousStatus,
			NewStatus:      h.NewStatus,
			ChangedBy:      h.ChangedBy,
			ChangedByName:  h.ChangedByName,
			ChangedByType:  h.ChangedByType,
			ChangeReason:   h.ChangeReason,
			Metadata:       h.Metadata,
			CreatedAt:      h.CreatedAt,
		})
	}

	return &bookingpb.BookingDetail{
		BookingId:          result.BookingID,
		BookingReference:   result.BookingReference,
		TripId:             result.TripID,
		PassengerId:        result.PassengerID,
		DriverId:           result.DriverID,
		PickupWaypointId:   result.PickupWaypointID,
		DropoffWaypointId:  result.DropoffWaypointID,
		SeatsBooked:        int32(result.SeatsBooked),
		PricePerSeat:       int32(result.PricePerSeat),
		Subtotal:           int32(result.Subtotal),
		ServiceFee:         int32(result.ServiceFee),
		TotalAmount:        int32(result.TotalAmount),
		PaymentMethod:      result.PaymentMethod,
		Status:             result.Status,
		PaymentCompletedAt: result.PaymentCompletedAt,
		ApprovedAt:         result.ApprovedAt,
		RejectedAt:         result.RejectedAt,
		CancelledAt:        result.CancelledAt,
		CompletedAt:        result.CompletedAt,
		CancellerId:        result.CancellerID,
		CancellationReason: result.CancellationReason,
		NoShowType:         result.NoShowType,
		NoShowReportedBy:   result.NoShowReportedBy,
		NoShowReportedAt:   result.NoShowReportedAt,
		NoShowDescription:  result.NoShowDescription,
		CreatedAt:          result.CreatedAt,
		UpdatedAt:          result.UpdatedAt,
		Segments:           pbSegments,
		History:            pbHistory,
		PassengerMessage:   result.PassengerMessage,
		RoutePolyline:      result.RoutePolyline,
	}
}

// GetPassengerBookings retourne la liste paginée des réservations d'un passager.
func (h *BookingHandler) GetPassengerBookings(ctx context.Context, req *bookingpb.GetPassengerBookingsRequest) (*bookingpb.GetPassengerBookingsResponse, error) {
	results, err := h.service.GetPassengerBookings(ctx, &serviceInterfaces.GetPassengerBookingsInput{
		PassengerID:  req.PassengerId,
		PageIndex:    int(req.Index),
		StatusFilter: req.StatusFilter,
	})
	if err != nil {
		return &bookingpb.GetPassengerBookingsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.GetPassengerBookingsResponse{
		Bookings: toProtoBookingPreviews(results),
	}, nil
}

// GetDriverTripBookings retourne les réservations d'un trajet conducteur (enrichies + compteurs).
func (h *BookingHandler) GetDriverTripBookings(ctx context.Context, req *bookingpb.GetDriverTripBookingsRequest) (*bookingpb.GetDriverTripBookingsResponse, error) {
	result, err := h.service.GetDriverTripBookings(ctx, &serviceInterfaces.GetDriverTripBookingsInput{
		DriverID:  req.DriverId,
		TripID:    req.TripId,
		PageIndex: int(req.Index),
	})
	if err != nil {
		return &bookingpb.GetDriverTripBookingsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.GetDriverTripBookingsResponse{
		Bookings:       toProtoBookingPreviews(result.Bookings),
		DriverBookings: toProtoDriverBookingPreviews(result.DriverBookings),
		Counts:         toProtoBookingCounts(result.Counts),
	}, nil
}

// GetDriverPendingBookings retourne la liste agrégée des demandes en attente du conducteur.
func (h *BookingHandler) GetDriverPendingBookings(ctx context.Context, req *bookingpb.GetDriverPendingBookingsRequest) (*bookingpb.GetDriverPendingBookingsResponse, error) {
	results, err := h.service.GetDriverPendingBookings(ctx, &serviceInterfaces.GetDriverPendingBookingsInput{
		DriverID:  req.DriverId,
		PageIndex: int(req.Index),
	})
	if err != nil {
		return &bookingpb.GetDriverPendingBookingsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.GetDriverPendingBookingsResponse{
		Bookings: toProtoDriverBookingPreviews(results),
	}, nil
}

// GetActivePassengerSummariesForTrip retourne les passagers actifs d'un trajet enrichis.
func (h *BookingHandler) GetActivePassengerSummariesForTrip(ctx context.Context, req *bookingpb.GetActivePassengerSummariesForTripRequest) (*bookingpb.GetActivePassengerSummariesForTripResponse, error) {
	results, err := h.service.GetActivePassengerSummariesForTrip(ctx, req.TripId)
	if err != nil {
		return &bookingpb.GetActivePassengerSummariesForTripResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	summaries := make([]*bookingpb.PassengerSummary, 0, len(results))
	for _, r := range results {
		summaries = append(summaries, &bookingpb.PassengerSummary{
			PassengerId:   r.PassengerID,
			PassengerName: r.PassengerName,
			SeatsBooked:   int32(r.SeatsBooked),
			PaymentMethod: r.PaymentMethod,
			PaymentStatus: r.PaymentStatus,
			Rating:        r.Rating,
			IsVerified:    r.IsVerified,
			BookingId:     r.BookingID,
		})
	}

	return &bookingpb.GetActivePassengerSummariesForTripResponse{Summaries: summaries}, nil
}

// ApproveBooking approuve une réservation.
func (h *BookingHandler) ApproveBooking(ctx context.Context, req *bookingpb.ApproveBookingRequest) (*bookingpb.ApproveBookingResponse, error) {
	err := h.service.ApproveBooking(ctx, &serviceInterfaces.ApproveBookingInput{
		DriverID:  req.DriverId,
		BookingID: req.BookingId,
	})
	if err != nil {
		return &bookingpb.ApproveBookingResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.ApproveBookingResponse{Success: true}, nil
}

// RejectBooking rejette une réservation.
func (h *BookingHandler) RejectBooking(ctx context.Context, req *bookingpb.RejectBookingRequest) (*bookingpb.RejectBookingResponse, error) {
	err := h.service.RejectBooking(ctx, &serviceInterfaces.RejectBookingInput{
		DriverID:  req.DriverId,
		BookingID: req.BookingId,
		Reason:    req.Reason,
	})
	if err != nil {
		return &bookingpb.RejectBookingResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.RejectBookingResponse{Success: true}, nil
}

// CancelBooking annule une réservation.
func (h *BookingHandler) CancelBooking(ctx context.Context, req *bookingpb.CancelBookingRequest) (*bookingpb.CancelBookingResponse, error) {
	err := h.service.CancelBooking(ctx, &serviceInterfaces.CancelBookingInput{
		UserID:    req.UserId,
		BookingID: req.BookingId,
		Reason:    req.Reason,
	})
	if err != nil {
		return &bookingpb.CancelBookingResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.CancelBookingResponse{Success: true}, nil
}

// StartBookingsForWaypoint démarre les réservations d'un waypoint.
func (h *BookingHandler) StartBookingsForWaypoint(ctx context.Context, req *bookingpb.StartBookingsForWaypointRequest) (*bookingpb.StartBookingsForWaypointResponse, error) {
	count, err := h.service.StartBookingsForWaypoint(ctx, &serviceInterfaces.StartBookingsForWaypointInput{
		TripID:     req.TripId,
		WaypointID: req.WaypointId,
	})
	if err != nil {
		return &bookingpb.StartBookingsForWaypointResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.StartBookingsForWaypointResponse{
		Success:       true,
		BookingsCount: int32(count),
	}, nil
}

// CompleteBookingsForWaypoint complète les réservations d'un waypoint.
func (h *BookingHandler) CompleteBookingsForWaypoint(ctx context.Context, req *bookingpb.CompleteBookingsForWaypointRequest) (*bookingpb.CompleteBookingsForWaypointResponse, error) {
	count, err := h.service.CompleteBookingsForWaypoint(ctx, &serviceInterfaces.CompleteBookingsForWaypointInput{
		TripID:     req.TripId,
		WaypointID: req.WaypointId,
	})
	if err != nil {
		return &bookingpb.CompleteBookingsForWaypointResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.CompleteBookingsForWaypointResponse{
		Success:       true,
		BookingsCount: int32(count),
	}, nil
}

// CancelBookingsForWaypoint annule les réservations actives d'un waypoint supprimé.
func (h *BookingHandler) CancelBookingsForWaypoint(ctx context.Context, req *bookingpb.CancelBookingsForWaypointRequest) (*bookingpb.CancelBookingsForWaypointResponse, error) {
	count, err := h.service.CancelBookingsForWaypoint(ctx, &serviceInterfaces.CancelBookingsForWaypointInput{
		TripID:     req.TripId,
		WaypointID: req.WaypointId,
	})
	if err != nil {
		return &bookingpb.CancelBookingsForWaypointResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.CancelBookingsForWaypointResponse{
		Success:       true,
		BookingsCount: int32(count),
	}, nil
}

// CancelBookingsForTrip annule toutes les réservations actives d'un trajet annulé.
func (h *BookingHandler) CancelBookingsForTrip(ctx context.Context, req *bookingpb.CancelBookingsForTripRequest) (*bookingpb.CancelBookingsForTripResponse, error) {
	count, err := h.service.CancelBookingsForTrip(ctx, &serviceInterfaces.CancelBookingsForTripInput{
		TripID: req.TripId,
	})
	if err != nil {
		return &bookingpb.CancelBookingsForTripResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.CancelBookingsForTripResponse{
		Success:       true,
		BookingsCount: int32(count),
	}, nil
}

// ReportNoShow signale un no-show.
func (h *BookingHandler) ReportNoShow(ctx context.Context, req *bookingpb.ReportNoShowRequest) (*bookingpb.ReportNoShowResponse, error) {
	err := h.service.ReportNoShow(ctx, &serviceInterfaces.ReportNoShowInput{
		BookingID:   req.BookingId,
		ReporterID:  req.ReporterId,
		NoShowType:  req.NoShowType,
		Description: req.Description,
	})
	if err != nil {
		return &bookingpb.ReportNoShowResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.ReportNoShowResponse{Success: true}, nil
}

// ConfirmPayment confirme le paiement d'une réservation.
func (h *BookingHandler) ConfirmPayment(ctx context.Context, req *bookingpb.ConfirmPaymentRequest) (*bookingpb.ConfirmPaymentResponse, error) {
	err := h.service.ConfirmPayment(ctx, &serviceInterfaces.ConfirmPaymentInput{
		BookingID:     req.BookingId,
		TransactionID: req.TransactionId,
	})
	if err != nil {
		return &bookingpb.ConfirmPaymentResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.ConfirmPaymentResponse{
		Success: true,
		Status:  "pendingApproval",
	}, nil
}

// FailPayment signale l'échec du paiement d'une réservation.
func (h *BookingHandler) FailPayment(ctx context.Context, req *bookingpb.FailPaymentRequest) (*bookingpb.FailPaymentResponse, error) {
	err := h.service.FailPayment(ctx, &serviceInterfaces.FailPaymentInput{
		BookingID: req.BookingId,
		Reason:    req.Reason,
	})
	if err != nil {
		return &bookingpb.FailPaymentResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.FailPaymentResponse{Success: true}, nil
}

// GetActivePassengerIDsForTrip retourne les IDs des passagers avec une réservation active sur un trajet.
func (h *BookingHandler) GetActivePassengerIDsForTrip(ctx context.Context, req *bookingpb.GetActivePassengerIDsForTripRequest) (*bookingpb.GetActivePassengerIDsForTripResponse, error) {
	ids, err := h.service.GetActivePassengerIDsForTrip(ctx, req.TripID)
	if err != nil {
		return &bookingpb.GetActivePassengerIDsForTripResponse{}, toGRPCError(err)
	}
	return &bookingpb.GetActivePassengerIDsForTripResponse{PassengerIDs: ids}, nil
}

// CheckDeletionEligibility vérifie si un utilisateur peut supprimer son compte côté booking-service.
func (h *BookingHandler) CheckDeletionEligibility(ctx context.Context, req *bookingpb.CheckDeletionEligibilityRequest) (*bookingpb.CheckDeletionEligibilityResponse, error) {
	canDelete, reason, err := h.service.CheckDeletionEligibility(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: CheckDeletionEligibility failed", zap.Error(err))
		return &bookingpb.CheckDeletionEligibilityResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &bookingpb.CheckDeletionEligibilityResponse{CanDelete: canDelete, BlockingReason: reason}, nil
}

// AnonymizeUserData anonymise les références de l'utilisateur dans booking-service.
func (h *BookingHandler) AnonymizeUserData(ctx context.Context, req *bookingpb.AnonymizeUserDataRequest) (*bookingpb.AnonymizeUserDataResponse, error) {
	if err := h.service.AnonymizeUserData(ctx, req.UserId); err != nil {
		h.logger.Error("handler: AnonymizeUserData failed", zap.Error(err))
		return &bookingpb.AnonymizeUserDataResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &bookingpb.AnonymizeUserDataResponse{Success: true}, nil
}

// GetPassengerBookingIDs retourne tous les IDs de réservation d'un passager.
func (h *BookingHandler) GetPassengerBookingIDs(ctx context.Context, req *bookingpb.GetPassengerBookingIDsRequest) (*bookingpb.GetPassengerBookingIDsResponse, error) {
	ids, err := h.service.GetPassengerBookingIDs(ctx, req.UserId)
	if err != nil {
		h.logger.Error("handler: GetPassengerBookingIDs failed", zap.Error(err))
		return &bookingpb.GetPassengerBookingIDsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}
	return &bookingpb.GetPassengerBookingIDsResponse{BookingIds: ids}, nil
}

// Health retourne l'état de santé du service.
func (h *BookingHandler) Health(ctx context.Context, req *bookingpb.HealthRequest) (*bookingpb.HealthResponse, error) {
	return &bookingpb.HealthResponse{
		Status:    "ok",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

func toProtoBookingPreviews(results []*serviceInterfaces.BookingPreviewResult) []*bookingpb.BookingPreview {
	previews := make([]*bookingpb.BookingPreview, 0, len(results))
	for _, r := range results {
		previews = append(previews, &bookingpb.BookingPreview{
			BookingId:           r.BookingID,
			BookingReference:    r.BookingReference,
			TripId:              r.TripID,
			Status:              r.Status,
			SeatsBooked:         int32(r.SeatsBooked),
			TotalAmount:         int32(r.TotalAmount),
			PickupLocationName:  r.PickupLocationName,
			DropoffLocationName: r.DropoffLocationName,
			DepartureDate:       r.DepartureDate,
			DepartureTime:       r.DepartureTime,
		})
	}
	return previews
}

func toProtoDriverBookingPreviews(results []*serviceInterfaces.DriverBookingPreviewResult) []*bookingpb.DriverBookingPreview {
	previews := make([]*bookingpb.DriverBookingPreview, 0, len(results))
	for _, r := range results {
		previews = append(previews, &bookingpb.DriverBookingPreview{
			BookingId:           r.BookingID,
			BookingReference:    r.BookingReference,
			TripId:              r.TripID,
			Status:              r.Status,
			SeatsBooked:         int32(r.SeatsBooked),
			TotalAmount:         int32(r.TotalAmount),
			PickupLocationName:  r.PickupLocationName,
			DropoffLocationName: r.DropoffLocationName,
			DepartureDate:       r.DepartureDate,
			DepartureTime:       r.DepartureTime,
			PassengerName:       r.PassengerName,
			PassengerRating:     r.PassengerRating,
			PassengerTripCount:  int32(r.PassengerTripCount),
			IsPassengerVerified: r.IsPassengerVerified,
			PassengerMessage:    r.PassengerMessage,
			PaymentMethod:       r.PaymentMethod,
			CreatedAt:           r.CreatedAt,
			ExtraMinutesDetour:  int32(r.ExtraMinutesDetour),
		})
	}
	return previews
}

func toProtoBookingCounts(c *serviceInterfaces.BookingCountsResult) *bookingpb.BookingCounts {
	if c == nil {
		return &bookingpb.BookingCounts{}
	}
	return &bookingpb.BookingCounts{
		Pending:   c.Pending,
		Approved:  c.Approved,
		Rejected:  c.Rejected,
		Cancelled: c.Cancelled,
	}
}

// ListBookings retourne la liste paginée et filtrée des réservations (vue support).
func (h *BookingHandler) ListBookings(ctx context.Context, req *bookingpb.ListBookingsRequest) (*bookingpb.ListBookingsResponse, error) {
	h.logger.Debug("handler: ListBookings called (support)", zap.String("status", req.Status), zap.String("bookingReference", req.BookingReference))

	result, err := h.service.ListBookingsAdmin(ctx, &serviceInterfaces.ListBookingsAdminInput{
		Status:           req.Status,
		BookingReference: req.BookingReference,
		DateFrom:         req.DateFrom,
		DateTo:           req.DateTo,
		PageIndex:        int(req.Index),
		PageSize:         int(req.PageSize),
	})
	if err != nil {
		return &bookingpb.ListBookingsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	pbBookings := make([]*bookingpb.AdminBookingPreview, 0, len(result.Bookings))
	for _, b := range result.Bookings {
		pbBookings = append(pbBookings, &bookingpb.AdminBookingPreview{
			BookingId:           b.BookingID,
			BookingReference:    b.BookingReference,
			TripId:              b.TripID,
			PassengerId:         b.PassengerID,
			DriverId:            b.DriverID,
			PassengerName:       b.PassengerName,
			DriverName:          b.DriverName,
			Status:              b.Status,
			SeatsBooked:         int32(b.SeatsBooked),
			TotalAmount:         int32(b.TotalAmount),
			PaymentMethod:       b.PaymentMethod,
			PickupLocationName:  b.PickupLocationName,
			DropoffLocationName: b.DropoffLocationName,
			DepartureDatetime:   b.DepartureDatetime,
			CreatedAt:           b.CreatedAt,
		})
	}

	return &bookingpb.ListBookingsResponse{
		Bookings: pbBookings,
		Total:    int32(result.Total),
	}, nil
}

// GetBookingDetailAdmin retourne le détail complet d'une réservation (vue support).
func (h *BookingHandler) GetBookingDetailAdmin(ctx context.Context, req *bookingpb.GetBookingDetailAdminRequest) (*bookingpb.GetBookingDetailAdminResponse, error) {
	h.logger.Debug("handler: GetBookingDetailAdmin called (support)", zap.String("bookingID", req.BookingId))

	result, err := h.service.GetBookingDetailAdmin(ctx, req.BookingId)
	if err != nil {
		return &bookingpb.GetBookingDetailAdminResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &bookingpb.GetBookingDetailAdminResponse{
		Booking:       toProtoBookingDetail(result.Booking),
		PassengerName: result.PassengerName,
		DriverName:    result.DriverName,
	}, nil
}

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, bookingErrors.ErrorInvalidInput),
		errors.Is(err, bookingErrors.ErrorInvalidWaypoints):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, bookingErrors.ErrorBookingNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, bookingErrors.ErrorTripNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, bookingErrors.ErrorPassengerNotFound),
		errors.Is(err, bookingErrors.ErrorDriverNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, bookingErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, bookingErrors.ErrorPassengerNotVerified):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, bookingErrors.ErrorTripNotAvailable),
		errors.Is(err, bookingErrors.ErrorNoSeatsAvailable),
		errors.Is(err, bookingErrors.ErrorPassengerIsDriver),
		errors.Is(err, bookingErrors.ErrorDuplicateBooking),
		errors.Is(err, bookingErrors.ErrorInvalidStatusTransition),
		errors.Is(err, bookingErrors.ErrorBookingAlreadyCancelled),
		errors.Is(err, bookingErrors.ErrorBookingNotPending),
		errors.Is(err, bookingErrors.ErrorBookingNotApproved),
		errors.Is(err, bookingErrors.ErrorBookingNotInProgress),
		errors.Is(err, bookingErrors.ErrorPaymentAlreadyConfirmed),
		errors.Is(err, bookingErrors.ErrorPaymentAlreadyFailed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, bookingErrors.ErrorSeatReservationFailed):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, bookingErrors.ErrorDataRetrievalFailed),
		errors.Is(err, bookingErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
