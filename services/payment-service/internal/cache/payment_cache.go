package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/payment-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	paymentByBookingTTL = 2 * time.Minute
	payoutLockTTL       = 10 * time.Minute
	tripPayoutLockTTL   = 2 * time.Minute
	refundLockTTL       = 2 * time.Minute
	expirationLockTTL   = 5 * time.Minute
	keyPrefix           = "payment:"
)

// PaymentCache gère le cache Redis pour le payment-service.
type PaymentCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewPaymentCache crée une instance du cache payment.
func NewPaymentCache(client *redis.Client, logger *zap.Logger) *PaymentCache {
	return &PaymentCache{client: client, logger: logger.Named("cache")}
}

// GetPaymentByBookingID récupère un paiement depuis le cache par bookingID.
func (c *PaymentCache) GetPaymentByBookingID(ctx context.Context, bookingID string) (*domain.Payment, error) {
	key := keyPrefix + "booking:" + bookingID

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		c.logger.Error("cache get failed", zap.Error(err), zap.String("key", key))
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var payment domain.Payment
	if err := json.Unmarshal(data, &payment); err != nil {
		c.logger.Error("cache unmarshal failed", zap.Error(err))
		c.client.Del(ctx, key) //nolint:errcheck
		return nil, nil
	}

	return &payment, nil
}

// SetPaymentByBookingID stocke un paiement dans le cache par bookingID.
func (c *PaymentCache) SetPaymentByBookingID(ctx context.Context, bookingID string, payment *domain.Payment) error {
	key := keyPrefix + "booking:" + bookingID

	data, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	return c.client.Set(ctx, key, data, paymentByBookingTTL).Err()
}

// InvalidatePaymentByBookingID supprime le cache d'un paiement.
func (c *PaymentCache) InvalidatePaymentByBookingID(ctx context.Context, bookingID string) {
	key := keyPrefix + "booking:" + bookingID
	c.client.Del(ctx, key) //nolint:errcheck
}

// AcquirePayoutLock tente d'acquérir un verrou distribué pour le payout worker.
func (c *PaymentCache) AcquirePayoutLock(ctx context.Context) (bool, error) {
	key := keyPrefix + "payout:lock"
	return c.client.SetNX(ctx, key, "locked", payoutLockTTL).Result()
}

// ReleasePayoutLock libère le verrou du payout worker.
func (c *PaymentCache) ReleasePayoutLock(ctx context.Context) {
	key := keyPrefix + "payout:lock"
	c.client.Del(ctx, key) //nolint:errcheck
}

// AcquireTripPayoutLock tente d'acquérir un verrou distribué pour le payout d'un trip spécifique.
func (c *PaymentCache) AcquireTripPayoutLock(ctx context.Context, tripID string) (bool, error) {
	key := keyPrefix + "payout:trip:" + tripID + ":lock"
	return c.client.SetNX(ctx, key, "locked", tripPayoutLockTTL).Result()
}

// ReleaseTripPayoutLock libère le verrou de payout d'un trip.
func (c *PaymentCache) ReleaseTripPayoutLock(ctx context.Context, tripID string) {
	key := keyPrefix + "payout:trip:" + tripID + ":lock"
	c.client.Del(ctx, key) //nolint:errcheck
}

// AcquireRefundLock tente d'acquérir un verrou distribué pour le traitement d'un refund spécifique.
func (c *PaymentCache) AcquireRefundLock(ctx context.Context, refundID string) (bool, error) {
	key := keyPrefix + "refund:lock:" + refundID
	return c.client.SetNX(ctx, key, "locked", refundLockTTL).Result()
}

// ReleaseRefundLock libère le verrou d'un refund.
func (c *PaymentCache) ReleaseRefundLock(ctx context.Context, refundID string) {
	key := keyPrefix + "refund:lock:" + refundID
	c.client.Del(ctx, key) //nolint:errcheck
}

// AcquireExpirationLock tente d'acquérir un verrou distribué pour l'expiration worker.
func (c *PaymentCache) AcquireExpirationLock(ctx context.Context) (bool, error) {
	key := keyPrefix + "expiration:lock"
	return c.client.SetNX(ctx, key, "locked", expirationLockTTL).Result()
}

// ReleaseExpirationLock libère le verrou de l'expiration worker.
func (c *PaymentCache) ReleaseExpirationLock(ctx context.Context) {
	key := keyPrefix + "expiration:lock"
	c.client.Del(ctx, key) //nolint:errcheck
}
