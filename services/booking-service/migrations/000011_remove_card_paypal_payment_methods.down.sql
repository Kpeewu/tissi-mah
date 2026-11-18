-- Restaurer l'enum booking_payment_method avec card et paypal
ALTER TYPE booking_payment_method RENAME TO booking_payment_method_old;
CREATE TYPE booking_payment_method AS ENUM ('mobileMoney', 'card', 'paypal', 'cash');
ALTER TABLE bookings
    ALTER COLUMN payment_method TYPE booking_payment_method
    USING payment_method::text::booking_payment_method;
DROP TYPE booking_payment_method_old;
