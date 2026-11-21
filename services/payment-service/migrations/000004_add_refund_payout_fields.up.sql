-- =============================================================================
-- Migration : extension de payments et refunds pour le remboursement effectif
-- =============================================================================

-- Numéro de téléphone du passager utilisé lors du paiement initial.
-- Réutilisé au moment du refund pour envoyer les fonds via FedaPay CreatePayout.
ALTER TABLE payments
    ADD COLUMN passenger_phone_number TEXT NOT NULL DEFAULT '';

-- Enrichir refunds pour tracer le transfert FedaPay (parité avec payouts).
ALTER TABLE refunds
    ADD COLUMN payout_destination         TEXT,
    ADD COLUMN payment_provider           VARCHAR(64)  NOT NULL DEFAULT 'fedapay',
    ADD COLUMN payment_provider_reference VARCHAR(256),
    ADD COLUMN failure_reason             TEXT,
    ADD COLUMN retry_count                SMALLINT     NOT NULL DEFAULT 0,
    ADD COLUMN last_retry_at              TIMESTAMPTZ,
    ADD CONSTRAINT chk_refund_retry_count CHECK (retry_count >= 0);

-- Index pour que le worker puisse retrouver rapidement les refunds à traiter.
CREATE INDEX idx_refunds_status_amount_to_passenger
    ON refunds(status, amount_to_passenger)
    WHERE amount_to_passenger > 0;
