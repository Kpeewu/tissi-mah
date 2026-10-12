-- =============================================================================
-- Migration 000009 : contestation et arbitrage des no-shows
-- =============================================================================

-- La partie accusée d'absence dispose de 48 h, à compter de la déclaration, pour la contester.
-- Pendant ce délai, et tant qu'une contestation n'est pas tranchée par le support, le remboursement
-- dû (refund_reason) n'est pas transmis à payment-service.
-- no_show_resolution : motif de remboursement retenu par le support (noShowDriver | noShowPassenger
-- | tripCancelled) ; no_show_resolved_by : UID de l'agent support.
ALTER TABLE bookings
    ADD COLUMN no_show_contested_at    TIMESTAMPTZ NULL,
    ADD COLUMN no_show_contested_by    VARCHAR(36) NULL,
    ADD COLUMN no_show_contest_reason  TEXT        NULL,
    ADD COLUMN no_show_resolved_at     TIMESTAMPTZ NULL,
    ADD COLUMN no_show_resolved_by     VARCHAR(64) NULL,
    ADD COLUMN no_show_resolution      VARCHAR(32) NULL,
    ADD COLUMN no_show_resolution_note TEXT        NULL;

-- File d'attente du support : contestations non tranchées
CREATE INDEX idx_bookings_no_show_contest_pending
    ON bookings (no_show_contested_at)
    WHERE no_show_contested_at IS NOT NULL AND no_show_resolved_at IS NULL;
