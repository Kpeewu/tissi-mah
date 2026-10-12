-- ============================================================================
-- Migration 000009 : notifications de déclaration, contestation et décision de no-show
-- ============================================================================
-- La partie déclarée absente peut contester sous 48 h (booking-service) ; en cas de contestation,
-- le support tranche. Variables : {{contest_deadline}} (date limite, GMT), {{decision}} (libellé
-- de la décision du support).

INSERT INTO notification_event_routing (event_type, send_push, send_email, create_inbox_entry, priority) VALUES
    ('DRIVER_NO_SHOW_REPORTED',  true, true,  true, 'critical'),
    ('NO_SHOW_CONTESTED',        true, false, true, 'standard'),
    ('NO_SHOW_CONTEST_RESOLVED', true, true,  true, 'critical')
ON CONFLICT (event_type) DO NOTHING;

INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html) VALUES
('DRIVER_NO_SHOW_REPORTED', 'push', 'fr',
    'Absence signalée',
    '',
    'Un passager a signalé votre absence au point de départ. Vous pouvez contester jusqu''au {{contest_deadline}}.',
    ''
),
('DRIVER_NO_SHOW_REPORTED', 'email', 'fr',
    'Absence signalée',
    'Votre absence au départ a été signalée',
    'Un passager a signalé votre absence au point de départ. Si vous étiez présent, vous pouvez contester depuis l''application jusqu''au {{contest_deadline}}. Passé ce délai, le passager sera remboursé.',
    '<p>Bonjour,</p><p>Un passager a signalé votre absence au point de départ.</p><p>Si vous étiez présent, vous pouvez contester depuis l''application jusqu''au {{contest_deadline}}. Passé ce délai, le passager sera remboursé.</p>'
),
('NO_SHOW_CONTESTED', 'push', 'fr',
    'Absence contestée',
    '',
    'L''absence que vous avez signalée est contestée. Le support va examiner la réservation.',
    ''
),
('NO_SHOW_CONTEST_RESOLVED', 'push', 'fr',
    'Décision du support',
    '',
    'Contestation d''absence examinée : {{decision}}.',
    ''
),
('NO_SHOW_CONTEST_RESOLVED', 'email', 'fr',
    'Décision du support',
    'Décision du support sur une absence contestée',
    'Le support a examiné la contestation de l''absence signalée sur votre réservation : {{decision}}.',
    '<p>Bonjour,</p><p>Le support a examiné la contestation de l''absence signalée sur votre réservation : {{decision}}.</p>'
)
ON CONFLICT (event_type, channel, language_code) DO NOTHING;

-- NO_SHOW_AT_DEPARTURE : la réservation n'est plus « annulée » sans recours, le passager peut contester
UPDATE notification_templates
SET body = 'Le conducteur a signalé votre absence au point de départ. Vous pouvez contester jusqu''au {{contest_deadline}}.'
WHERE event_type = 'NO_SHOW_AT_DEPARTURE' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET body = 'Le conducteur a signalé votre absence au point de départ. Si vous étiez présent, vous pouvez contester depuis l''application jusqu''au {{contest_deadline}}.',
    body_html = '<p>Bonjour,</p><p>Le conducteur a signalé votre absence au point de départ.</p><p>Si vous étiez présent, vous pouvez contester depuis l''application jusqu''au {{contest_deadline}}.</p>'
WHERE event_type = 'NO_SHOW_AT_DEPARTURE' AND channel = 'email' AND language_code = 'fr';
