-- chat_threads : une conversation par réservation approuvée.
-- Le thread se ferme automatiquement quand le trajet se termine.
--
-- IMPORTANT : tous les IDs (thread_id, message_id) sont générés côté
-- application (uuid.New().String() en Go) et envoyés à la DB. Pas de
-- DEFAULT gen_random_uuid() : si un INSERT oublie l'ID, on veut un échec
-- explicite plutôt qu'une UUID auto-générée silencieusement.
--
-- driver_id et passenger_id sont les UserIDs internes retournés par
-- booking-service (et par user-service.GetUserByFirebaseID), JAMAIS le
-- Firebase UID. Cohérent avec sender_id côté chat_messages.
CREATE TABLE IF NOT EXISTS chat_threads (
    thread_id    UUID        PRIMARY KEY,
    booking_id   VARCHAR(36) UNIQUE NOT NULL,
    trip_id      VARCHAR(36) NOT NULL,
    driver_id    VARCHAR(36) NOT NULL,  -- UserID interne (pas Firebase UID)
    passenger_id VARCHAR(36) NOT NULL,  -- UserID interne (pas Firebase UID)
    status       VARCHAR(20) NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'closed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at    TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_threads_driver_id     ON chat_threads(driver_id);
CREATE INDEX IF NOT EXISTS idx_chat_threads_passenger_id  ON chat_threads(passenger_id);
CREATE INDEX IF NOT EXISTS idx_chat_threads_trip_id       ON chat_threads(trip_id);
CREATE INDEX IF NOT EXISTS idx_chat_threads_status        ON chat_threads(status);
CREATE INDEX IF NOT EXISTS idx_chat_threads_updated_at    ON chat_threads(updated_at DESC);

-- chat_messages : messages chiffrés AES-256-GCM.
-- Le contenu en clair n'est JAMAIS stocké.
-- has_redaction = true : du contenu PII a été masqué (★★★) dans le clair
-- qui a été chiffré. Le support peut déchiffrer les messages flagged uniquement.
--
-- sender_id et flagged_by sont des UserIDs internes — résolus depuis le
-- Firebase UID du caller via user-service.GetUserByFirebaseID au moment
-- de l'opération. Comparaisons d'autorisation toujours sur l'UserID interne.
CREATE TABLE IF NOT EXISTS chat_messages (
    message_id         UUID        PRIMARY KEY,
    thread_id          UUID        NOT NULL REFERENCES chat_threads(thread_id) ON DELETE CASCADE,
    sender_id          VARCHAR(36) NOT NULL,  -- UserID interne (pas Firebase UID)
    sender_role        VARCHAR(20) NOT NULL CHECK (sender_role IN ('driver', 'passenger')),
    content_encrypted  BYTEA       NOT NULL,  -- AES-256-GCM ciphertext
    content_nonce      BYTEA       NOT NULL,  -- 12 bytes GCM nonce
    has_redaction      BOOLEAN     NOT NULL DEFAULT FALSE,
    flagged            BOOLEAN     NOT NULL DEFAULT FALSE,
    flagged_by         VARCHAR(36),           -- UserID interne du signaleur
    flagged_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at            TIMESTAMPTZ            -- NULL = non lu
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_thread_id   ON chat_messages(thread_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_sender_id   ON chat_messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_flagged     ON chat_messages(flagged) WHERE flagged = TRUE;
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at  ON chat_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_unread
    ON chat_messages(thread_id, sender_id, read_at)
    WHERE read_at IS NULL;
