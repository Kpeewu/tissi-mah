package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Kpeewu/tissi-mah/services/booking-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TTL du cache pour les détails d'une réservation
	bookingDetailTTL = 3 * time.Minute

	// TTL du cache pour la liste paginée des réservations d'un passager
	passengerBookingsTTL = 2 * time.Minute

	// TTL du cache pour la liste paginée des réservations d'un trajet (conducteur)
	driverTripBookingsTTL = 2 * time.Minute

	// TTL du compteur atomique de places disponibles
	seatCounterTTL = 24 * time.Hour

	// Préfixe des clés Redis pour le booking-service
	keyPrefix = "booking:"

	// Préfixe des clés Redis pour le compteur de places
	seatPrefix = "trip:"
)

// BookingCache gère le cache Redis pour le booking-service.
type BookingCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewBookingCache crée une instance du cache booking.
func NewBookingCache(client *redis.Client, logger *zap.Logger) *BookingCache {
	return &BookingCache{client: client, logger: logger.Named("cache")}
}

// =============================================================================
// Booking Details cache
// =============================================================================

// GetBookingDetails récupère les détails d'une réservation depuis le cache.
// Retourne nil, nil si la clé n'existe pas (cache miss).
func (c *BookingCache) GetBookingDetails(ctx context.Context, bookingID string) (*domain.Booking, error) {
	key := c.bookingDetailKey(bookingID)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			c.logger.Debug("cache miss: booking details", zap.String("bookingID", bookingID))
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var booking domain.Booking
	if err := json.Unmarshal(data, &booking); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	c.logger.Debug("cache hit: booking details", zap.String("bookingID", bookingID))
	return &booking, nil
}

// SetBookingDetails stocke les détails d'une réservation dans le cache.
func (c *BookingCache) SetBookingDetails(ctx context.Context, bookingID string, booking *domain.Booking) error {
	key := c.bookingDetailKey(bookingID)

	data, err := json.Marshal(booking)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, bookingDetailTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	c.logger.Debug("cache set: booking details", zap.String("bookingID", bookingID))
	return nil
}

// InvalidateBooking supprime le cache d'une réservation.
func (c *BookingCache) InvalidateBooking(ctx context.Context, bookingID string) {
	key := c.bookingDetailKey(bookingID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("bookingID", bookingID))
	}
}

// =============================================================================
// Passenger Bookings cache (liste paginée)
// =============================================================================

// GetPassengerBookings récupère la liste paginée des réservations d'un passager depuis le cache.
func (c *BookingCache) GetPassengerBookings(ctx context.Context, passengerID string, pageIndex int, statusFilter string) ([]*domain.BookingPreview, error) {
	key := c.passengerBookingsKey(passengerID, pageIndex, statusFilter)

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var previews []*domain.BookingPreview
	if err := json.Unmarshal(data, &previews); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err), zap.String("key", key))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	return previews, nil
}

// SetPassengerBookings stocke la liste paginée des réservations d'un passager dans le cache.
func (c *BookingCache) SetPassengerBookings(ctx context.Context, passengerID string, pageIndex int, statusFilter string, previews []*domain.BookingPreview) error {
	key := c.passengerBookingsKey(passengerID, pageIndex, statusFilter)

	data, err := json.Marshal(previews)
	if err != nil {
		c.logger.Error("cache marshal failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, passengerBookingsTTL).Err(); err != nil {
		c.logger.Error("cache set failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}

	return nil
}

// InvalidatePassengerBookings supprime toutes les pages de réservations d'un passager.
func (c *BookingCache) InvalidatePassengerBookings(ctx context.Context, passengerID string) {
	pattern := keyPrefix + "passenger:" + passengerID + ":*"
	var keys []string

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		c.logger.Warn("cache scan failed during invalidation", zap.Error(err), zap.String("passengerID", passengerID))
		return
	}

	if len(keys) == 0 {
		return
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.String("passengerID", passengerID), zap.Int("keys", len(keys)))
	}
}

// InvalidateDriverTripBookings supprime toutes les pages de réservations d'un trajet conducteur.
func (c *BookingCache) InvalidateDriverTripBookings(ctx context.Context, driverID, tripID string) {
	pattern := keyPrefix + "driver-trip:" + driverID + ":" + tripID + ":*"
	var keys []string

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		c.logger.Warn("cache scan failed during invalidation", zap.Error(err))
		return
	}

	if len(keys) == 0 {
		return
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache invalidation failed", zap.Error(err), zap.Int("keys", len(keys)))
	}
}

// =============================================================================
// Atomic Seat Counter (compteur atomique de places disponibles)
// =============================================================================

// InitSeatCounter initialise le compteur de places si la clé n'existe pas.
// Retourne true si le compteur a été initialisé, false s'il existait déjà.
func (c *BookingCache) InitSeatCounter(ctx context.Context, tripID string, availableSeats int) (bool, error) {
	key := c.seatCounterKey(tripID)

	set, err := c.client.SetNX(ctx, key, availableSeats, seatCounterTTL).Result()
	if err != nil {
		c.logger.Error("seat counter init failed", zap.Error(err), zap.String("tripID", tripID))
		return false, fmt.Errorf("seat counter init: %w", err)
	}

	if set {
		c.logger.Debug("seat counter initialized", zap.String("tripID", tripID), zap.Int("seats", availableSeats))
	}
	return set, nil
}

// ReserveSeats réserve des places de manière atomique.
// Si le résultat après DECRBY est < 0, restaure les places et retourne une erreur.
func (c *BookingCache) ReserveSeats(ctx context.Context, tripID string, seats int) error {
	key := c.seatCounterKey(tripID)

	result, err := c.client.DecrBy(ctx, key, int64(seats)).Result()
	if err != nil {
		c.logger.Error("seat reserve failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("seat reserve: %w", err)
	}

	if result < 0 {
		// Restaurer les places — pas assez de places disponibles
		c.client.IncrBy(ctx, key, int64(seats)) //nolint:errcheck
		c.logger.Debug("seat reserve rejected: insufficient seats",
			zap.String("tripID", tripID),
			zap.Int("requested", seats),
			zap.Int64("resultAfterDecr", result),
		)
		return fmt.Errorf("no seats available")
	}

	c.logger.Debug("seats reserved", zap.String("tripID", tripID), zap.Int("seats", seats), zap.Int64("remaining", result))
	return nil
}

// RestoreSeats restaure des places (annulation, rejet).
func (c *BookingCache) RestoreSeats(ctx context.Context, tripID string, seats int) error {
	key := c.seatCounterKey(tripID)

	result, err := c.client.IncrBy(ctx, key, int64(seats)).Result()
	if err != nil {
		c.logger.Error("seat restore failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("seat restore: %w", err)
	}

	c.logger.Debug("seats restored", zap.String("tripID", tripID), zap.Int("seats", seats), zap.Int64("total", result))
	return nil
}

// GetSeatCounter retourne le nombre de places disponibles.
// Retourne -1, nil si la clé n'existe pas.
func (c *BookingCache) GetSeatCounter(ctx context.Context, tripID string) (int, error) {
	key := c.seatCounterKey(tripID)

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, nil
		}
		return -1, fmt.Errorf("seat counter get: %w", err)
	}

	seats, err := strconv.Atoi(val)
	if err != nil {
		return -1, fmt.Errorf("seat counter parse: %w", err)
	}

	return seats, nil
}

// SetSeatCounter force la valeur du compteur (utilisé par le job de réconciliation).
func (c *BookingCache) SetSeatCounter(ctx context.Context, tripID string, seats int) error {
	key := c.seatCounterKey(tripID)

	if err := c.client.Set(ctx, key, seats, seatCounterTTL).Err(); err != nil {
		return fmt.Errorf("seat counter set: %w", err)
	}

	return nil
}

// =============================================================================
// Key helpers
// =============================================================================

func (c *BookingCache) bookingDetailKey(bookingID string) string {
	return keyPrefix + "detail:" + bookingID
}

func (c *BookingCache) passengerBookingsKey(passengerID string, pageIndex int, statusFilter string) string {
	base := keyPrefix + "passenger:" + passengerID + ":page:" + strconv.Itoa(pageIndex)
	if statusFilter != "" {
		base += ":status:" + statusFilter
	}
	return base
}

func (c *BookingCache) driverTripBookingsKey(driverID, tripID string, pageIndex int) string {
	return keyPrefix + "driver-trip:" + driverID + ":" + tripID + ":page:" + strconv.Itoa(pageIndex)
}

func (c *BookingCache) seatCounterKey(tripID string) string {
	return seatPrefix + tripID + ":available_seats"
}
