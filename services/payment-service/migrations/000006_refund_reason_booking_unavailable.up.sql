-- =============================================================================
-- Migration : motif de remboursement 'bookingUnavailable'
-- =============================================================================

-- Remboursement intégral quand FedaPay approuve un paiement dont la réservation
-- n'est plus confirmable (annulée, rejetée, etc.).
ALTER TYPE refund_reason ADD VALUE IF NOT EXISTS 'bookingUnavailable';
