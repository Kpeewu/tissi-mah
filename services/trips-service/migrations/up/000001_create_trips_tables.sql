-- =============================================================================
-- Extension PostGIS — indexation spatiale des waypoints
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS postgis;

-- =============================================================================
-- Énumérations
-- =============================================================================
CREATE TYPE trip_status AS ENUM (
    'scheduled',
    'inProgress',
    'completed',
    'cancelled'
);

CREATE TYPE waypoint_type AS ENUM (
    'departure',
    'stop',
    'arrival'
);

CREATE TYPE payment_method AS ENUM (
    'mobileMoney',
    'card',
    'paypal',
    'cash'
);

CREATE TYPE recurrence_type AS ENUM (
    'daily',
    'weekly',
    'custom'
);

-- =============================================================================
-- Fonction de mise à jour automatique de updated_at
-- =============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- Table : recurring_patterns
-- Définit les schémas de trajets récurrents (quotidien, hebdo, custom).
-- Doit être créée avant trips (FK nullable).
-- =============================================================================
CREATE TABLE IF NOT EXISTS recurring_patterns (
    trip_pattern_id         TEXT                NOT NULL PRIMARY KEY,
    driver_id               TEXT                NOT NULL,
    vehicle_id              TEXT                NOT NULL,
    departure_time          TIMESTAMPTZ         NOT NULL,
    recurrence_type         recurrence_type     NOT NULL,
    days_of_week            SMALLINT[]          NOT NULL DEFAULT '{}',
    start_date              DATE                NOT NULL,
    end_date                DATE                NOT NULL,
    total_seats             INTEGER             NOT NULL,
    price_per_seat          INTEGER             NOT NULL,
    allow_luggages          BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_pets              BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_food              BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_smoking           BOOLEAN             NOT NULL DEFAULT FALSE,
    auto_approve_enabled    BOOLEAN             NOT NULL DEFAULT FALSE,
    description             TEXT,
    generation_horizon_days INTEGER             NOT NULL DEFAULT 30,
    last_generated_date     DATE,
    is_active               BOOLEAN             NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ         NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_recurring_patterns_end_after_start
        CHECK (end_date >= start_date),
    CONSTRAINT ck_recurring_patterns_total_seats_positive
        CHECK (total_seats > 0),
    CONSTRAINT ck_recurring_patterns_price_positive
        CHECK (price_per_seat >= 0),
    CONSTRAINT ck_recurring_patterns_horizon_positive
        CHECK (generation_horizon_days > 0)
);

CREATE INDEX IF NOT EXISTS idx_recurring_patterns_driver_id
    ON recurring_patterns(driver_id);

CREATE INDEX IF NOT EXISTS idx_recurring_patterns_vehicle_id
    ON recurring_patterns(vehicle_id);

-- Seuls les patterns actifs sont interrogés en temps normal
CREATE INDEX IF NOT EXISTS idx_recurring_patterns_is_active
    ON recurring_patterns(is_active)
    WHERE is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_recurring_patterns_recurrence_type
    ON recurring_patterns(recurrence_type);

-- Recherche par plage de dates (trajets actifs dans une période donnée)
CREATE INDEX IF NOT EXISTS idx_recurring_patterns_date_range
    ON recurring_patterns(start_date, end_date);

-- Génération automatique : patterns actifs dont la dernière génération est ancienne
CREATE INDEX IF NOT EXISTS idx_recurring_patterns_generation
    ON recurring_patterns(last_generated_date, is_active)
    WHERE is_active = TRUE;

CREATE TRIGGER update_recurring_patterns_updated_at
    BEFORE UPDATE ON recurring_patterns
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : trips
-- Un trajet instancié (ponctuel ou issu d'un recurring_pattern).
-- =============================================================================
CREATE TABLE IF NOT EXISTS trips (
    trip_id                     TEXT                NOT NULL PRIMARY KEY,
    driver_id                   TEXT                NOT NULL,
    vehicle_id                  TEXT                NOT NULL,
    recurring_pattern_id        TEXT                REFERENCES recurring_patterns(trip_pattern_id) ON DELETE SET NULL,
    departure_datetime          TIMESTAMPTZ         NOT NULL,
    actual_departure_datetime   TIMESTAMPTZ,
    estimated_arrival_datetime  TIMESTAMPTZ         NOT NULL,
    actual_arrival_datetime     TIMESTAMPTZ,
    estimated_duration_minutes  INTEGER             NOT NULL,
    estimated_distance_meters   INTEGER             NOT NULL,
    total_seats                 SMALLINT            NOT NULL,
    available_seats             SMALLINT            NOT NULL,
    price_per_seat              INTEGER             NOT NULL,
    payment_methods_accepted    payment_method[]    NOT NULL DEFAULT '{}',
    allow_luggages              BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_pets                  BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_food                  BOOLEAN             NOT NULL DEFAULT FALSE,
    allow_smoking               BOOLEAN             NOT NULL DEFAULT FALSE,
    status                      trip_status         NOT NULL DEFAULT 'scheduled',
    auto_approve_enabled        BOOLEAN             NOT NULL DEFAULT FALSE,
    canceller_id                TEXT,
    cancellation_reason         TEXT,
    description                 TEXT,
    created_at                  TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ         NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_trips_available_seats
        CHECK (available_seats >= 0 AND available_seats <= total_seats),
    CONSTRAINT ck_trips_total_seats_positive
        CHECK (total_seats > 0),
    CONSTRAINT ck_trips_price_positive
        CHECK (price_per_seat >= 0),
    CONSTRAINT ck_trips_duration_positive
        CHECK (estimated_duration_minutes > 0),
    CONSTRAINT ck_trips_distance_positive
        CHECK (estimated_distance_meters > 0),
    CONSTRAINT ck_trips_arrival_after_departure
        CHECK (estimated_arrival_datetime > departure_datetime),
    -- canceller_id et cancellation_reason requis si annulé
    CONSTRAINT ck_trips_cancellation
        CHECK (
            status != 'cancelled'
            OR (canceller_id IS NOT NULL AND cancellation_reason IS NOT NULL)
        )
);

CREATE INDEX IF NOT EXISTS idx_trips_driver_id
    ON trips(driver_id);

CREATE INDEX IF NOT EXISTS idx_trips_vehicle_id
    ON trips(vehicle_id);

CREATE INDEX IF NOT EXISTS idx_trips_recurring_pattern_id
    ON trips(recurring_pattern_id)
    WHERE recurring_pattern_id IS NOT NULL;

-- Requête principale : trajets disponibles par date de départ
CREATE INDEX IF NOT EXISTS idx_trips_status_departure
    ON trips(status, departure_datetime);

-- Recherche de trajets avec places disponibles (partiel — exclut les pleins)
CREATE INDEX IF NOT EXISTS idx_trips_available_seats
    ON trips(available_seats, departure_datetime)
    WHERE available_seats > 0;

-- Tri chronologique par départ
CREATE INDEX IF NOT EXISTS idx_trips_departure_datetime
    ON trips(departure_datetime);

-- Recherche des trajets d'un driver par statut (dashboard conducteur)
CREATE INDEX IF NOT EXISTS idx_trips_driver_status
    ON trips(driver_id, status, departure_datetime);

CREATE TRIGGER update_trips_updated_at
    BEFORE UPDATE ON trips
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : trips_waypoints
-- Points de passage d'un trajet (départ, arrêts, arrivée).
-- PostGIS : colonne position GEOGRAPHY pour indexation spatiale.
-- =============================================================================
CREATE TABLE IF NOT EXISTS trips_waypoints (
    waypoint_id                         TEXT                    NOT NULL PRIMARY KEY,
    trip_id                             TEXT                    NOT NULL REFERENCES trips(trip_id) ON DELETE CASCADE,
    sequencer_order                     SMALLINT                NOT NULL,
    waypoint_type                       waypoint_type           NOT NULL,
    location_name                       TEXT                    NOT NULL,
    location_lng                        DECIMAL(10, 7)          NOT NULL,
    location_lat                        DECIMAL(10, 7)          NOT NULL,
    position                            GEOGRAPHY(POINT, 4326)  NOT NULL,
    city                                TEXT                    NOT NULL,
    country                             TEXT                    NOT NULL,
    scheduled_pickup_datetime           TIMESTAMPTZ,
    actual_scheduled_pickup_datetime    TIMESTAMPTZ,
    minutes_from_departure              INTEGER                 NOT NULL DEFAULT 0,
    price_from_previous                 INTEGER                 NOT NULL DEFAULT 0,
    created_at                          TIMESTAMPTZ             NOT NULL DEFAULT NOW(),
    updated_at                          TIMESTAMPTZ             NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_trips_waypoints_order_positive
        CHECK (sequencer_order >= 0),
    CONSTRAINT ck_trips_waypoints_minutes_positive
        CHECK (minutes_from_departure >= 0),
    CONSTRAINT ck_trips_waypoints_price_positive
        CHECK (price_from_previous >= 0),
    -- Un seul départ et une seule arrivée par trajet
    CONSTRAINT uq_trips_waypoints_type_per_trip
        UNIQUE (trip_id, waypoint_type) DEFERRABLE INITIALLY DEFERRED,
    -- Ordre de passage unique par trajet
    CONSTRAINT uq_trips_waypoints_sequence
        UNIQUE (trip_id, sequencer_order)
);

-- Récupération ordonnée de tous les waypoints d'un trajet
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_trip_sequence
    ON trips_waypoints(trip_id, sequencer_order);

-- Index spatial GiST — recherche géographique (ex: waypoints proches d'un point)
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_position
    ON trips_waypoints USING GIST(position);

-- Recherche combinée ville + type (ex: tous les départs depuis Lomé)
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_city_type
    ON trips_waypoints(city, waypoint_type);

-- Filtrage temporel sur les horaires de pickup
CREATE INDEX IF NOT EXISTS idx_trips_waypoints_scheduled_pickup
    ON trips_waypoints(scheduled_pickup_datetime)
    WHERE scheduled_pickup_datetime IS NOT NULL;

CREATE TRIGGER update_trips_waypoints_updated_at
    BEFORE UPDATE ON trips_waypoints
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Table : waypoint_recurring_patterns
-- Points de passage associés à un recurring_pattern.
-- Pas de colonne position/GEOGRAPHY — les coordonnées sont stockées brutes
-- et la géographie sera construite à la génération du trajet instancié.
-- =============================================================================
CREATE TABLE IF NOT EXISTS waypoint_recurring_patterns (
    pattern_waypoint_id     TEXT            NOT NULL PRIMARY KEY,
    trip_pattern_id         TEXT            NOT NULL REFERENCES recurring_patterns(trip_pattern_id) ON DELETE CASCADE,
    sequencer_order         SMALLINT        NOT NULL,
    waypoint_type           waypoint_type   NOT NULL,
    location_name           TEXT            NOT NULL,
    location_lng            DECIMAL(10, 7)  NOT NULL,
    location_lat            DECIMAL(10, 7)  NOT NULL,
    city                    TEXT            NOT NULL,
    country                 TEXT            NOT NULL,
    price_from_previous     INTEGER         NOT NULL DEFAULT 0,
    minutes_from_departure  INTEGER         NOT NULL DEFAULT 0,

    CONSTRAINT ck_waypoint_recurring_patterns_order_positive
        CHECK (sequencer_order >= 0),
    CONSTRAINT ck_waypoint_recurring_patterns_price_positive
        CHECK (price_from_previous >= 0),
    CONSTRAINT ck_waypoint_recurring_patterns_minutes_positive
        CHECK (minutes_from_departure >= 0),
    -- Un seul départ et une seule arrivée par pattern
    CONSTRAINT uq_waypoint_recurring_patterns_type_per_pattern
        UNIQUE (trip_pattern_id, waypoint_type) DEFERRABLE INITIALLY DEFERRED,
    -- Ordre unique par pattern
    CONSTRAINT uq_waypoint_recurring_patterns_sequence
        UNIQUE (trip_pattern_id, sequencer_order)
);

-- Récupération ordonnée des waypoints d'un pattern
CREATE INDEX IF NOT EXISTS idx_waypoint_recurring_patterns_sequence
    ON waypoint_recurring_patterns(trip_pattern_id, sequencer_order);

-- Recherche par ville + type (ex: tous les patterns avec départ depuis Lomé)
CREATE INDEX IF NOT EXISTS idx_waypoint_recurring_patterns_city_type
    ON waypoint_recurring_patterns(city, waypoint_type);
