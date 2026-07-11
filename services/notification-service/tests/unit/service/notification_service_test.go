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
func newService() (*service.NotificationServiceImpl, *mocks.MockInboxRepository, *mocks.MockPreferenceRepository, *mocks.MockDeviceTokenRepository, *mocks.MockUserClient) {
	inboxRepo := new(mocks.MockInboxRepository)
	prefRepo := new(mocks.MockPreferenceRepository)
	deviceRepo := new(mocks.MockDeviceTokenRepository)
	userClient := new(mocks.MockUserClient)
	svc := service.NewNotificationService(inboxRepo, prefRepo, deviceRepo, userClient, zap.NewNop())
	return svc, inboxRepo, prefRepo, deviceRepo, userClient
}

// expectResolve configure le mock user-service pour résoudre firebaseUID → internalUserID.
func expectResolve(userClient *mocks.MockUserClient, firebaseUID, internalUserID string) {
	userClient.On("GetUserIDByFirebaseID", mock.Anything, firebaseUID).Return(internalUserID, nil)
}

// =============================================================================
// GetInbox
// =============================================================================

func TestGetInbox_NormalizesPage(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-abc", "user-abc")
	entries := []*domain.InboxEntry{{InboxID: "inbox-1", Title: "Test"}}
	// page < 1 doit être normalisé à 1
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return(entries, 1, nil)

	result, total, err := svc.GetInbox(ctx, "fb-abc", 0, 20)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
	inboxRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestGetInbox_NormalizesPageSize(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-abc", "user-abc")
	entries := []*domain.InboxEntry{}
	// pageSize > 50 doit être normalisé à 20
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return(entries, 0, nil)

	_, _, err := svc.GetInbox(ctx, "fb-abc", 1, 100)
	require.NoError(t, err)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_NormalizesPageSizeZero(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-abc", "user-abc")
	// pageSize < 1 doit aussi être normalisé à 20
	inboxRepo.On("GetByUserID", mock.Anything, "user-abc", 1, 20).
		Return([]*domain.InboxEntry{}, 0, nil)

	_, _, err := svc.GetInbox(ctx, "fb-abc", 1, 0)
	require.NoError(t, err)
	inboxRepo.AssertExpectations(t)
}

func TestGetInbox_Success(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-xyz", "user-xyz")
	entries := []*domain.InboxEntry{
		{InboxID: "inbox-1", Title: "Notification 1"},
		{InboxID: "inbox-2", Title: "Notification 2"},
	}
	inboxRepo.On("GetByUserID", mock.Anything, "user-xyz", 2, 10).
		Return(entries, 15, nil)

	result, total, err := svc.GetInbox(ctx, "fb-xyz", 2, 10)

	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.Len(t, result, 2)
	inboxRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestGetInbox_RepoError(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-err", "user-err")
	repoErr := errors.New("database error")
	inboxRepo.On("GetByUserID", mock.Anything, "user-err", 1, 20).
		Return(nil, 0, repoErr)

	result, _, err := svc.GetInbox(ctx, "fb-err", 1, 20)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, repoErr)
}

func TestGetInbox_ResolveError(t *testing.T) {
	svc, inboxRepo, _, _, userClient := newService()
	ctx := context.Background()

	resolveErr := errors.New("user not found")
	userClient.On("GetUserIDByFirebaseID", mock.Anything, "fb-unknown").Return("", resolveErr)

	result, _, err := svc.GetInbox(ctx, "fb-unknown", 1, 20)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, resolveErr)
	// La résolution ayant échoué, le repo ne doit jamais être touché.
	inboxRepo.AssertNotCalled(t, "GetByUserID", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// =============================================================================
// UpdatePreferences
// =============================================================================

func TestUpdatePreferences_Success(t *testing.T) {
	svc, _, prefRepo, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-pref", "user-pref")
	prefRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(p *domain.UserNotificationPreference) bool {
		return p.UserID == "user-pref" && p.PushEnabled == false && p.EmailEnabled == true
	})).Return(nil)

	err := svc.UpdatePreferences(ctx, "fb-pref", false, true)
	require.NoError(t, err)
	prefRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestUpdatePreferences_RepoError(t *testing.T) {
	svc, _, prefRepo, _, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-pref", "user-pref")
	repoErr := errors.New("upsert failed")
	prefRepo.On("Upsert", mock.Anything, mock.Anything).Return(repoErr)

	err := svc.UpdatePreferences(ctx, "fb-pref", true, true)
	assert.ErrorIs(t, err, repoErr)
}

// =============================================================================
// RegisterDeviceToken
// =============================================================================

func TestRegisterDeviceToken_Success(t *testing.T) {
	svc, _, _, deviceRepo, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-dev", "user-dev")
	deviceRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(t *domain.UserDeviceToken) bool {
		return t.UserID == "user-dev" && t.FCMToken == "fcm-xyz" && t.Platform == "android"
	})).Return("token-id-001", nil)

	tokenID, err := svc.RegisterDeviceToken(ctx, "fb-dev", "fcm-xyz", "android", "Pixel 8")
	require.NoError(t, err)
	assert.Equal(t, "token-id-001", tokenID)
	deviceRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestRegisterDeviceToken_RepoError(t *testing.T) {
	svc, _, _, deviceRepo, userClient := newService()
	ctx := context.Background()

	expectResolve(userClient, "fb-dev", "user-dev")
	repoErr := errors.New("insert failed")
	deviceRepo.On("Upsert", mock.Anything, mock.Anything).Return("", repoErr)

	tokenID, err := svc.RegisterDeviceToken(ctx, "fb-dev", "fcm-xyz", "android", "Device")
	assert.Empty(t, tokenID)
	assert.ErrorIs(t, err, repoErr)
}

// =============================================================================
// InvalidateDeviceToken (keyé sur fcm_token — pas de résolution)
// =============================================================================

func TestInvalidateDeviceToken_Success(t *testing.T) {
	svc, _, _, deviceRepo, _ := newService()
	ctx := context.Background()

	deviceRepo.On("InvalidateByFCMToken", mock.Anything, "fcm-old").Return(nil)

	err := svc.InvalidateDeviceToken(ctx, "fcm-old")
	require.NoError(t, err)
	deviceRepo.AssertExpectations(t)
}

func TestInvalidateDeviceToken_RepoError(t *testing.T) {
	svc, _, _, deviceRepo, _ := newService()
	ctx := context.Background()

	repoErr := errors.New("update failed")
	deviceRepo.On("InvalidateByFCMToken", mock.Anything, "fcm-old").Return(repoErr)

	err := svc.InvalidateDeviceToken(ctx, "fcm-old")
	assert.ErrorIs(t, err, repoErr)
}
