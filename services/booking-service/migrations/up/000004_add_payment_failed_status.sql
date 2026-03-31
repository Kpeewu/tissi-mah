-- Ajouter le statut paymentFailed à l'enum booking_status
ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'paymentFailed' AFTER 'paymentPending';
