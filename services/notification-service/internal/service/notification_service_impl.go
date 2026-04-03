package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

type NotificationServiceImpl struct {
	inboxRepo      repoInterfaces.InboxRepository
	preferenceRepo repoInterfaces.PreferenceRepository
	deviceRepo     repoInterfaces.DeviceTokenRepository
	logger         *zap.Logger
}

func NewNotificationService(
	inboxRepo repoInterfaces.InboxRepository,
	preferenceRepo repoInterfaces.PreferenceRepository,
	deviceRepo repoInterfaces.DeviceTokenRepository,
	logger *zap.Logger,
) *NotificationServiceImpl {
	return &NotificationServiceImpl{
		inboxRepo:      inboxRepo,
		preferenceRepo: preferenceRepo,
		deviceRepo:     deviceRepo,
		logger:         logger,
	}
}

// === Inbox ===

func (s *NotificationServiceImpl) GetInbox(ctx context.Context, userID string, page, pageSize int) ([]*domain.InboxEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return s.inboxRepo.GetByUserID(ctx, userID, page, pageSize)
}

func (s *NotificationServiceImpl) MarkAsRead(ctx context.Context, inboxID, userID string) error {
	return s.inboxRepo.MarkAsRead(ctx, inboxID, userID)
}

func (s *NotificationServiceImpl) MarkAllAsRead(ctx context.Context, userID string) (int, error) {
	return s.inboxRepo.MarkAllAsRead(ctx, userID)
}

func (s *NotificationServiceImpl) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return s.inboxRepo.GetUnreadCount(ctx, userID)
}

// === Preferences ===

func (s *NotificationServiceImpl) GetPreferences(ctx context.Context, userID string) (*domain.UserNotificationPreference, error) {
	return s.preferenceRepo.GetByUserID(ctx, userID)
}

func (s *NotificationServiceImpl) UpdatePreferences(ctx context.Context, userID string, pushEnabled, emailEnabled bool) error {
	pref := &domain.UserNotificationPreference{
		UserID:       userID,
		PushEnabled:  pushEnabled,
		EmailEnabled: emailEnabled,
	}
	return s.preferenceRepo.Upsert(ctx, pref)
}

// === Device tokens ===

func (s *NotificationServiceImpl) RegisterDeviceToken(ctx context.Context, userID, fcmToken, platform, deviceName string) (string, error) {
	token := &domain.UserDeviceToken{
		UserID:     userID,
		FCMToken:   fcmToken,
		Platform:   platform,
		DeviceName: deviceName,
	}
	return s.deviceRepo.Upsert(ctx, token)
}

func (s *NotificationServiceImpl) UnregisterDeviceToken(ctx context.Context, fcmToken string) error {
	return s.deviceRepo.DeleteByFCMToken(ctx, fcmToken)
}

func (s *NotificationServiceImpl) InvalidateDeviceToken(ctx context.Context, fcmToken string) error {
	return s.deviceRepo.InvalidateByFCMToken(ctx, fcmToken)
}
