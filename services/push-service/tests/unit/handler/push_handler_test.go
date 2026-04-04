package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/push-service/internal/fcm"
	pushGrpc "github.com/Kpeewu/tissi-mah/services/push-service/internal/grpc"
	pushpb "github.com/Kpeewu/tissi-mah/services/push-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/push-service/tests/mocks"
)

// newHandler crée un handler avec un mock FCMClient frais.
func newHandler() (*pushGrpc.PushHandler, *mocks.MockFCMClient) {
	mockFCM := new(mocks.MockFCMClient)
	handler := pushGrpc.NewPushHandler(mockFCM, zap.NewNop())
	return handler, mockFCM
}

// =============================================================================
// SendPush
// =============================================================================

func TestSendPush_Success(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	mockFCM.On("Send", mock.Anything, "Nouveau trajet", "Votre trajet commence", "fcm-token-abc", mock.Anything).
		Return(nil)

	resp, err := handler.SendPush(ctx, &pushpb.SendPushRequest{
		FcmToken: "fcm-token-abc",
		Title:    "Nouveau trajet",
		Body:     "Votre trajet commence",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.ErrorCode)
	mockFCM.AssertExpectations(t)
}

func TestSendPush_FcmTokenEmpty(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	resp, err := handler.SendPush(ctx, &pushpb.SendPushRequest{
		FcmToken: "",
		Title:    "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "INVALID_INPUT", resp.ErrorCode)
	mockFCM.AssertNotCalled(t, "Send")
}

func TestSendPush_TitleEmpty(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	resp, err := handler.SendPush(ctx, &pushpb.SendPushRequest{
		FcmToken: "fcm-token-xyz",
		Title:    "",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "INVALID_INPUT", resp.ErrorCode)
	mockFCM.AssertNotCalled(t, "Send")
}

func TestSendPush_FCMError(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	fcmErr := errors.New("token invalide")
	mockFCM.On("Send", mock.Anything, mock.Anything, mock.Anything, "bad-token", mock.Anything).
		Return(fcmErr)

	resp, err := handler.SendPush(ctx, &pushpb.SendPushRequest{
		FcmToken: "bad-token",
		Title:    "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "FCM_FAILED", resp.ErrorCode)
	assert.Equal(t, fcmErr.Error(), resp.ErrorMessage)
	mockFCM.AssertExpectations(t)
}

// =============================================================================
// SendPushMulticast
// =============================================================================

func TestSendPushMulticast_Success(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	tokens := []string{"token-1", "token-2", "token-3"}
	results := []fcm.SendResult{
		{Token: "token-1", Success: true},
		{Token: "token-2", Success: true},
		{Token: "token-3", Success: false, ErrorCode: "UNREGISTERED"},
	}

	mockFCM.On("SendMulticast", mock.Anything, "Alerte trajet", "Départ dans 5 min", tokens, mock.Anything).
		Return(results, nil)

	resp, err := handler.SendPushMulticast(ctx, &pushpb.SendPushMulticastRequest{
		FcmTokens: tokens,
		Title:     "Alerte trajet",
		Body:      "Départ dans 5 min",
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.SuccessCount)
	assert.Equal(t, int32(1), resp.FailureCount)
	require.Len(t, resp.Results, 3)
	assert.True(t, resp.Results[0].Success)
	assert.False(t, resp.Results[2].Success)
	assert.Equal(t, "UNREGISTERED", resp.Results[2].ErrorCode)
	mockFCM.AssertExpectations(t)
}

func TestSendPushMulticast_EmptyTokens(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	resp, err := handler.SendPushMulticast(ctx, &pushpb.SendPushMulticastRequest{
		FcmTokens: []string{},
		Title:     "Test",
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.SuccessCount)
	assert.Equal(t, int32(0), resp.FailureCount)
	mockFCM.AssertNotCalled(t, "SendMulticast")
}

func TestSendPushMulticast_TitleEmpty(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	resp, err := handler.SendPushMulticast(ctx, &pushpb.SendPushMulticastRequest{
		FcmTokens: []string{"token-a"},
		Title:     "",
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.SuccessCount)
	assert.Equal(t, int32(0), resp.FailureCount)
	mockFCM.AssertNotCalled(t, "SendMulticast")
}

func TestSendPushMulticast_FCMError(t *testing.T) {
	handler, mockFCM := newHandler()
	ctx := context.Background()

	tokens := []string{"token-x", "token-y"}
	fcmErr := errors.New("firebase indisponible")
	mockFCM.On("SendMulticast", mock.Anything, mock.Anything, mock.Anything, tokens, mock.Anything).
		Return(nil, fcmErr)

	resp, err := handler.SendPushMulticast(ctx, &pushpb.SendPushMulticastRequest{
		FcmTokens: tokens,
		Title:     "Test",
	})

	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.SuccessCount)
	assert.Equal(t, int32(2), resp.FailureCount)
	assert.Empty(t, resp.Results)
	mockFCM.AssertExpectations(t)
}

// =============================================================================
// Health
// =============================================================================

func TestHealth_ReturnsOK(t *testing.T) {
	handler, _ := newHandler()
	ctx := context.Background()

	resp, err := handler.Health(ctx, &pushpb.HealthRequest{})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "push-service", resp.Service)
	assert.NotEmpty(t, resp.Version)
}
