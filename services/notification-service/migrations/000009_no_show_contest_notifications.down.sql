-- Restaure les textes NO_SHOW_AT_DEPARTURE et retire les événements de contestation.
UPDATE notification_templates
SET body = 'Votre absence au départ a été signalée.'
WHERE event_type = 'NO_SHOW_AT_DEPARTURE' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET body = 'Le conducteur a signalé votre absence au point de départ. Votre réservation a été annulée.',
    body_html = '<p>Bonjour,</p><p>Le conducteur a signalé votre absence au point de départ.</p><p>Votre réservation a été annulée.</p>'
WHERE event_type = 'NO_SHOW_AT_DEPARTURE' AND channel = 'email' AND language_code = 'fr';

DELETE FROM notification_templates
WHERE event_type IN ('DRIVER_NO_SHOW_REPORTED', 'NO_SHOW_CONTESTED', 'NO_SHOW_CONTEST_RESOLVED');

DELETE FROM notification_event_routing
WHERE event_type IN ('DRIVER_NO_SHOW_REPORTED', 'NO_SHOW_CONTESTED', 'NO_SHOW_CONTEST_RESOLVED');
