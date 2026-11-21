package notification_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kpeewu/tissi-mah/pkg/notification"
)

// newTestRedis démarre un serveur Redis in-memory et retourne le client.
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return s, rdb
}

// TestPublish_Success vérifie qu'un événement valide est écrit dans le stream.
func TestPublish_Success(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	event := notification.Event{
		EventID:   "evt-001",
		EventType: notification.BookingConfirmed,
		UserID:    "user-123",
	}

	err := notification.Publish(ctx, rdb, event)
	require.NoError(t, err)

	msgs, err := rdb.XRange(ctx, notification.StreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Values, "data")
}

// TestPublish_AutoGeneratesEventID vérifie qu'un UUID est injecté si EventID est vide.
func TestPublish_AutoGeneratesEventID(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	event := notification.Event{
		EventID:   "", // vide → doit être généré
		EventType: notification.BookingConfirmed,
		UserID:    "user-456",
	}

	err := notification.Publish(ctx, rdb, event)
	require.NoError(t, err)

	msgs, err := rdb.XRange(ctx, notification.StreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	// Vérifier que l'EventID a été injecté dans le JSON sérialisé
	raw, ok := msgs[0].Values["data"].(string)
	require.True(t, ok, "la valeur 'data' doit être une chaîne")
	var decoded notification.Event
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	assert.NotEmpty(t, decoded.EventID, "EventID doit être généré automatiquement")
}

// TestPublish_SerializationJSON vérifie que le payload est sérialisé correctement.
func TestPublish_SerializationJSON(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	event := notification.Event{
		EventID:       "evt-002",
		EventType:     notification.BookingRejected,
		UserID:        "user-789",
		ReferenceID:   "booking-abc",
		ReferenceType: notification.RefBooking,
		Payload:       map[string]string{"driver_name": "Kofi Mensah"},
	}

	require.NoError(t, notification.Publish(ctx, rdb, event))

	msgs, err := rdb.XRange(ctx, notification.StreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	raw, ok := msgs[0].Values["data"].(string)
	require.True(t, ok)

	var decoded notification.Event
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))

	assert.Equal(t, event.EventID, decoded.EventID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.UserID, decoded.UserID)
	assert.Equal(t, event.ReferenceID, decoded.ReferenceID)
	assert.Equal(t, event.ReferenceType, decoded.ReferenceType)
	assert.Equal(t, event.Payload["driver_name"], decoded.Payload["driver_name"])
}

// TestPublish_MultipleEvents vérifie que plusieurs événements sont bien ajoutés au stream.
func TestPublish_MultipleEvents(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		event := notification.Event{
			EventID:   "evt-multi-" + string(rune('A'+i)),
			EventType: notification.TripStarted,
			UserID:    "user-multi",
		}
		require.NoError(t, notification.Publish(ctx, rdb, event))
	}

	msgs, err := rdb.XRange(ctx, notification.StreamName, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

// TestPublish_ErrorWhenRedisFails vérifie que l'erreur Redis est propagée.
func TestPublish_ErrorWhenRedisFails(t *testing.T) {
	s, rdb := newTestRedis(t)
	ctx := context.Background()

	// Couper le serveur miniredis pour simuler une panne
	s.Close()

	event := notification.Event{
		EventID:   "evt-fail",
		EventType: notification.Welcome,
		UserID:    "user-000",
	}

	err := notification.Publish(ctx, rdb, event)
	assert.Error(t, err)
}
