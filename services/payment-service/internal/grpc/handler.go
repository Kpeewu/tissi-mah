package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/middleware"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/payment-service/internal/service/interfaces"
	paymentErrors "github.com/Kpeewu/tissi-mah/services/payment-service/pkg/errors"
	paymentpb "github.com/Kpeewu/tissi-mah/services/payment-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceVersion = "1.0.0"

// PaymentHandler implémente paymentpb.PaymentServiceServer.
type PaymentHandler struct {
	paymentpb.UnimplementedPaymentServiceServer
	service serviceInterfaces.PaymentService
	logger  *zap.Logger
}

func NewPaymentHandler(service serviceInterfaces.PaymentService, logger *zap.Logger) *PaymentHandler {
	return &PaymentHandler{service: service, logger: logger}
}

// CreatePayment crée un paiement pour une réservation.
func (h *PaymentHandler) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentpb.CreatePaymentResponse, error) {
	h.logger.Debug("handler: CreatePayment called", zap.String("bookingID", req.BookingId))

	result, err := h.service.CreatePayment(ctx, &serviceInterfaces.CreatePaymentInput{
		BookingID:           req.BookingId,
		TripID:              req.TripId,
		PaymentMethod:       req.PaymentMethod,
		Amount:              int(req.Amount),
		PassengerPhoneNumber: req.PassengerPhoneNumber,
		MobileMoneyMode:     req.MobileMoneyMode,
	})
	if err != nil {
		h.logger.Error("handler: CreatePayment failed", zap.Error(err))
		return &paymentpb.CreatePaymentResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.CreatePaymentResponse{
		PaymentId:        result.PaymentID,
		PaymentReference: result.PaymentReference,
		Status:           result.Status,
	}, nil
}

// GetPaymentStatus retourne le statut d'un paiement.
func (h *PaymentHandler) GetPaymentStatus(ctx context.Context, req *paymentpb.GetPaymentStatusRequest) (*paymentpb.GetPaymentStatusResponse, error) {
	result, err := h.service.GetPaymentStatus(ctx, req.PaymentId)
	if err != nil {
		return &paymentpb.GetPaymentStatusResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.GetPaymentStatusResponse{
		PaymentId:        result.PaymentID,
		BookingId:        result.BookingID,
		Amount:           int32(result.Amount),
		PaymentMethod:    result.PaymentMethod,
		Status:           result.Status,
		PaymentReference: result.PaymentReference,
		CreatedAt:        result.CreatedAt,
		CompletedAt:      result.CompletedAt,
	}, nil
}

// GetPaymentByBooking retourne le paiement associé à une réservation.
func (h *PaymentHandler) GetPaymentByBooking(ctx context.Context, req *paymentpb.GetPaymentByBookingRequest) (*paymentpb.GetPaymentByBookingResponse, error) {
	result, err := h.service.GetPaymentByBooking(ctx, req.BookingId)
	if err != nil {
		return &paymentpb.GetPaymentByBookingResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.GetPaymentByBookingResponse{
		PaymentId:        result.PaymentID,
		BookingId:        result.BookingID,
		Amount:           int32(result.Amount),
		PaymentMethod:    result.PaymentMethod,
		Status:           result.Status,
		PaymentReference: result.PaymentReference,
		CreatedAt:        result.CreatedAt,
	}, nil
}

// ProcessWebhook traite les webhooks FedaPay.
func (h *PaymentHandler) ProcessWebhook(ctx context.Context, req *paymentpb.ProcessWebhookRequest) (*paymentpb.ProcessWebhookResponse, error) {
	err := h.service.ProcessWebhook(ctx, &serviceInterfaces.ProcessWebhookInput{
		Signature:  req.Signature,
		RawPayload: req.RawPayload,
	})
	if err != nil {
		h.logger.Error("handler: ProcessWebhook failed", zap.Error(err))
		return &paymentpb.ProcessWebhookResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.ProcessWebhookResponse{Success: true}, nil
}

// RequestRefund crée un remboursement.
func (h *PaymentHandler) RequestRefund(ctx context.Context, req *paymentpb.RequestRefundRequest) (*paymentpb.RequestRefundResponse, error) {
	result, err := h.service.RequestRefund(ctx, &serviceInterfaces.RequestRefundInput{
		BookingID:         req.BookingId,
		RefundReason:      req.RefundReason,
		OriginalAmount:    int(req.OriginalAmount),
		ServiceFee:        int(req.ServiceFee),
		DepartureDatetime: req.DepartureDatetime,
		ApprovedAt:        req.ApprovedAt,
		CancelledAt:       req.CancelledAt,
	})
	if err != nil {
		return &paymentpb.RequestRefundResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.RequestRefundResponse{
		RefundId:        result.RefundID,
		RefundReference: result.RefundReference,
		Status:          result.Status,
		RefundAmount:    int32(result.RefundAmount),
	}, nil
}

// GetRefundStatus retourne le statut d'un remboursement.
func (h *PaymentHandler) GetRefundStatus(ctx context.Context, req *paymentpb.GetRefundStatusRequest) (*paymentpb.GetRefundStatusResponse, error) {
	result, err := h.service.GetRefundStatus(ctx, req.RefundId)
	if err != nil {
		return &paymentpb.GetRefundStatusResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.GetRefundStatusResponse{
		RefundId:          result.RefundID,
		BookingId:         result.BookingID,
		RefundReason:      result.RefundReason,
		RefundRule:        result.RefundRule,
		OriginalAmount:    int32(result.OriginalAmount),
		RefundPercentage:  int32(result.RefundPercentage),
		RefundAmount:      int32(result.RefundAmount),
		AmountToPassenger: int32(result.AmountToPassenger),
		AmountToDriver:    int32(result.AmountToDriver),
		AmountToPlatform:  int32(result.AmountToPlatform),
		Status:            result.Status,
		ProcessedAt:       result.ProcessedAt,
		CompletedAt:       result.CompletedAt,
	}, nil
}

// ReleasePayment libère un paiement held.
func (h *PaymentHandler) ReleasePayment(ctx context.Context, req *paymentpb.ReleasePaymentRequest) (*paymentpb.ReleasePaymentResponse, error) {
	err := h.service.ReleasePayment(ctx, req.BookingId)
	if err != nil {
		return &paymentpb.ReleasePaymentResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.ReleasePaymentResponse{Success: true, Status: "released"}, nil
}

// GetPayoutStatus retourne le statut d'un payout.
func (h *PaymentHandler) GetPayoutStatus(ctx context.Context, req *paymentpb.GetPayoutStatusRequest) (*paymentpb.GetPayoutStatusResponse, error) {
	result, err := h.service.GetPayoutStatus(ctx, req.PayoutId)
	if err != nil {
		return &paymentpb.GetPayoutStatusResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.GetPayoutStatusResponse{
		PayoutId:      result.PayoutID,
		DriverId:      result.DriverID,
		TripId:        result.TripID,
		GrossAmount:   int32(result.GrossAmount),
		PlatformFee:   int32(result.PlatformFee),
		NetAmount:     int32(result.NetAmount),
		Status:        result.Status,
		ScheduledAt:   result.ScheduledAt,
		CompletedAt:   result.CompletedAt,
		FailureReason: result.FailureReason,
	}, nil
}

// TriggerManualPayout permet à un agent support de lancer manuellement un payout pour un trajet.
func (h *PaymentHandler) TriggerManualPayout(ctx context.Context, req *paymentpb.TriggerManualPayoutRequest) (*paymentpb.TriggerManualPayoutResponse, error) {
	supportUID, ok := ctx.Value(middleware.SupportUIDKey).(string)
	if !ok || supportUID == "" {
		return &paymentpb.TriggerManualPayoutResponse{Success: false, ErrorMessage: paymentErrors.ErrorUnauthorized.Error()},
			status.Error(codes.Unauthenticated, paymentErrors.ErrorUnauthorized.Error())
	}

	netAmount, err := h.service.TriggerManualPayout(ctx, req.TripId, supportUID)
	if err != nil {
		h.logger.Error("handler: TriggerManualPayout failed", zap.Error(err), zap.String("tripID", req.TripId))
		return &paymentpb.TriggerManualPayoutResponse{Success: false, ErrorMessage: err.Error()}, toGRPCError(err)
	}

	return &paymentpb.TriggerManualPayoutResponse{
		Success:   true,
		NetAmount: int32(netAmount),
	}, nil
}

// GetDriverPayouts retourne les payouts d'un chauffeur.
func (h *PaymentHandler) GetDriverPayouts(ctx context.Context, req *paymentpb.GetDriverPayoutsRequest) (*paymentpb.GetDriverPayoutsResponse, error) {
	results, err := h.service.GetDriverPayouts(ctx, req.DriverId, int(req.PageIndex))
	if err != nil {
		return &paymentpb.GetDriverPayoutsResponse{ErrorMessage: err.Error()}, toGRPCError(err)
	}

	payouts := make([]*paymentpb.PayoutPreview, 0, len(results))
	for _, r := range results {
		payouts = append(payouts, &paymentpb.PayoutPreview{
			PayoutId:        r.PayoutID,
			PayoutReference: r.PayoutReference,
			TripId:          r.TripID,
			NetAmount:       int32(r.NetAmount),
			Status:          r.Status,
			CompletedAt:     r.CompletedAt,
			CreatedAt:       r.CreatedAt,
		})
	}

	return &paymentpb.GetDriverPayoutsResponse{Payouts: payouts}, nil
}

// Health retourne l'état de santé du service.
func (h *PaymentHandler) Health(ctx context.Context, req *paymentpb.HealthRequest) (*paymentpb.HealthResponse, error) {
	return &paymentpb.HealthResponse{
		Status:    "ok",
		Version:   serviceVersion,
		Timestamp: time.Now().Unix(),
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

// toGRPCError traduit les erreurs domaine en codes de statut gRPC.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, paymentErrors.ErrorInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, paymentErrors.ErrorPaymentNotFound),
		errors.Is(err, paymentErrors.ErrorRefundNotFound),
		errors.Is(err, paymentErrors.ErrorPayoutNotFound),
		errors.Is(err, paymentErrors.ErrorBookingNotFound),
		errors.Is(err, paymentErrors.ErrorUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, paymentErrors.ErrorUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, paymentErrors.ErrorDuplicatePayment):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, paymentErrors.ErrorPaymentAlreadyProcessed),
		errors.Is(err, paymentErrors.ErrorPaymentAlreadyHeld),
		errors.Is(err, paymentErrors.ErrorPaymentAlreadyFailed),
		errors.Is(err, paymentErrors.ErrorInvalidPaymentStatus),
		errors.Is(err, paymentErrors.ErrorRefundAlreadyProcessed),
		errors.Is(err, paymentErrors.ErrorRefundAmountExceedsPayment),
		errors.Is(err, paymentErrors.ErrorPayoutAlreadyExists),
		errors.Is(err, paymentErrors.ErrorDuplicateWebhookEvent):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, paymentErrors.ErrorInsufficientBalance):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, paymentErrors.ErrorTripNotReadyForPayout):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, paymentErrors.ErrorFedaPayAPIError),
		errors.Is(err, paymentErrors.ErrorWebhookVerificationFailed),
		errors.Is(err, paymentErrors.ErrorSupportServiceUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, paymentErrors.ErrorDataRetrievalFailed),
		errors.Is(err, paymentErrors.ErrorInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
