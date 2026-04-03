package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

const (
	retryInterval = 1 * time.Minute
	retryBatch    = 50
)

// RetryWorker relance les notifications en échec.
type RetryWorker struct {
	notifRepo   repoInterfaces.NotificationRepository
	pushClient  client.PushClient
	emailClient client.EmailClient
	logger      *zap.Logger
}

func NewRetryWorker(
	notifRepo repoInterfaces.NotificationRepository,
	pushClient client.PushClient,
	emailClient client.EmailClient,
	logger *zap.Logger,
) *RetryWorker {
	return &RetryWorker{
		notifRepo:   notifRepo,
		pushClient:  pushClient,
		emailClient: emailClient,
		logger:      logger,
	}
}

// Start lance la boucle de retry. Bloquant, s'arrête quand ctx est annulé.
func (w *RetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	w.logger.Info("retry worker started", zap.Duration("interval", retryInterval))

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("retry worker stopping")
			return
		case <-ticker.C:
			w.processRetries(ctx)
		}
	}
}

func (w *RetryWorker) processRetries(ctx context.Context) {
	pending, err := w.notifRepo.GetPendingForRetry(ctx, retryBatch)
	if err != nil {
		w.logger.Error("failed to get pending retries", zap.Error(err))
		return
	}

	if len(pending) == 0 {
		return
	}

	w.logger.Info("processing retries", zap.Int("count", len(pending)))

	for _, n := range pending {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.retryNotification(ctx, n)
	}
}

func (w *RetryWorker) retryNotification(ctx context.Context, n *domain.Notification) {
	var success bool
	var errMsg string

	switch n.Channel {
	case "push":
		ok, errCode, err := w.pushClient.SendPush(ctx, n.ResolvedTitle, n.ResolvedBody, n.RecipientAddress, nil)
		success = ok && err == nil
		if errCode != "" {
			errMsg = errCode
		}
		if err != nil {
			errMsg = err.Error()
		}

	case "email":
		ok, errCode, err := w.emailClient.SendEmail(ctx, n.RecipientAddress, n.ResolvedSubject, n.ResolvedBody, "")
		success = ok && err == nil
		if errCode != "" {
			errMsg = errCode
		}
		if err != nil {
			errMsg = err.Error()
		}

	default:
		w.logger.Warn("unknown channel for retry", zap.String("channel", n.Channel))
		return
	}

	n.AttemptCount++

	if success {
		n.Status = "sent"
		n.FailureReason = ""
	} else if n.AttemptCount >= n.MaxAttempts {
		n.Status = "cancelled"
		n.FailureReason = errMsg
	} else {
		n.Status = "failed"
		n.FailureReason = errMsg
		next := time.Now().Add(backoffDuration(n.AttemptCount))
		n.NextAttemptAt = &next
	}

	if err := w.notifRepo.UpdateRetry(ctx, n); err != nil {
		w.logger.Error("failed to update retry", zap.String("id", n.NotificationID), zap.Error(err))
	}
}

// backoffDuration retourne un délai exponentiel : 30s, 2min, 8min, 30min...
func backoffDuration(attempt int16) time.Duration {
	base := 30 * time.Second
	for i := int16(1); i < attempt; i++ {
		base *= 4
		if base > 30*time.Minute {
			return 30 * time.Minute
		}
	}
	return base
}
