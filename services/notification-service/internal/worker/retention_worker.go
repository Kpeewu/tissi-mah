package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	repoInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

const (
	// retentionInterval : exécution toutes les 24h (configuré pour tourner quotidiennement)
	retentionInterval = 24 * time.Hour
	// notificationRetentionDays : 14 jours pour les notifications envoyées
	notificationRetentionDays = 14
	// inboxRetentionDays : 90 jours pour les entrées inbox lues
	inboxRetentionDays = 90
)

// RetentionWorker purge les données anciennes (notifications et inbox).
type RetentionWorker struct {
	notifRepo repoInterfaces.NotificationRepository
	inboxRepo repoInterfaces.InboxRepository
	logger    *zap.Logger
}

func NewRetentionWorker(
	notifRepo repoInterfaces.NotificationRepository,
	inboxRepo repoInterfaces.InboxRepository,
	logger *zap.Logger,
) *RetentionWorker {
	return &RetentionWorker{
		notifRepo: notifRepo,
		inboxRepo: inboxRepo,
		logger:    logger,
	}
}

// Start lance la boucle de rétention. Bloquant, s'arrête quand ctx est annulé.
func (w *RetentionWorker) Start(ctx context.Context) {
	// Exécuter une première fois au démarrage
	w.Purge(ctx)

	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	w.logger.Info("retention worker started", zap.Duration("interval", retentionInterval))

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("retention worker stopping")
			return
		case <-ticker.C:
			w.Purge(ctx)
		}
	}
}

// Purge exécute un cycle de rétention (exporté pour les tests).
func (w *RetentionWorker) Purge(ctx context.Context) {
	// Purge des notifications (envoyées/annulées de +14j)
	deletedNotifs, err := w.notifRepo.PurgeOldSent(ctx, notificationRetentionDays)
	if err != nil {
		w.logger.Error("failed to purge notifications", zap.Error(err))
	} else if deletedNotifs > 0 {
		w.logger.Info("purged old notifications", zap.Int64("count", deletedNotifs))
	}

	// Purge des inbox lues de +90j
	deletedInbox, err := w.inboxRepo.PurgeOldRead(ctx, inboxRetentionDays)
	if err != nil {
		w.logger.Error("failed to purge inbox", zap.Error(err))
	} else if deletedInbox > 0 {
		w.logger.Info("purged old inbox entries", zap.Int64("count", deletedInbox))
	}
}
