-- =============================================================================
-- Migration : suivi du rattachement des paiements held à leur réservation
-- =============================================================================

-- booking_synced_at : rattachement du paiement held à la réservation abouti
-- (ConfirmPayment accepté ou réservation déjà confirmée). NULL sur un paiement held
-- = rattachement à retenter (booking-service indisponible lors de l'approbation).
ALTER TABLE payments ADD COLUMN booking_synced_at TIMESTAMPTZ;

-- booking_sync_attempts : reprises de rattachement échouées. Au plafond (5), le rattachement
-- est abandonné et le passager est remboursé intégralement.
ALTER TABLE payments ADD COLUMN booking_sync_attempts SMALLINT NOT NULL DEFAULT 0;

-- Paiements existants déjà traités : pas de reprise
UPDATE payments SET booking_synced_at = COALESCE(completed_at, NOW())
 WHERE status IN ('held', 'released', 'paidOut', 'refunded');

-- Index partiel pour le worker de rattachement.
CREATE INDEX idx_payments_held_unsynced
    ON payments(completed_at)
    WHERE status = 'held' AND booking_synced_at IS NULL;
