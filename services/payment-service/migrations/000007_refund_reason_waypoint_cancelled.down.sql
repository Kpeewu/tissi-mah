-- =============================================================================
-- Rollback migration 000007
-- =============================================================================
-- Note : la valeur 'waypointCancelled' de l'enum refund_reason n'est pas retirée
-- (PostgreSQL ne supporte pas DROP VALUE sur un enum).
SELECT 1;
