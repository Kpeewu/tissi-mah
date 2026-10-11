package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
)

// newNotifRedis démarre un Redis en mémoire pour capturer les notifications publiées par le service.
func newNotifRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// publishedEvents retourne les événements publiés dans le stream de notifications, dans l'ordre.
func publishedEvents(t *testing.T, rdb *redis.Client) []notification.Event {
	t.Helper()
	msgs, err := rdb.XRange(context.Background(), notification.StreamName, "-", "+").Result()
	require.NoError(t, err)

	events := make([]notification.Event, 0, len(msgs))
	for _, msg := range msgs {
		data, ok := msg.Values["data"].(string)
		require.True(t, ok)
		var event notification.Event
		require.NoError(t, json.Unmarshal([]byte(data), &event))
		events = append(events, event)
	}
	return events
}

// findEvent retourne le premier événement du type donné, nil s'il n'a pas été publié.
func findEvent(events []notification.Event, eventType string) *notification.Event {
	for i := range events {
		if events[i].EventType == eventType {
			return &events[i]
		}
	}
	return nil
}
