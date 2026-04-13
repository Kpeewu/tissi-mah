-- Migration 000004 : corrections routing et templates email manquants
-- Aligne la table notification_event_routing avec la spec des 24 événements.

-- =============================================================================
-- Corrections routing
-- =============================================================================

-- BOOKING_CANCELLED_BY_PASSENGER : spec email=false (migration 000003 avait true)
UPDATE notification_event_routing SET send_email = false WHERE event_type = 'BOOKING_CANCELLED_BY_PASSENGER';

-- WAYPOINT_CANCELED : spec email=false (migration 000003 avait true)
UPDATE notification_event_routing SET send_email = false WHERE event_type = 'WAYPOINT_CANCELED';

-- NO_SHOW_AT_DEPARTURE : spec email=true, inbox=true, priority=critical (migration 000003 avait email=false, inbox=true(?), priority=standard)
UPDATE notification_event_routing
SET send_email = true, create_inbox_entry = true, priority = 'critical'
WHERE event_type = 'NO_SHOW_AT_DEPARTURE';

-- DOCUMENT_VALIDATED + DOCUMENT_REJECTED : spec inbox=true (migration 000003 avait false)
UPDATE notification_event_routing SET create_inbox_entry = true WHERE event_type IN ('DOCUMENT_VALIDATED', 'DOCUMENT_REJECTED');

-- TRIP_DEPARTURE_REMINDER : spec inbox=false, priority=critical (migration 000003 avait inbox=true, priority=standard)
UPDATE notification_event_routing
SET create_inbox_entry = false, priority = 'critical'
WHERE event_type = 'TRIP_DEPARTURE_REMINDER';

-- PAYMENT_COMPLETED : spec priority=standard (migration 000003 avait critical)
UPDATE notification_event_routing SET priority = 'standard' WHERE event_type = 'PAYMENT_COMPLETED';

-- REFUND_PROCESSED : spec priority=critical (migration 000003 avait standard)
UPDATE notification_event_routing SET priority = 'critical' WHERE event_type = 'REFUND_PROCESSED';

-- =============================================================================
-- Templates email manquants
-- =============================================================================

-- TRIP_CANCELLED : routing (000001) a send_email=true mais aucun template email n'existait
INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html)
VALUES (
    'TRIP_CANCELLED', 'email', 'fr',
    'Trajet annulé',
    'Votre trajet a été annulé',
    'Le conducteur a annulé le trajet prévu. Toutes les réservations associées ont été annulées et un remboursement sera traité si applicable.',
    '<p>Bonjour,</p><p>Le conducteur a annulé le trajet prévu. Toutes les réservations associées ont été annulées.</p><p>Un remboursement sera traité si applicable.</p>'
) ON CONFLICT (event_type, channel, language_code) DO NOTHING;

-- NO_SHOW_AT_DEPARTURE : send_email corrigé à true ci-dessus, template manquant
INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html)
VALUES (
    'NO_SHOW_AT_DEPARTURE', 'email', 'fr',
    'Absence signalée',
    'Votre absence au départ a été signalée',
    'Le conducteur a signalé votre absence au point de départ. Votre réservation a été annulée.',
    '<p>Bonjour,</p><p>Le conducteur a signalé votre absence au point de départ.</p><p>Votre réservation a été annulée.</p>'
) ON CONFLICT (event_type, channel, language_code) DO NOTHING;
