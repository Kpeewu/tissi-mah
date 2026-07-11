package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

type NotificationServiceImpl struct {
	inboxRepo      repoInterfaces.InboxRepository
	preferenceRepo repoInterfaces.PreferenceRepository
	deviceRepo     repoInterfaces.DeviceTokenRepository
	userClient     client.UserClient
	logger         *zap.Logger
}

func NewNotificationService(
	inboxRepo repoInterfaces.InboxRepository,
	preferenceRepo repoInterfaces.PreferenceRepository,
	deviceRepo repoInterfaces.DeviceTokenRepository,
	userClient client.UserClient,
	logger *zap.Logger,
) *NotificationServiceImpl {
	return &NotificationServiceImpl{
		inboxRepo:      inboxRepo,
		preferenceRepo: preferenceRepo,
		deviceRepo:     deviceRepo,
		userClient:     userClient,
		logger:         logger,
	}
}

// resolveInternalUserID traduit le Firebase UID reçu depuis l'api-gateway en UserID
// interne via user-service. Les device tokens et l'inbox sont keyés sur l'UserID interne
// (comme le fait le dispatcher) — cette résolution doit précéder tout accès repo.
func (s *NotificationServiceImpl) resolveInternalUserID(ctx context.Context, firebaseUID string) (string, error) {
	internalID, err := s.userClient.GetUserIDByFirebaseID(ctx, firebaseUID)
	if err != nil {
		s.logger.Error("failed to resolve firebaseUID to internal userID",
			zap.String("firebase_uid", firebaseUID),
			zap.Error(err),
		)
		return "", err
	}
	return internalID, nil
}

// === Inbox ===

func (s *NotificationServiceImpl) GetInbox(ctx context.Context, firebaseUID string, page, pageSize int) ([]*domain.InboxEntry, int, error) {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return s.inboxRepo.GetByUserID(ctx, userID, page, pageSize)
}

func (s *NotificationServiceImpl) MarkAsRead(ctx context.Context, inboxID, firebaseUID string) error {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return err
	}
	return s.inboxRepo.MarkAsRead(ctx, inboxID, userID)
}

func (s *NotificationServiceImpl) MarkAllAsRead(ctx context.Context, firebaseUID string) (int, error) {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return 0, err
	}
	return s.inboxRepo.MarkAllAsRead(ctx, userID)
}

func (s *NotificationServiceImpl) GetUnreadCount(ctx context.Context, firebaseUID string) (int, error) {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return 0, err
	}
	return s.inboxRepo.GetUnreadCount(ctx, userID)
}

// === Preferences ===

func (s *NotificationServiceImpl) GetPreferences(ctx context.Context, firebaseUID string) (*domain.UserNotificationPreference, error) {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return nil, err
	}
	return s.preferenceRepo.GetByUserID(ctx, userID)
}

func (s *NotificationServiceImpl) UpdatePreferences(ctx context.Context, firebaseUID string, pushEnabled, emailEnabled bool) error {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return err
	}
	pref := &domain.UserNotificationPreference{
		UserID:       userID,
		PushEnabled:  pushEnabled,
		EmailEnabled: emailEnabled,
	}
	return s.preferenceRepo.Upsert(ctx, pref)
}

// === Device tokens ===

func (s *NotificationServiceImpl) RegisterDeviceToken(ctx context.Context, firebaseUID, fcmToken, platform, deviceName string) (string, error) {
	userID, err := s.resolveInternalUserID(ctx, firebaseUID)
	if err != nil {
		return "", err
	}
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
