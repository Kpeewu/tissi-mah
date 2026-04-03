package consumer

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/dispatcher"
)

const (
	streamName    = "notification-events"
	groupName     = "notification-service"
	consumerName  = "consumer-1"
	blockDuration = 5 * time.Second
	batchSize     = 10
)

// Consumer lit les événements du Redis Stream et les dispatch.
type Consumer struct {
	rdb        *redis.Client
	dispatcher *dispatcher.Dispatcher
	logger     *zap.Logger
}

func NewConsumer(rdb *redis.Client, d *dispatcher.Dispatcher, logger *zap.Logger) *Consumer {
	return &Consumer{rdb: rdb, dispatcher: d, logger: logger}
}

// Start lance la boucle de lecture du stream. Bloquant, s'arrête quand ctx est annulé.
func (c *Consumer) Start(ctx context.Context) error {
	// Créer le consumer group s'il n'existe pas
	err := c.rdb.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		c.logger.Error("failed to create consumer group", zap.Error(err))
		return err
	}

	c.logger.Info("consumer started", zap.String("stream", streamName), zap.String("group", groupName))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping")
			return nil
		default:
		}

		streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: consumerName,
			Streams:  []string{streamName, ">"},
			Count:    batchSize,
			Block:    blockDuration,
		}).Result()

		if err != nil {
			if err == redis.Nil || err == context.Canceled {
				continue
			}
			c.logger.Error("xreadgroup error", zap.Error(err))
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				c.processMessage(ctx, msg)
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) {
	data, ok := msg.Values["data"].(string)
	if !ok {
		c.logger.Error("invalid message format", zap.String("id", msg.ID))
		c.ack(ctx, msg.ID)
		return
	}

	err := c.dispatcher.ProcessRaw(ctx, data)
	if err != nil {
		c.logger.Error("dispatch failed",
			zap.String("msg_id", msg.ID),
			zap.Error(err),
		)
		// On ACK quand même pour ne pas bloquer le stream.
		// Les notifications en échec sont marquées "failed" en DB pour le retry worker.
	}

	c.ack(ctx, msg.ID)
}

func (c *Consumer) ack(ctx context.Context, msgID string) {
	if err := c.rdb.XAck(ctx, streamName, groupName, msgID).Err(); err != nil {
		c.logger.Error("xack failed", zap.String("msg_id", msgID), zap.Error(err))
	}
}
