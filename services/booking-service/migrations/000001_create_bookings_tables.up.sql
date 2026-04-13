-- =============================================================================
-- Migration : création des tables du booking-service
-- =============================================================================

-- Enumerations
CREATE TYPE booking_status AS ENUM (
    'created',
    'paymentPending',
    'pendingApproval',
    'approved',
    'rejected',
    'cancelled',
    'inProgress',
    'completed',
    'noShow',
    'expired'
);

CREATE TYPE booking_payment_method AS ENUM (
    'mobileMoney',
    'card',
    'paypal',
    'cash'
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
-- Table : bookings
-- =============================================================================
CREATE TABLE IF NOT EXISTS bookings (
    booking_id           VARCHAR(36)            NOT NULL PRIMARY KEY,
    booking_reference    VARCHAR(128)           UNIQUE NOT NULL,
    trip_id              VARCHAR(36)            NOT NULL,
    passenger_id         VARCHAR(36)            NOT NULL,
    driver_id            VARCHAR(36)            NOT NULL,
    pickup_waypoint_id   VARCHAR(36)            NOT NULL,
    dropoff_waypoint_id  VARCHAR(36)            NOT NULL,
    seats_booked         SMALLINT               NOT NULL,
    price_per_seat       INTEGER                NOT NULL,
    subtotal             INTEGER                NOT NULL,
    service_fee          INTEGER                NOT NULL,
    total_amount         INTEGER                NOT NULL,
    payment_method       booking_payment_method NOT NULL,
    status               booking_status         NOT NULL DEFAULT 'created',

    -- Timestamps d'état
    payment_completed_at TIMESTAMPTZ,
    approved_at          TIMESTAMPTZ,
    rejected_at          TIMESTAMPTZ,
    cancelled_at         TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,

    -- Annulation
    canceller_id         VARCHAR(36),
    cancellation_reason  TEXT,

    -- No-show
    no_show_type         VARCHAR(20),
    no_show_reported_by  VARCHAR(36),
    no_show_reported_at  TIMESTAMPTZ,
    no_show_description  TEXT,

    -- Timestamps système
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Contraintes
    CONSTRAINT ck_bookings_seats_positive CHECK (seats_booked >= 1),
    CONSTRAINT ck_bookings_price_non_negative CHECK (price_per_seat >= 0),
    CONSTRAINT ck_bookings_service_fee_non_negative CHECK (service_fee >= 0),
    CONSTRAINT ck_bookings_subtotal CHECK (subtotal = seats_booked * price_per_seat),
    CONSTRAINT ck_bookings_total CHECK (total_amount = subtotal + service_fee),
    CONSTRAINT ck_bookings_no_show_type CHECK (
        no_show_type IS NULL OR no_show_type IN ('driver', 'passenger')
    ),
    CONSTRAINT ck_bookings_cancellation CHECK (
        status != 'cancelled'
        OR (canceller_id IS NOT NULL AND cancellation_reason IS NOT NULL)
    ),
    CONSTRAINT ck_bookings_no_show CHECK (
        status != 'noShow'
        OR (no_show_type IS NOT NULL AND no_show_reported_by IS NOT NULL)
    )
);

-- Index
CREATE INDEX idx_bookings_trip_id ON bookings(trip_id);
CREATE INDEX idx_bookings_passenger_id ON bookings(passenger_id);
CREATE INDEX idx_bookings_driver_id ON bookings(driver_id);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_trip_status ON bookings(trip_id, status);
CREATE INDEX idx_bookings_passenger_status ON bookings(passenger_id, status, created_at DESC);
CREATE INDEX idx_bookings_reference ON bookings(booking_reference);
CREATE INDEX idx_bookings_created_at ON bookings(created_at DESC);

-- Trigger auto-update updated_at
CREATE TRIGGER update_bookings_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : bookings_segments
-- =============================================================================
CREATE TABLE IF NOT EXISTS bookings_segments (
    segment_id               VARCHAR(36)  NOT NULL PRIMARY KEY,
    booking_id               VARCHAR(36)  NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    pickup_waypoint_id       VARCHAR(36)  NOT NULL,
    dropoff_waypoint_id      VARCHAR(36)  NOT NULL,
    pickup_location_name     VARCHAR(255),
    pickup_city              VARCHAR(128),
    pickup_lat               DECIMAL(10,7),
    pickup_lng               DECIMAL(10,7),
    pickup_scheduled_at      TIMESTAMPTZ,
    pickup_actual_at         TIMESTAMPTZ,
    dropoff_location_name    VARCHAR(255),
    dropoff_city             VARCHAR(128),
    dropoff_lat              DECIMAL(10,7),
    dropoff_lng              DECIMAL(10,7),
    dropoff_scheduled_at     TIMESTAMPTZ,
    dropoff_actual_at        TIMESTAMPTZ,
    segment_distance_meters  INTEGER,
    segment_duration_minutes INTEGER,
    segment_price            INTEGER,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookings_segments_booking_id ON bookings_segments(booking_id);

-- =============================================================================
-- Table : bookings_status_history
-- =============================================================================
CREATE TABLE IF NOT EXISTS bookings_status_history (
    history_id      VARCHAR(36)  NOT NULL PRIMARY KEY,
    booking_id      VARCHAR(36)  NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    previous_status VARCHAR(20)  NOT NULL,
    new_status      VARCHAR(20)  NOT NULL,
    changed_by      VARCHAR(36)  NOT NULL,
    changed_by_type VARCHAR(20)  NOT NULL,
    change_reason   TEXT,
    metadata        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_status_history_changed_by_type CHECK (
        changed_by_type IN ('passenger', 'driver', 'admin', 'system')
    )
);

CREATE INDEX idx_bookings_status_history_booking_id ON bookings_status_history(booking_id);
CREATE INDEX idx_bookings_status_history_created_at ON bookings_status_history(booking_id, created_at DESC);
