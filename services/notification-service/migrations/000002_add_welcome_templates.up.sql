-- ============================================================================
-- Migration 000002: Add WELCOME notification templates
-- ============================================================================

INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html) VALUES
('WELCOME', 'push', 'fr',
    'Bienvenue {{first_name}} ! 🎉',
    '',
    'Votre compte TissiMah est prêt. Bon voyage !',
    ''
),
('WELCOME', 'email', 'fr',
    'Bienvenue sur TissiMah {{first_name}} !',
    'Bienvenue sur TissiMah',
    'Bonjour {{first_name}},

Votre compte a été créé avec succès.

Bonne route !
L''équipe TissiMah',
    '<p>Bonjour <strong>{{first_name}}</strong>,</p><p>Votre compte a été créé avec succès.</p><p>Bonne route !<br/>L''équipe TissiMah</p>'
)
ON CONFLICT (event_type, channel, language_code) DO NOTHING;
