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

	// TTL du cache pour la liste agrégée des demandes en attente du conducteur
	driverPendingBookingsTTL = 60 * time.Second

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

// =============================================================================
// Driver Pending Bookings cache (liste agrégée toutes trajets)
// =============================================================================

// GetDriverPendingBookings récupère le JSON sérialisé de la liste depuis le cache.
// Retourne nil, nil si cache miss.
func (c *BookingCache) GetDriverPendingBookings(ctx context.Context, driverID string, pageIndex int) ([]byte, error) {
	key := c.driverPendingBookingsKey(driverID, pageIndex)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		c.logger.Error("cache get driver pending failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}
	return data, nil
}

// SetDriverPendingBookings stocke le JSON sérialisé de la liste dans le cache.
func (c *BookingCache) SetDriverPendingBookings(ctx context.Context, driverID string, pageIndex int, data []byte) error {
	key := c.driverPendingBookingsKey(driverID, pageIndex)
	if err := c.client.Set(ctx, key, data, driverPendingBookingsTTL).Err(); err != nil {
		c.logger.Error("cache set driver pending failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// InvalidateDriverPendingBookings supprime toutes les pages de demandes en attente d'un conducteur.
func (c *BookingCache) InvalidateDriverPendingBookings(ctx context.Context, driverID string) {
	pattern := keyPrefix + "driver-pending:" + driverID + ":*"
	var keys []string

	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		c.logger.Warn("cache scan failed during driver pending invalidation", zap.Error(err), zap.String("driverID", driverID))
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		c.logger.Warn("cache driver pending invalidation failed", zap.Error(err), zap.String("driverID", driverID), zap.Int("keys", len(keys)))
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
// Per-segment seat management
// =============================================================================

// InitSegmentSeatCounters initialise les compteurs de places par segment.
// maxOrder est le plus grand sequencer_order parmi les waypoints du trajet.
// Crée un compteur pour chaque leg (order 1 à maxOrder-1) via SETNX.
func (c *BookingCache) InitSegmentSeatCounters(ctx context.Context, tripID string, totalSeats int, maxOrder int) error {
	for order := 1; order < maxOrder; order++ {
		key := c.segmentSeatKey(tripID, order)
		if err := c.client.SetNX(ctx, key, totalSeats, seatCounterTTL).Err(); err != nil {
			c.logger.Error("segment seat counter init failed", zap.Error(err), zap.String("tripID", tripID), zap.Int("order", order))
			return fmt.Errorf("segment seat counter init: %w", err)
		}
	}

	c.logger.Debug("segment seat counters initialized", zap.String("tripID", tripID), zap.Int("totalSeats", totalSeats), zap.Int("legs", maxOrder-1))
	return nil
}

// reserveSegmentSeatsScript vérifie que chaque leg a assez de places puis les décrémente atomiquement.
// KEYS: les clés de chaque leg concerné
// ARGV[1]: nombre de places à réserver
// Retourne 1 si OK, -1 si pas assez de places.
var reserveSegmentSeatsScript = redis.NewScript(`
local seats = tonumber(ARGV[1])
-- Vérifier que chaque leg a assez de places
for i, key in ipairs(KEYS) do
    local avail = tonumber(redis.call('GET', key) or '0')
    if avail < seats then
        return -1
    end
end
-- Décrémenter atomiquement chaque leg
for i, key in ipairs(KEYS) do
    redis.call('DECRBY', key, seats)
end
return 1
`)

// ReserveSegmentSeats réserve des places sur les legs [pickupOrder, dropoffOrder-1] de manière atomique.
func (c *BookingCache) ReserveSegmentSeats(ctx context.Context, tripID string, pickupOrder, dropoffOrder, seats int) error {
	keys := make([]string, 0, dropoffOrder-pickupOrder)
	for order := pickupOrder; order < dropoffOrder; order++ {
		keys = append(keys, c.segmentSeatKey(tripID, order))
	}

	result, err := reserveSegmentSeatsScript.Run(ctx, c.client, keys, seats).Int()
	if err != nil {
		c.logger.Error("reserve segment seats script failed", zap.Error(err), zap.String("tripID", tripID))
		return fmt.Errorf("reserve segment seats: %w", err)
	}

	if result == -1 {
		c.logger.Debug("reserve segment seats rejected: insufficient seats",
			zap.String("tripID", tripID),
			zap.Int("pickupOrder", pickupOrder),
			zap.Int("dropoffOrder", dropoffOrder),
			zap.Int("requested", seats),
		)
		return fmt.Errorf("no seats available")
	}

	c.logger.Debug("segment seats reserved",
		zap.String("tripID", tripID),
		zap.Int("pickupOrder", pickupOrder),
		zap.Int("dropoffOrder", dropoffOrder),
		zap.Int("seats", seats),
	)
	return nil
}

// RestoreSegmentSeats restaure des places sur les legs [pickupOrder, dropoffOrder-1].
func (c *BookingCache) RestoreSegmentSeats(ctx context.Context, tripID string, pickupOrder, dropoffOrder, seats int) error {
	for order := pickupOrder; order < dropoffOrder; order++ {
		key := c.segmentSeatKey(tripID, order)
		if err := c.client.IncrBy(ctx, key, int64(seats)).Err(); err != nil {
			c.logger.Error("restore segment seats failed", zap.Error(err), zap.String("tripID", tripID), zap.Int("order", order))
			return fmt.Errorf("restore segment seats: %w", err)
		}
	}

	c.logger.Debug("segment seats restored",
		zap.String("tripID", tripID),
		zap.Int("pickupOrder", pickupOrder),
		zap.Int("dropoffOrder", dropoffOrder),
		zap.Int("seats", seats),
	)
	return nil
}

// GetMinSegmentSeats retourne le minimum de places disponibles entre pickupOrder et dropoffOrder-1.
func (c *BookingCache) GetMinSegmentSeats(ctx context.Context, tripID string, pickupOrder, dropoffOrder int) (int, error) {
	minSeats := -1
	for order := pickupOrder; order < dropoffOrder; order++ {
		key := c.segmentSeatKey(tripID, order)
		val, err := c.client.Get(ctx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return -1, nil // pas encore initialisé
			}
			return -1, fmt.Errorf("get segment seat: %w", err)
		}
		seats, err := strconv.Atoi(val)
		if err != nil {
			return -1, fmt.Errorf("parse segment seat: %w", err)
		}
		if minSeats == -1 || seats < minSeats {
			minSeats = seats
		}
	}
	return minSeats, nil
}

// SetSegmentSeatCounter force la valeur du compteur pour un leg (réconciliation).
func (c *BookingCache) SetSegmentSeatCounter(ctx context.Context, tripID string, order, seats int) error {
	key := c.segmentSeatKey(tripID, order)
	if err := c.client.Set(ctx, key, seats, seatCounterTTL).Err(); err != nil {
		return fmt.Errorf("set segment seat counter: %w", err)
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

func (c *BookingCache) driverPendingBookingsKey(driverID string, pageIndex int) string {
	return keyPrefix + "driver-pending:" + driverID + ":page:" + strconv.Itoa(pageIndex)
}

func (c *BookingCache) seatCounterKey(tripID string) string {
	return seatPrefix + tripID + ":available_seats"
}

func (c *BookingCache) segmentSeatKey(tripID string, order int) string {
	return seatPrefix + tripID + ":seg:" + strconv.Itoa(order) + ":seats"
}
