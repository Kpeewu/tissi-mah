package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	emailGrpc "github.com/Kpeewu/tissi-mah/services/email-service/internal/grpc"
	emailpb "github.com/Kpeewu/tissi-mah/services/email-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/email-service/tests/mocks"
)

// newHandler crée un handler avec un mock provider frais.
func newHandler() (*emailGrpc.EmailHandler, *mocks.MockEmailProvider) {
	mockProvider := new(mocks.MockEmailProvider)
	handler := emailGrpc.NewEmailHandler(mockProvider, zap.NewNop())
	return handler, mockProvider
}

// =============================================================================
// SendEmail
// =============================================================================

func TestSendEmail_Success(t *testing.T) {
	handler, mockProvider := newHandler()
	ctx := context.Background()

	mockProvider.On("Send", mock.Anything, "alice@example.com", "Bienvenue", "Corps texte", "").
		Return("msg-id-001", nil)
	mockProvider.On("Name").Return("sendgrid")

	resp, err := handler.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:       "alice@example.com",
		Subject:  "Bienvenue",
		BodyText: "Corps texte",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "msg-id-001", resp.ProviderMessageId)
	assert.Empty(t, resp.ErrorMessage)
	mockProvider.AssertExpectations(t)
}

func TestSendEmail_ToEmpty(t *testing.T) {
	handler, mockProvider := newHandler()
	ctx := context.Background()

	resp, err := handler.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:      "",
		Subject: "Bienvenue",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.ErrorMessage)
	// Send ne doit pas être appelé
	mockProvider.AssertNotCalled(t, "Send")
}

func TestSendEmail_SubjectEmpty(t *testing.T) {
	handler, mockProvider := newHandler()
	ctx := context.Background()

	resp, err := handler.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:      "bob@example.com",
		Subject: "",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.ErrorMessage)
	mockProvider.AssertNotCalled(t, "Send")
}

func TestSendEmail_ProviderError(t *testing.T) {
	handler, mockProvider := newHandler()
	ctx := context.Background()

	providerErr := errors.New("sendgrid rate limit exceeded")
	mockProvider.On("Send", mock.Anything, "bob@example.com", "Test", mock.Anything, mock.Anything).
		Return("", providerErr)
	mockProvider.On("Name").Return("sendgrid")

	resp, err := handler.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:      "bob@example.com",
		Subject: "Test",
	})

	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, providerErr.Error(), resp.ErrorMessage)
	mockProvider.AssertExpectations(t)
}

// =============================================================================
// Health
// =============================================================================

func TestHealth_ReturnsOK(t *testing.T) {
	handler, _ := newHandler()
	ctx := context.Background()

	resp, err := handler.Health(ctx, &emailpb.HealthRequest{})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "email-service", resp.Service)
	assert.NotEmpty(t, resp.Version)
}
