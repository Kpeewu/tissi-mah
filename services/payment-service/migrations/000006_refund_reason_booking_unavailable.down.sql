-- =============================================================================
-- Rollback migration 000006
-- =============================================================================
-- Note : la valeur 'bookingUnavailable' de l'enum refund_reason n'est pas retirée
-- (PostgreSQL ne supporte pas DROP VALUE sur un enum).
SELECT 1;
