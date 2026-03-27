-- Ajout du champ payment_released_at pour tracker la libération des paiements
ALTER TABLE bookings ADD COLUMN payment_released_at TIMESTAMPTZ;

-- Index pour le worker de libération des paiements
CREATE INDEX idx_bookings_payment_release_pending
    ON bookings (completed_at)
    WHERE status = 'completed'
      AND payment_method != 'cash'
      AND payment_completed_at IS NOT NULL
      AND payment_released_at IS NULL;
