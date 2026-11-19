-- Même contrainte qu'à l'aller : l'index partiel dépend de l'enum
DROP INDEX IF EXISTS idx_bookings_payment_release_pending;

-- Restaurer l'enum booking_payment_method avec card et paypal
ALTER TYPE booking_payment_method RENAME TO booking_payment_method_old;
CREATE TYPE booking_payment_method AS ENUM ('mobileMoney', 'card', 'paypal', 'cash');
ALTER TABLE bookings
    ALTER COLUMN payment_method TYPE booking_payment_method
    USING payment_method::text::booking_payment_method;
DROP TYPE booking_payment_method_old;

CREATE INDEX idx_bookings_payment_release_pending
    ON bookings (completed_at)
    WHERE status = 'completed'
      AND payment_method != 'cash'
      AND payment_completed_at IS NOT NULL
      AND payment_released_at IS NULL;
