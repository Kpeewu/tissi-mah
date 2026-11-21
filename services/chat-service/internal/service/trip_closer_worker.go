package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	svcInterfaces "github.com/Kpeewu/tissi-mah/services/chat-service/internal/service/interfaces"
)

// tripCompletedEvent est le payload des events trip.completed sur Redis Streams.
type tripCompletedEvent struct {
	TripID string `json:"trip_id"`
}

// StartTripCloserWorker subscribe au stream Redis trip.completed et ferme les
// threads de conversation dès qu'un trajet se termine.
// Les messages antérieurs restent accessibles en lecture mais plus aucun envoi.
func StartTripCloserWorker(ctx context.Context, svc svcInterfaces.ChatService, redisClient *redis.Client, intervalSeconds int, logger *zap.Logger) {
	const (
		streamKey = "trip.completed"
		groupName = "chat-service"
		consumer  = "chat-trip-closer"
	)

	if intervalSeconds <= 0 {
		intervalSeconds = 30
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	logger.Info("trip closer worker started",
		zap.String("stream", streamKey),
		zap.Int("intervalSeconds", intervalSeconds),
	)

	// Créer le consumer group si absent (idempotent via MKSTREAM).
	_ = redisClient.XGroupCreateMkStream(ctx, streamKey, groupName, "$").Err()

	for {
		select {
		case <-ctx.Done():
			logger.Info("trip closer worker stopped")
			return
		case <-ticker.C:
			processTripEvents(ctx, svc, redisClient, streamKey, groupName, consumer, logger)
		}
	}
}

func processTripEvents(
	ctx context.Context,
	svc svcInterfaces.ChatService,
	rc *redis.Client,
	streamKey, groupName, consumer string,
	logger *zap.Logger,
) {
	msgs, err := rc.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumer,
		Streams:  []string{streamKey, ">"},
		Count:    50,
		Block:    0,
		NoAck:    false,
	}).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Error("trip closer: XReadGroup failed", zap.Error(err))
		}
		return
	}

	for _, stream := range msgs {
		for _, msg := range stream.Messages {
			payload, ok := msg.Values["payload"].(string)
			if !ok {
				_ = rc.XAck(ctx, streamKey, groupName, msg.ID)
				continue
			}

			var event tripCompletedEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				logger.Warn("trip closer: unmarshal event", zap.Error(err), zap.String("msgID", msg.ID))
				_ = rc.XAck(ctx, streamKey, groupName, msg.ID)
				continue
			}

			if event.TripID != "" {
				n, closeErr := svc.CloseThreadsByTripID(ctx, event.TripID)
				if closeErr != nil {
					logger.Error("trip closer: close threads", zap.String("tripID", event.TripID), zap.Error(closeErr))
					continue // retry on next tick
				}
				logger.Info("trip closer: threads closed", zap.String("tripID", event.TripID), zap.Int("count", n))
			}

			_ = rc.XAck(ctx, streamKey, groupName, msg.ID)
		}
	}
}
