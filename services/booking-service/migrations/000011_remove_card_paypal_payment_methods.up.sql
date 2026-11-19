-- Annuler les réservations bloquées en paymentPending avec card ou paypal
UPDATE bookings
SET status = 'cancelled'
WHERE payment_method IN ('card', 'paypal')
  AND status = 'paymentPending';

-- L'index partiel du worker de libération compare payment_method à un littéral de l'enum :
-- il doit être supprimé avant de changer le type de la colonne, puis recréé.
DROP INDEX IF EXISTS idx_bookings_payment_release_pending;

-- Recréer l'enum booking_payment_method sans card et paypal
ALTER TYPE booking_payment_method RENAME TO booking_payment_method_old;
CREATE TYPE booking_payment_method AS ENUM ('mobileMoney', 'cash');
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
