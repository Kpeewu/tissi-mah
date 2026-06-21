-- Demande de réinitialisation de mot de passe par un agent support.
-- NULL = pas de demande en attente ; NON-NULL = horodatage de la demande,
-- listée au dashboard admin jusqu'à ce qu'un admin lance la réinitialisation.
ALTER TABLE support_users ADD COLUMN IF NOT EXISTS password_reset_requested_at TIMESTAMPTZ;
