package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/service"
	"github.com/Kpeewu/tissi-mah/services/notification-service/tests/mocks"
)

// newService crée un service avec les mocks injectés.
func newService() (*service.NotificationServiceImpl, *mocks.MockInboxRepository, *mocks.MockPreferenceRepository, *mocks.MockDeviceTokenRepository) {
	inboxRepo := new(mocks.MockInboxRepository)
	prefRepo := new(mocks.MockPreferenceRepository)
	deviceRepo := new(mocks.MockDeviceTokenRepository)
	svc := service.NewNotificationService(inboxRepo, prefRepo, deviceRepo, zap.NewNop())
	return svc, inboxRepo, prefRepo, deviceRepo
}

// =============================================================================
// GetInbox
// =============================================================================

func TestGetInbox_NormalizesPage(t *testing.T) {
	svc, inboxRepo, _, _ := newService()
	ctx := context.Background()

	entries := []*domain.InboxEntry{{InboxID: "inbox-1", Title: "Test"}}
	// page < 1 doit être normalisé à 1
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return(entries, 1, nil)

	result, total, err := svc.GetInbox(ctx, "user-abc", 0, 20)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_NormalizesPageSize(t *testing.T) {
	svc, inboxRepo, _, _ := newService()
	ctx := context.Background()

	entries := []*domain.InboxEntry{}
	// pageSize > 50 doit être normalisé à 20
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return(entries, 0, nil)

	_, _, err := svc.GetInbox(ctx, "user-abc", 1, 100)
	require.NoError(t, err)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_NormalizesPageSizeZero(t *testing.T) {
	svc, inboxRepo, _, _ := newService()
	ctx := context.Background()

	// pageSize < 1 doit aussi être normalisé à 20
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return([]*domain.InboxEntry{}, 0, nil)

	_, _, err := svc.GetInbox(ctx, "user-abc", 1, 0)
	require.NoError(t, err)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_Success(t *testing.T) {
	svc, inboxRepo, _, _ := newService()
	ctx := context.Background()

	entries := []*domain.InboxEntry{
		{InboxID: "inbox-1", Title: "Notification 1"},
		{InboxID: "inbox-2", Title: "Notification 2"},
	}
	inboxRepo.On("GetByUserID", mock.Anything, "user-xyz", 2, 10).
		Return(entries, 15, nil)

	result, total, err := svc.GetInbox(ctx, "user-xyz", 2, 10)

	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.Len(t, result, 2)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_RepoError(t *testing.T) {
	svc, inboxRepo, _, _ := newService()
	ctx := context.Background()

	repoErr := errors.New("database error")
	inboxRepo.On("GetByUserID", mock.Anything, "user-err", 1, 20).
		Return(nil, 0, repoErr)

	result, _, err := svc.GetInbox(ctx, "user-err", 1, 20)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoErr)
}

// =============================================================================
// UpdatePreferences
// =============================================================================

func TestUpdatePreferences_Success(t *testing.T) {
	svc, _, prefRepo, _ := newService()
	ctx := context.Background()

	prefRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(p *domain.UserNotificationPreference) bool {
		return p.UserID == "user-pref" && p.PushEnabled == false && p.EmailEnabled == true
	})).Return(nil)

	err := svc.UpdatePreferences(ctx, "user-pref", false, true)
	require.NoError(t, err)
	prefRepo.AssertExpectations(t)
}

func TestUpdatePreferences_RepoError(t *testing.T) {
	svc, _, prefRepo, _ := newService()
	ctx := context.Background()

	repoErr := errors.New("upsert failed")
	prefRepo.On("Upsert", mock.Anything, mock.Anything).Return(repoErr)

	err := svc.UpdatePreferences(ctx, "user-pref", true, true)
	assert.ErrorIs(t, err, repoErr)
}

// =============================================================================
// RegisterDeviceToken
// =============================================================================

func TestRegisterDeviceToken_Success(t *testing.T) {
	svc, _, _, deviceRepo := newService()
	ctx := context.Background()

	deviceRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(t *domain.UserDeviceToken) bool {
		return t.UserID == "user-dev" && t.FCMToken == "fcm-xyz" && t.Platform == "android"
	})).Return("token-id-001", nil)

	tokenID, err := svc.RegisterDeviceToken(ctx, "user-dev", "fcm-xyz", "android", "Pixel 8")
	require.NoError(t, err)
	assert.Equal(t, "token-id-001", tokenID)
	deviceRepo.AssertExpectations(t)
}

func TestRegisterDeviceToken_RepoError(t *testing.T) {
	svc, _, _, deviceRepo := newService()
	ctx := context.Background()

	repoErr := errors.New("insert failed")
	deviceRepo.On("Upsert", mock.Anything, mock.Anything).Return("", repoErr)

	tokenID, err := svc.RegisterDeviceToken(ctx, "user-dev", "fcm-xyz", "android", "Device")
	assert.Empty(t, tokenID)
	assert.ErrorIs(t, err, repoErr)
}

// =============================================================================
// InvalidateDeviceToken
// =============================================================================

func TestInvalidateDeviceToken_Success(t *testing.T) {
	svc, _, _, deviceRepo := newService()
	ctx := context.Background()

	deviceRepo.On("InvalidateByFCMToken", mock.Anything, "fcm-old").Return(nil)

	err := svc.InvalidateDeviceToken(ctx, "fcm-old")
	require.NoError(t, err)
	deviceRepo.AssertExpectations(t)
}

func TestInvalidateDeviceToken_RepoError(t *testing.T) {
	svc, _, _, deviceRepo := newService()
	ctx := context.Background()

	repoErr := errors.New("update failed")
	deviceRepo.On("InvalidateByFCMToken", mock.Anything, "fcm-old").Return(repoErr)

	err := svc.InvalidateDeviceToken(ctx, "fcm-old")
	assert.ErrorIs(t, err, repoErr)
}
