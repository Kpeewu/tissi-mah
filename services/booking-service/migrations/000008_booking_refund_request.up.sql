-- =============================================================================
-- Migration 000008 : demande de remboursement durable
-- =============================================================================

-- refund_reason : motif du remboursement dû à payment-service, posé dans la même transaction que
-- l'annulation / le rejet / le no-show d'une réservation payée en ligne (NULL = rien à rembourser).
-- refund_requested_at : demande acquittée par payment-service.
-- refund_request_attempts : refus définitifs de payment-service ; au-delà du plafond, la demande
-- n'est plus rejouée (traitement manuel par le support).
ALTER TABLE bookings
    ADD COLUMN refund_reason           VARCHAR(32) NULL,
    ADD COLUMN refund_requested_at     TIMESTAMPTZ NULL,
    ADD COLUMN refund_request_attempts SMALLINT    NOT NULL DEFAULT 0;

-- Index pour le worker de rejeu des demandes de remboursement
CREATE INDEX idx_bookings_refund_due
    ON bookings (updated_at)
    WHERE refund_reason IS NOT NULL AND refund_requested_at IS NULL;
