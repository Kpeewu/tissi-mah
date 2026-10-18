-- Annuler les réservations bloquées en paymentPending avec card ou paypal
UPDATE bookings
SET status = 'cancelled'
WHERE payment_method IN ('card', 'paypal')
  AND status = 'paymentPending';

-- Recréer l'enum booking_payment_method sans card et paypal
ALTER TYPE booking_payment_method RENAME TO booking_payment_method_old;
CREATE TYPE booking_payment_method AS ENUM ('mobileMoney', 'cash');
ALTER TABLE bookings
    ALTER COLUMN payment_method TYPE booking_payment_method
    USING payment_method::text::booking_payment_method;
DROP TYPE booking_payment_method_old;
