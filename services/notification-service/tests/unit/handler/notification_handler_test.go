package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifGrpc "github.com/Kpeewu/tissi-mah/services/notification-service/internal/grpc"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/middleware"
	notifpb "github.com/Kpeewu/tissi-mah/services/notification-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/notification-service/tests/mocks"
)

// ctxWithUID retourne un contexte contenant le FirebaseIDKey.
func ctxWithUID(uid string) context.Context {
	return context.WithValue(context.Background(), middleware.FirebaseIDKey, uid)
}

// assertGRPCCode vérifie que l'erreur est un code gRPC attendu.
func assertGRPCCode(t *testing.T, err error, expected codes.Code) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, expected, st.Code())
}

// newHandler crée un handler avec un mock service frais.
func newHandler() (*notifGrpc.NotificationHandler, *mocks.MockNotificationService) {
	mockSvc := new(mocks.MockNotificationService)
	h := notifGrpc.NewNotificationHandler(mockSvc, zap.NewNop())
	return h, mockSvc
}

// =============================================================================
// GetInbox
// =============================================================================

func TestGetInbox_NoUID_Unauthenticated(t *testing.T) {
	h, _ := newHandler()
	ctx := context.Background() // pas de firebase_uid

	_, err := h.GetInbox(ctx, &notifpb.GetInboxRequest{})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestGetInbox_Success_MapsEntries(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-abc")

	now := time.Now().UTC()
	entries := []*domain.InboxEntry{
		{
			InboxID:    "inbox-1",
			EventType:  "BOOKING_CONFIRMED",
			Title:      "Réservation confirmée",
			Body:       "Votre réservation est confirmée",
			ActionType: "booking_detail",
			ActionID:   "booking-999",
			IsRead:     false,
			CreatedAt:  now,
		},
	}
	mockSvc.On("GetInbox", mock.Anything, "uid-abc", 1, 20).Return(entries, 1, nil)

	resp, err := h.GetInbox(ctx, &notifpb.GetInboxRequest{Page: 1, PageSize: 20})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.TotalCount)
	require.Len(t, resp.Entries, 1)
	assert.Equal(t, "inbox-1", resp.Entries[0].InboxId)
	assert.Equal(t, "BOOKING_CONFIRMED", resp.Entries[0].EventType)
	assert.Equal(t, "Réservation confirmée", resp.Entries[0].Title)
	assert.False(t, resp.Entries[0].IsRead)
}

func TestGetInbox_ServiceError_Internal(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-abc")

	mockSvc.On("GetInbox", mock.Anything, "uid-abc", mock.Anything, mock.Anything).
		Return(nil, 0, errors.New("db error"))

	_, err := h.GetInbox(ctx, &notifpb.GetInboxRequest{})
	assertGRPCCode(t, err, codes.Internal)
}

// =============================================================================
// MarkAsRead
// =============================================================================

func TestMarkAsRead_Success(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-xyz")

	mockSvc.On("MarkAsRead", mock.Anything, "inbox-abc", "uid-xyz").Return(nil)

	resp, err := h.MarkAsRead(ctx, &notifpb.MarkAsReadRequest{InboxId: "inbox-abc"})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestMarkAsRead_NoUID_Unauthenticated(t *testing.T) {
	h, _ := newHandler()
	ctx := context.Background()

	_, err := h.MarkAsRead(ctx, &notifpb.MarkAsReadRequest{InboxId: "inbox-abc"})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestMarkAsRead_ServiceError_Internal(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-xyz")

	mockSvc.On("MarkAsRead", mock.Anything, "inbox-abc", "uid-xyz").Return(errors.New("not found"))

	_, err := h.MarkAsRead(ctx, &notifpb.MarkAsReadRequest{InboxId: "inbox-abc"})
	assertGRPCCode(t, err, codes.Internal)
}

// =============================================================================
// GetPreferences
// =============================================================================

func TestGetPreferences_Success(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-pref")

	prefs := &domain.UserNotificationPreference{
		UserID:       "uid-pref",
		PushEnabled:  true,
		EmailEnabled: false,
	}
	mockSvc.On("GetPreferences", mock.Anything, "uid-pref").Return(prefs, nil)

	resp, err := h.GetPreferences(ctx, &notifpb.GetPreferencesRequest{})

	require.NoError(t, err)
	assert.True(t, resp.PushEnabled)
	assert.False(t, resp.EmailEnabled)
}

func TestGetPreferences_NoUID_Unauthenticated(t *testing.T) {
	h, _ := newHandler()
	ctx := context.Background()

	_, err := h.GetPreferences(ctx, &notifpb.GetPreferencesRequest{})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

// =============================================================================
// UpdatePreferences
// =============================================================================

func TestUpdatePreferences_Success(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-upref")

	mockSvc.On("UpdatePreferences", mock.Anything, "uid-upref", false, true).Return(nil)

	resp, err := h.UpdatePreferences(ctx, &notifpb.UpdatePreferencesRequest{PushEnabled: false, EmailEnabled: true})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// =============================================================================
// RegisterDeviceToken
// =============================================================================

func TestRegisterDeviceToken_Success(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := ctxWithUID("uid-reg")

	mockSvc.On("RegisterDeviceToken", mock.Anything, "uid-reg", "fcm-token-001", "android", "Pixel 8").
		Return("token-id-001", nil)

	resp, err := h.RegisterDeviceToken(ctx, &notifpb.RegisterDeviceTokenRequest{
		FcmToken:   "fcm-token-001",
		Platform:   "android",
		DeviceName: "Pixel 8",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "token-id-001", resp.TokenId)
}

func TestRegisterDeviceToken_NoUID_Unauthenticated(t *testing.T) {
	h, _ := newHandler()
	ctx := context.Background()

	_, err := h.RegisterDeviceToken(ctx, &notifpb.RegisterDeviceTokenRequest{FcmToken: "fcm-001"})
	assertGRPCCode(t, err, codes.Unauthenticated)
}

func TestRegisterDeviceToken_EmptyToken_InvalidArgument(t *testing.T) {
	h, _ := newHandler()
	ctx := ctxWithUID("uid-reg")

	_, err := h.RegisterDeviceToken(ctx, &notifpb.RegisterDeviceTokenRequest{FcmToken: ""})
	assertGRPCCode(t, err, codes.InvalidArgument)
}

// =============================================================================
// InvalidateDeviceToken (inter-service — pas de firebase_uid requis)
// =============================================================================

func TestInvalidateDeviceToken_Success_NoAuthRequired(t *testing.T) {
	h, mockSvc := newHandler()
	ctx := context.Background() // pas de UID

	mockSvc.On("InvalidateDeviceToken", mock.Anything, "fcm-old-001").Return(nil)

	resp, err := h.InvalidateDeviceToken(ctx, &notifpb.InvalidateDeviceTokenRequest{FcmToken: "fcm-old-001"})

	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestInvalidateDeviceToken_EmptyToken_InvalidArgument(t *testing.T) {
	h, _ := newHandler()
	ctx := context.Background()

	_, err := h.InvalidateDeviceToken(ctx, &notifpb.InvalidateDeviceTokenRequest{FcmToken: ""})
	assertGRPCCode(t, err, codes.InvalidArgument)
}
