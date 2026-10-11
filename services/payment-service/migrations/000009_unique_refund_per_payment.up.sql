-- =============================================================================
-- Migration : un seul remboursement par paiement
-- =============================================================================

-- booking-service rejoue les demandes de remboursement jusqu'à acquittement : l'index unique
-- empêche deux demandes concurrentes de créer deux remboursements (et deux virements).
-- Prérequis : SELECT payment_id FROM refunds GROUP BY 1 HAVING count(*) > 1; doit être vide.
DROP INDEX IF EXISTS idx_refunds_payment_id;
CREATE UNIQUE INDEX IF NOT EXISTS uq_refunds_payment_id ON refunds(payment_id);
