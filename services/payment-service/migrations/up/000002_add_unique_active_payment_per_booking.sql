-- Empêcher deux paiements actifs (pending ou held) pour le même booking.
-- Permet les retries après échec (failed, refunded).
CREATE UNIQUE INDEX idx_unique_active_payment_per_booking
    ON payments (booking_id)
    WHERE status IN ('pending', 'held');
