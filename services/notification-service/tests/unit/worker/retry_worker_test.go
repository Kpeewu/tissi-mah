package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/worker"
	"github.com/Kpeewu/tissi-mah/services/notification-service/tests/mocks"
)

// newRetryWorker crée un RetryWorker avec les mocks injectés.
func newRetryWorker() (*worker.RetryWorker, *mocks.MockNotificationRepository, *mocks.MockPushClient, *mocks.MockEmailClient) {
	notifRepo := new(mocks.MockNotificationRepository)
	pushClient := new(mocks.MockPushClient)
	emailClient := new(mocks.MockEmailClient)
	w := worker.NewRetryWorker(notifRepo, pushClient, emailClient, zap.NewNop())
	return w, notifRepo, pushClient, emailClient
}

// pendingPushNotif retourne une notification push en attente de retry.
func pendingPushNotif(id string, attempts, maxAttempts int16) *domain.Notification {
	return &domain.Notification{
		NotificationID:  id,
		EventID:         "evt-" + id,
		UserID:          "user-123",
		EventType:       "BOOKING_CONFIRMED",
		Channel:         "push",
		ResolvedTitle:   "Titre test",
		ResolvedBody:    "Corps test",
		RecipientAddress: "fcm-token-test",
		Status:          "failed",
		AttemptCount:    attempts,
		MaxAttempts:     maxAttempts,
	}
}

// pendingEmailNotif retourne une notification email en attente de retry.
func pendingEmailNotif(id string, attempts, maxAttempts int16) *domain.Notification {
	return &domain.Notification{
		NotificationID:  id,
		Channel:         "email",
		ResolvedSubject: "Sujet test",
		ResolvedBody:    "Corps test",
		RecipientAddress: "user@example.com",
		Status:          "failed",
		AttemptCount:    attempts,
		MaxAttempts:     maxAttempts,
	}
}

// =============================================================================
// ProcessRetries — notification push failed → Send appelé
// =============================================================================

func TestRetryWorker_PushNotification_Retried(t *testing.T) {
	w, notifRepo, pushClient, _ := newRetryWorker()
	ctx := context.Background()

	notif := pendingPushNotif("notif-push-001", 1, 3)
	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{notif}, nil)
	pushClient.On("SendPush", mock.Anything, "Titre test", "Corps test", "fcm-token-test", mock.Anything).
		Return(true, "", nil)
	notifRepo.On("UpdateRetry", mock.Anything, mock.MatchedBy(func(n *domain.Notification) bool {
		return n.Status == "sent" && n.AttemptCount == 2
	})).Return(nil)

	w.ProcessRetries(ctx)

	notifRepo.AssertExpectations(t)
	pushClient.AssertExpectations(t)
}

// =============================================================================
// ProcessRetries — notification email failed → SendEmail appelé
// =============================================================================

func TestRetryWorker_EmailNotification_Retried(t *testing.T) {
	w, notifRepo, _, emailClient := newRetryWorker()
	ctx := context.Background()

	notif := pendingEmailNotif("notif-email-001", 1, 3)
	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{notif}, nil)
	emailClient.On("SendEmail", mock.Anything, "user@example.com", "Sujet test", "Corps test", "").
		Return(true, "", nil)
	notifRepo.On("UpdateRetry", mock.Anything, mock.MatchedBy(func(n *domain.Notification) bool {
		return n.Status == "sent"
	})).Return(nil)

	w.ProcessRetries(ctx)

	notifRepo.AssertExpectations(t)
	emailClient.AssertExpectations(t)
}

// =============================================================================
// ProcessRetries — succès → status = "sent"
// =============================================================================

func TestRetryWorker_Success_StatusSent(t *testing.T) {
	w, notifRepo, pushClient, _ := newRetryWorker()
	ctx := context.Background()

	notif := pendingPushNotif("notif-success", 1, 3)
	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{notif}, nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, "", nil)

	var capturedNotif *domain.Notification
	notifRepo.On("UpdateRetry", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedNotif = args.Get(1).(*domain.Notification)
	}).Return(nil)

	w.ProcessRetries(ctx)

	require.NotNil(t, capturedNotif)
	assert.Equal(t, "sent", capturedNotif.Status)
	assert.Equal(t, int16(2), capturedNotif.AttemptCount)
	assert.Empty(t, capturedNotif.FailureReason)
}

// =============================================================================
// ProcessRetries — échec avec attempts restants → status = "failed"
// =============================================================================

func TestRetryWorker_Failure_WithRemainingAttempts_StatusFailed(t *testing.T) {
	w, notifRepo, pushClient, _ := newRetryWorker()
	ctx := context.Background()

	notif := pendingPushNotif("notif-fail-retry", 1, 3) // 1 attempt, max 3 → still retryable
	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{notif}, nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, "UNREGISTERED", nil)

	var capturedNotif *domain.Notification
	notifRepo.On("UpdateRetry", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedNotif = args.Get(1).(*domain.Notification)
	}).Return(nil)

	w.ProcessRetries(ctx)

	require.NotNil(t, capturedNotif)
	assert.Equal(t, "failed", capturedNotif.Status)
	assert.Equal(t, int16(2), capturedNotif.AttemptCount)
	assert.NotNil(t, capturedNotif.NextAttemptAt)
}

// =============================================================================
// ProcessRetries — attempts épuisés → status = "cancelled"
// =============================================================================

func TestRetryWorker_Failure_AttemptsExhausted_StatusCancelled(t *testing.T) {
	w, notifRepo, pushClient, _ := newRetryWorker()
	ctx := context.Background()

	// attempt_count = max_attempts - 1 → après increment = max_attempts → cancelled
	notif := pendingPushNotif("notif-cancelled", 2, 3)
	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{notif}, nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(false, "FCM_ERROR", nil)

	var capturedNotif *domain.Notification
	notifRepo.On("UpdateRetry", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedNotif = args.Get(1).(*domain.Notification)
	}).Return(nil)

	w.ProcessRetries(ctx)

	require.NotNil(t, capturedNotif)
	assert.Equal(t, "cancelled", capturedNotif.Status)
	assert.Equal(t, int16(3), capturedNotif.AttemptCount)
}

// =============================================================================
// ProcessRetries — liste vide → rien n'est fait
// =============================================================================

func TestRetryWorker_NoPending_NoAction(t *testing.T) {
	w, notifRepo, pushClient, emailClient := newRetryWorker()
	ctx := context.Background()

	notifRepo.On("GetPendingForRetry", mock.Anything, 50).Return([]*domain.Notification{}, nil)

	w.ProcessRetries(ctx)

	pushClient.AssertNotCalled(t, "SendPush")
	emailClient.AssertNotCalled(t, "SendEmail")
}

// =============================================================================
// backoffDuration — vérification des valeurs
// =============================================================================

func TestBackoffDuration_Values(t *testing.T) {
	cases := []struct {
		attempt  int16
		expected time.Duration
	}{
		{1, 30 * time.Second},
		{2, 2 * time.Minute},
		{3, 8 * time.Minute},
		{4, 30 * time.Minute},  // cap à 30min
		{10, 30 * time.Minute}, // cap à 30min
	}
	for _, tc := range cases {
		got := worker.BackoffDuration(tc.attempt)
		assert.Equal(t, tc.expected, got, "attempt=%d", tc.attempt)
	}
}
