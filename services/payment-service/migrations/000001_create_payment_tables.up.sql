-- =============================================================================
-- Migration : création des tables du payment-service
-- =============================================================================

-- Enumerations
CREATE TYPE payment_status AS ENUM (
    'pending',
    'held',
    'released',
    'paidOut',
    'failed',
    'refunded'
);

CREATE TYPE payment_method AS ENUM (
    'mobileMoney',
    'card',
    'paypal'
);

CREATE TYPE refund_reason AS ENUM (
    'cancelledByDriver',
    'cancelledByPassenger',
    'noShowDriver',
    'noShowPassenger',
    'tripCancelled',
    'bookingRejected',
    'dispute',
    'other'
);

CREATE TYPE refund_rule AS ENUM (
    'cancelled24hBefore',
    'cancelled30minAfterApproval',
    'cancelledOver30minAfterApproval',
    'driverCancellation',
    'noShowDriver',
    'noShowPassenger'
);

CREATE TYPE refund_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed'
);

CREATE TYPE payout_status AS ENUM (
    'pending',
    'scheduled',
    'processing',
    'completed',
    'failed',
    'cancelled'
);

-- Fonction de mise à jour automatique de updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Table : payments
-- =============================================================================
CREATE TABLE IF NOT EXISTS payments (
    payment_id              VARCHAR(36)      NOT NULL PRIMARY KEY,
    booking_id              VARCHAR(36)      NOT NULL,
    trip_id                 VARCHAR(36)      NOT NULL,
    amount                  INTEGER          NOT NULL,
    payment_method          payment_method   NOT NULL,
    payment_provider        VARCHAR(64)      NOT NULL DEFAULT 'fedapay',
    status                  payment_status   NOT NULL DEFAULT 'pending',
    external_transaction_id VARCHAR(256),
    payment_reference       VARCHAR(128)     UNIQUE NOT NULL,
    created_at              TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ,
    failed_at               TIMESTAMPTZ,
    failure_reason          TEXT,
    metadata                JSONB,
    updated_at              TIMESTAMPTZ      NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payment_amount_positive CHECK (amount > 0)
);

CREATE INDEX idx_payments_booking_id ON payments(booking_id);
CREATE INDEX idx_payments_trip_id ON payments(trip_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_external_transaction_id ON payments(external_transaction_id);
CREATE INDEX idx_payments_payment_reference ON payments(payment_reference);
CREATE INDEX idx_payments_created_at ON payments(created_at DESC);
CREATE INDEX idx_payments_trip_status ON payments(trip_id, status);

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : refunds
-- =============================================================================
CREATE TABLE IF NOT EXISTS refunds (
    refund_id            VARCHAR(36)    NOT NULL PRIMARY KEY,
    refund_reference     VARCHAR(128)   UNIQUE NOT NULL,
    payment_id           VARCHAR(36)    NOT NULL REFERENCES payments(payment_id),
    booking_id           VARCHAR(36)    NOT NULL,
    refund_reason        refund_reason  NOT NULL,
    refund_rule_applied  refund_rule    NOT NULL,
    original_amount      INTEGER        NOT NULL,
    refund_percentage    SMALLINT       NOT NULL,
    refund_amount        INTEGER        NOT NULL,
    service_fee_refunded BOOLEAN        NOT NULL DEFAULT FALSE,
    amount_to_passenger  INTEGER        NOT NULL DEFAULT 0,
    amount_to_driver     INTEGER        NOT NULL DEFAULT 0,
    amount_to_platform   INTEGER        NOT NULL DEFAULT 0,
    status               refund_status  NOT NULL DEFAULT 'pending',
    refund_method        VARCHAR(64),
    processed_at         TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,
    estimated_completion TIMESTAMPTZ,
    passenger_notified   BOOLEAN        NOT NULL DEFAULT FALSE,
    notification_sent_at TIMESTAMPTZ,
    notes                TEXT,
    updated_at           TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_refund_percentage CHECK (refund_percentage >= 0 AND refund_percentage <= 100),
    CONSTRAINT chk_refund_amounts_non_negative CHECK (
        refund_amount >= 0 AND
        amount_to_passenger >= 0 AND
        amount_to_driver >= 0 AND
        amount_to_platform >= 0
    )
);

CREATE INDEX idx_refunds_payment_id ON refunds(payment_id);
CREATE INDEX idx_refunds_booking_id ON refunds(booking_id);
CREATE INDEX idx_refunds_status ON refunds(status);

CREATE TRIGGER trg_refunds_updated_at
    BEFORE UPDATE ON refunds
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : payouts
-- =============================================================================
CREATE TABLE IF NOT EXISTS payouts (
    payout_id                  VARCHAR(36)    NOT NULL PRIMARY KEY,
    payout_reference           VARCHAR(128)   UNIQUE NOT NULL,
    driver_id                  VARCHAR(36)    NOT NULL,
    trip_id                    VARCHAR(36)    NOT NULL,
    gross_amount               INTEGER        NOT NULL,
    platform_fee               INTEGER        NOT NULL DEFAULT 0,
    net_amount                 INTEGER        NOT NULL,
    payout_method              VARCHAR(64),
    payout_destination         VARCHAR(128),
    destination_name           VARCHAR(256),
    status                     payout_status  NOT NULL DEFAULT 'pending',
    scheduled_at               TIMESTAMPTZ,
    completed_at               TIMESTAMPTZ,
    failed_at                  TIMESTAMPTZ,
    cancelled_at               TIMESTAMPTZ,
    payment_provider           VARCHAR(64)    NOT NULL DEFAULT 'fedapay',
    payment_provider_reference VARCHAR(256),
    failure_reason             TEXT,
    retry_count                SMALLINT       NOT NULL DEFAULT 0,
    last_retry_at              TIMESTAMPTZ,
    created_at                 TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payout_amounts_non_negative CHECK (
        gross_amount >= 0 AND
        platform_fee >= 0 AND
        net_amount >= 0
    ),
    CONSTRAINT chk_payout_retry_count CHECK (retry_count >= 0)
);

CREATE INDEX idx_payouts_driver_id ON payouts(driver_id);
CREATE INDEX idx_payouts_trip_id ON payouts(trip_id);
CREATE INDEX idx_payouts_status ON payouts(status);
CREATE INDEX idx_payouts_scheduled_at ON payouts(scheduled_at);
CREATE INDEX idx_payouts_created_at ON payouts(created_at DESC);

CREATE TRIGGER trg_payouts_updated_at
    BEFORE UPDATE ON payouts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : webhook_events (idempotence)
-- =============================================================================
CREATE TABLE IF NOT EXISTS webhook_events (
    event_id        VARCHAR(36)   NOT NULL PRIMARY KEY,
    fedapay_event_id VARCHAR(256) UNIQUE NOT NULL,
    event_type      VARCHAR(128)  NOT NULL,
    payload         JSONB         NOT NULL,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_events_event_type ON webhook_events(event_type);
CREATE INDEX idx_webhook_events_created_at ON webhook_events(created_at DESC);

-- =============================================================================
-- Table : payout_batches
-- =============================================================================
CREATE TABLE IF NOT EXISTS payout_batches (
    batch_id         VARCHAR(36)   NOT NULL PRIMARY KEY,
    status           VARCHAR(32)   NOT NULL DEFAULT 'pending',
    total_payouts    INTEGER       NOT NULL DEFAULT 0,
    successful_count INTEGER       NOT NULL DEFAULT 0,
    failed_count     INTEGER       NOT NULL DEFAULT 0,
    total_amount     INTEGER       NOT NULL DEFAULT 0,
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payout_batches_status ON payout_batches(status);
CREATE INDEX idx_payout_batches_created_at ON payout_batches(created_at DESC);
