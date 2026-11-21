-- =============================================================================
-- Migration : motif de remboursement 'waypointCancelled'
-- =============================================================================

-- Remboursement intégral quand le chauffeur supprime l'arrêt de prise en charge
-- ou de dépose d'un passager (annulation groupée par booking-service).
ALTER TYPE refund_reason ADD VALUE IF NOT EXISTS 'waypointCancelled';
