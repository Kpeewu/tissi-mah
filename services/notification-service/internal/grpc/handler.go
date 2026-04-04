package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/middleware"
	svcInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/service/interfaces"
	notifpb "github.com/Kpeewu/tissi-mah/services/notification-service/proto/gen"
)

const serviceVersion = "1.0.0"

type NotificationHandler struct {
	notifpb.UnimplementedNotificationServiceServer
	service svcInterfaces.NotificationService
	logger  *zap.Logger
}

func NewNotificationHandler(service svcInterfaces.NotificationService, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{service: service, logger: logger}
}

func (h *NotificationHandler) getFirebaseUID(ctx context.Context) (string, error) {
	uid, ok := ctx.Value(middleware.FirebaseIDKey).(string)
	if !ok || uid == "" {
		return "", status.Error(codes.Unauthenticated, "missing firebase uid")
	}
	return uid, nil
}

// === Inbox ===

func (h *NotificationHandler) GetInbox(ctx context.Context, req *notifpb.GetInboxRequest) (*notifpb.GetInboxResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	entries, total, err := h.service.GetInbox(ctx, uid, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get inbox")
	}

	pbEntries := make([]*notifpb.InboxEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &notifpb.InboxEntry{
			InboxId:    e.InboxID,
			EventType:  e.EventType,
			Title:      e.Title,
			Body:       e.Body,
			ActionType: e.ActionType,
			ActionId:   e.ActionID,
			IsRead:     e.IsRead,
			CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return &notifpb.GetInboxResponse{
		Entries:    pbEntries,
		TotalCount: int32(total),
	}, nil
}

func (h *NotificationHandler) MarkAsRead(ctx context.Context, req *notifpb.MarkAsReadRequest) (*notifpb.MarkAsReadResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.service.MarkAsRead(ctx, req.InboxId, uid); err != nil {
		return nil, status.Error(codes.Internal, "failed to mark as read")
	}

	return &notifpb.MarkAsReadResponse{Success: true}, nil
}

func (h *NotificationHandler) MarkAllAsRead(ctx context.Context, req *notifpb.MarkAllAsReadRequest) (*notifpb.MarkAllAsReadResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	count, err := h.service.MarkAllAsRead(ctx, uid)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to mark all as read")
	}

	return &notifpb.MarkAllAsReadResponse{UpdatedCount: int32(count)}, nil
}

func (h *NotificationHandler) GetUnreadCount(ctx context.Context, req *notifpb.GetUnreadCountRequest) (*notifpb.GetUnreadCountResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	count, err := h.service.GetUnreadCount(ctx, uid)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get unread count")
	}

	return &notifpb.GetUnreadCountResponse{Count: int32(count)}, nil
}

// === Preferences ===

func (h *NotificationHandler) GetPreferences(ctx context.Context, req *notifpb.GetPreferencesRequest) (*notifpb.GetPreferencesResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	prefs, err := h.service.GetPreferences(ctx, uid)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get preferences")
	}

	return &notifpb.GetPreferencesResponse{
		PushEnabled:  prefs.PushEnabled,
		EmailEnabled: prefs.EmailEnabled,
	}, nil
}

func (h *NotificationHandler) UpdatePreferences(ctx context.Context, req *notifpb.UpdatePreferencesRequest) (*notifpb.UpdatePreferencesResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.service.UpdatePreferences(ctx, uid, req.PushEnabled, req.EmailEnabled); err != nil {
		return nil, status.Error(codes.Internal, "failed to update preferences")
	}

	return &notifpb.UpdatePreferencesResponse{Success: true}, nil
}

// === Device Tokens ===

func (h *NotificationHandler) RegisterDeviceToken(ctx context.Context, req *notifpb.RegisterDeviceTokenRequest) (*notifpb.RegisterDeviceTokenResponse, error) {
	uid, err := h.getFirebaseUID(ctx)
	if err != nil {
		return nil, err
	}

	if req.FcmToken == "" {
		return nil, status.Error(codes.InvalidArgument, "fcm_token is required")
	}

	tokenID, err := h.service.RegisterDeviceToken(ctx, uid, req.FcmToken, req.Platform, req.DeviceName)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to register device token")
	}

	return &notifpb.RegisterDeviceTokenResponse{
		Success: true,
		TokenId: tokenID,
	}, nil
}

func (h *NotificationHandler) UnregisterDeviceToken(ctx context.Context, req *notifpb.UnregisterDeviceTokenRequest) (*notifpb.UnregisterDeviceTokenResponse, error) {
	if req.FcmToken == "" {
		return nil, status.Error(codes.InvalidArgument, "fcm_token is required")
	}

	if err := h.service.UnregisterDeviceToken(ctx, req.FcmToken); err != nil {
		return nil, status.Error(codes.Internal, "failed to unregister device token")
	}

	return &notifpb.UnregisterDeviceTokenResponse{Success: true}, nil
}

// === Inter-service ===

func (h *NotificationHandler) InvalidateDeviceToken(ctx context.Context, req *notifpb.InvalidateDeviceTokenRequest) (*notifpb.InvalidateDeviceTokenResponse, error) {
	if req.FcmToken == "" {
		return nil, status.Error(codes.InvalidArgument, "fcm_token is required")
	}

	if err := h.service.InvalidateDeviceToken(ctx, req.FcmToken); err != nil {
		return nil, status.Error(codes.Internal, "failed to invalidate device token")
	}

	return &notifpb.InvalidateDeviceTokenResponse{Success: true}, nil
}

// === Health ===

func (h *NotificationHandler) Health(ctx context.Context, req *notifpb.HealthRequest) (*notifpb.HealthResponse, error) {
	return &notifpb.HealthResponse{
		Status:  "ok",
		Version: serviceVersion,
		Service: "notification-service",
	}, nil
}
