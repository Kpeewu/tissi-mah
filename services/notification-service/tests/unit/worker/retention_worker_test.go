package worker_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/worker"
	"github.com/Kpeewu/tissi-mah/services/notification-service/tests/mocks"
)

// newRetentionWorker crée un RetentionWorker avec les mocks injectés.
func newRetentionWorker() (*worker.RetentionWorker, *mocks.MockNotificationRepository, *mocks.MockInboxRepository) {
	notifRepo := new(mocks.MockNotificationRepository)
	inboxRepo := new(mocks.MockInboxRepository)
	w := worker.NewRetentionWorker(notifRepo, inboxRepo, zap.NewNop())
	return w, notifRepo, inboxRepo
}

// =============================================================================
// Purge — appelle notifRepo.PurgeOldSent(14) et inboxRepo.PurgeOldRead(90)
// =============================================================================

func TestRetentionWorker_Purge_CallsBothRepos(t *testing.T) {
	w, notifRepo, inboxRepo := newRetentionWorker()
	ctx := context.Background()

	notifRepo.On("PurgeOldSent", mock.Anything, 14).Return(int64(5), nil)
	inboxRepo.On("PurgeOldRead", mock.Anything, 90).Return(int64(12), nil)

	w.Purge(ctx)

	notifRepo.AssertCalled(t, "PurgeOldSent", mock.Anything, 14)
	inboxRepo.AssertCalled(t, "PurgeOldRead", mock.Anything, 90)
}

// =============================================================================
// Purge — erreur notifRepo ignorée (log seulement, pas de panique)
// =============================================================================

func TestRetentionWorker_PurgeNotifError_DoesNotPanic(t *testing.T) {
	w, notifRepo, inboxRepo := newRetentionWorker()
	ctx := context.Background()

	notifRepo.On("PurgeOldSent", mock.Anything, 14).Return(int64(0), errors.New("purge notif failed"))
	inboxRepo.On("PurgeOldRead", mock.Anything, 90).Return(int64(0), nil)

	// Doit terminer sans paniquer
	w.Purge(ctx)

	inboxRepo.AssertCalled(t, "PurgeOldRead", mock.Anything, 90)
}

// =============================================================================
// Purge — erreur inboxRepo ignorée (log seulement, pas de panique)
// =============================================================================

func TestRetentionWorker_PurgeInboxError_DoesNotPanic(t *testing.T) {
	w, notifRepo, inboxRepo := newRetentionWorker()
	ctx := context.Background()

	notifRepo.On("PurgeOldSent", mock.Anything, 14).Return(int64(0), nil)
	inboxRepo.On("PurgeOldRead", mock.Anything, 90).Return(int64(0), errors.New("purge inbox failed"))

	// Doit terminer sans paniquer
	w.Purge(ctx)

	notifRepo.AssertExpectations(t)
}
