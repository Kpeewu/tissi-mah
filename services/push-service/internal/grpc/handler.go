package grpc

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/push-service/internal/fcm"
	pushpb "github.com/Kpeewu/tissi-mah/services/push-service/proto/gen"
	"go.uber.org/zap"
)

const serviceVersion = "1.0.0"

// PushHandler implémente pushpb.PushServiceServer.
type PushHandler struct {
	pushpb.UnimplementedPushServiceServer
	fcmClient fcm.FCMClient
	logger    *zap.Logger
}

func NewPushHandler(fcmClient fcm.FCMClient, logger *zap.Logger) *PushHandler {
	return &PushHandler{fcmClient: fcmClient, logger: logger}
}

// SendPush envoie une notification push à un seul token FCM.
func (h *PushHandler) SendPush(ctx context.Context, req *pushpb.SendPushRequest) (*pushpb.SendPushResponse, error) {
	h.logger.Debug("handler: SendPush called",
		zap.String("title", req.Title),
		zap.String("token", req.FcmToken[:min(len(req.FcmToken), 10)]+"..."),
	)

	if req.FcmToken == "" || req.Title == "" {
		return &pushpb.SendPushResponse{
			Success:      false,
			ErrorCode:    "INVALID_INPUT",
			ErrorMessage: "fcm_token and title are required",
		}, nil
	}

	err := h.fcmClient.Send(ctx, req.Title, req.Body, req.FcmToken, req.Data)
	if err != nil {
		h.logger.Error("handler: SendPush failed", zap.Error(err))
		return &pushpb.SendPushResponse{
			Success:      false,
			ErrorCode:    "FCM_FAILED",
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: SendPush success", zap.String("title", req.Title))

	return &pushpb.SendPushResponse{
		Success: true,
	}, nil
}

// SendPushMulticast envoie une notification push à plusieurs tokens FCM (max 500).
func (h *PushHandler) SendPushMulticast(ctx context.Context, req *pushpb.SendPushMulticastRequest) (*pushpb.SendPushMulticastResponse, error) {
	h.logger.Debug("handler: SendPushMulticast called",
		zap.String("title", req.Title),
		zap.Int("tokens_count", len(req.FcmTokens)),
	)

	if len(req.FcmTokens) == 0 || req.Title == "" {
		return &pushpb.SendPushMulticastResponse{
			SuccessCount: 0,
			FailureCount: 0,
		}, nil
	}

	results, err := h.fcmClient.SendMulticast(ctx, req.Title, req.Body, req.FcmTokens, req.Data)
	if err != nil {
		h.logger.Error("handler: SendPushMulticast failed", zap.Error(err))
		return &pushpb.SendPushMulticastResponse{
			SuccessCount: 0,
			FailureCount: int32(len(req.FcmTokens)),
		}, nil
	}

	var successCount, failureCount int32
	pbResults := make([]*pushpb.PushResult, len(results))
	for i, r := range results {
		if r.Success {
			successCount++
		} else {
			failureCount++
		}
		pbResults[i] = &pushpb.PushResult{
			FcmToken:  r.Token,
			Success:   r.Success,
			ErrorCode: r.ErrorCode,
		}
	}

	h.logger.Info("handler: SendPushMulticast done",
		zap.Int32("success", successCount),
		zap.Int32("failure", failureCount),
	)

	return &pushpb.SendPushMulticastResponse{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Results:      pbResults,
	}, nil
}

// Health retourne l'état de santé du service.
func (h *PushHandler) Health(ctx context.Context, req *pushpb.HealthRequest) (*pushpb.HealthResponse, error) {
	return &pushpb.HealthResponse{
		Status:  "ok",
		Version: serviceVersion,
		Service: "push-service",
	}, nil
}
