-- =============================================================================
-- Migration : historique des statuts des paiements et des remboursements
-- =============================================================================

-- Une entrée par transition (ou événement notable) d'un paiement passager, sur le modèle
-- de payout_status_history. status contient le statut interne atteint (pending, held, failed,
-- released, refunded, paidOut) ou un événement sans changement de statut (attempt_failed,
-- booking_synced, booking_sync_failed, …).
CREATE TABLE payment_status_history (
    history_id      VARCHAR(36)  NOT NULL PRIMARY KEY,
    payment_id      VARCHAR(36)  NOT NULL REFERENCES payments(payment_id),
    status          VARCHAR(64)  NOT NULL,
    provider_status VARCHAR(32),                              -- statut FedaPay observé lors de l'événement
    initiated_by    VARCHAR(32)  NOT NULL DEFAULT 'system',   -- system | passenger | booking_service | support
    source          VARCHAR(32)  NOT NULL DEFAULT 'unknown',  -- canal : fedapay_webhook, expiration_worker, …
    occurred_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    notes           TEXT
);

CREATE INDEX idx_payment_status_history_payment_id ON payment_status_history(payment_id, occurred_at);

-- Une entrée par transition d'un remboursement (création, virement, reprises, échec définitif).
CREATE TABLE refund_status_history (
    history_id   VARCHAR(36)  NOT NULL PRIMARY KEY,
    refund_id    VARCHAR(36)  NOT NULL REFERENCES refunds(refund_id),
    status       VARCHAR(64)  NOT NULL,                       -- pending | processing | retry_scheduled | failed | completed
    retry_count  SMALLINT     NOT NULL DEFAULT 0,
    initiated_by VARCHAR(32)  NOT NULL DEFAULT 'system',
    source       VARCHAR(32)  NOT NULL DEFAULT 'unknown',
    occurred_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    notes        TEXT
);

CREATE INDEX idx_refund_status_history_refund_id ON refund_status_history(refund_id, occurred_at);
