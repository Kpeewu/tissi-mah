package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	repoInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/repository/interfaces"
)

// StartRetentionWorker purge les messages chat non signalés plus vieux que
// retentionDays jours. Tourne une fois par jour à intervalle fixe.
func StartRetentionWorker(ctx context.Context, messageRepo repoInterfaces.ChatMessageRepository, retentionDays int, logger *zap.Logger) {
	if retentionDays <= 0 {
		retentionDays = 180
	}
	// Vérification toutes les 24h
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	logger.Info("retention worker started", zap.Int("retentionDays", retentionDays))

	for {
		select {
		case <-ctx.Done():
			logger.Info("retention worker stopped")
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
			n, err := messageRepo.DeleteOlderThan(ctx, cutoff)
			if err != nil {
				logger.Error("retention: delete old messages failed", zap.Error(err))
				continue
			}
			if n > 0 {
				logger.Info("retention: messages purged",
					zap.Int64("count", n),
					zap.Time("cutoff", cutoff),
				)
			}
		}
	}
}
