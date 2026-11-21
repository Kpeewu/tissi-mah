-- =============================================================================
-- Migration : paiement FedaPay avec redirection
-- =============================================================================

-- Lien de la page de paiement FedaPay (réutilisé si le passager relance createPayment).
-- Dernier statut FedaPay observé : declined/canceled ne sont pas finaux, le paiement
-- reste 'pending' mais le mobile doit pouvoir afficher l'échec de la tentative.
-- expires_at : fin de la fenêtre de paiement (création de la réservation + PAYMENT_TIMEOUT_MINUTES).
-- Passé ce délai, le worker supprime la transaction FedaPay et libère les places.
ALTER TABLE payments
    ADD COLUMN payment_url                TEXT,
    ADD COLUMN provider_status            VARCHAR(32),
    ADD COLUMN provider_status_updated_at TIMESTAMPTZ,
    ADD COLUMN expires_at                 TIMESTAMPTZ;

UPDATE payments SET expires_at = created_at + INTERVAL '10 minutes' WHERE expires_at IS NULL;

ALTER TABLE payments ALTER COLUMN expires_at SET NOT NULL;

-- Index partiel pour le worker d'expiration.
CREATE INDEX idx_payments_pending_expires_at
    ON payments(expires_at)
    WHERE status = 'pending';

-- Remboursement intégral quand un paiement approuvé tardivement ne peut plus
-- être rattaché à des places (réservation déjà libérée et trajet complet).
ALTER TYPE refund_reason ADD VALUE IF NOT EXISTS 'seatsUnavailable';
