-- =============================================================================
-- Migration : suivi du payout chauffeur issu d'un remboursement
-- Contexte : annulation tardive (50 %) et no-show passager (100 %) donnent une
--            part au chauffeur via FedaPay, traitée par un worker dédié.
-- =============================================================================

ALTER TABLE refunds
    ADD COLUMN driver_payout_status        VARCHAR(32),   -- 'pending'|'processing'|'completed'|'failed'
    ADD COLUMN driver_payout_provider_ref  VARCHAR(256),  -- ID FedaPay du virement chauffeur
    ADD COLUMN driver_payout_retry_count   SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN driver_payout_last_retry_at TIMESTAMPTZ,
    ADD COLUMN driver_payout_completed_at  TIMESTAMPTZ,
    ADD CONSTRAINT chk_driver_payout_retry_count CHECK (driver_payout_retry_count >= 0);

-- Index pour que le worker charge uniquement les refunds en attente d'un payout chauffeur.
CREATE INDEX idx_refunds_driver_payout_pending
    ON refunds(driver_payout_status, updated_at)
    WHERE amount_to_driver > 0 AND driver_payout_status = 'pending';

-- Index pour le réconciliateur (virements démarrés mais non confirmés).
CREATE INDEX idx_refunds_driver_payout_processing
    ON refunds(driver_payout_status, updated_at)
    WHERE amount_to_driver > 0 AND driver_payout_status = 'processing';
