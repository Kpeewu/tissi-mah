-- Recréer l'enum payment_method sans card et paypal (aucune ligne ne les utilise)
ALTER TYPE payment_method RENAME TO payment_method_old;
CREATE TYPE payment_method AS ENUM ('mobileMoney');
ALTER TABLE payments
    ALTER COLUMN payment_method TYPE payment_method
    USING payment_method::text::payment_method;
DROP TYPE payment_method_old;
