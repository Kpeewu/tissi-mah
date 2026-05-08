-- Template push pour les messages chat reçus.
-- L'événement NEW_MESSAGE est publié par chat-service après chaque
-- SendMessage réussi (cf. services/chat-service/internal/service/chat_service_impl.go).
-- Le payload contient {thread_id, sender_role, has_redaction} mais PAS le
-- contenu en clair (privacy + chiffrement AES-GCM serveur).
-- Le mobile lit data.thread_id pour faire un deep link vers la conversation.
--
-- Le routing send_push=true / send_email=false / create_inbox=true est déjà
-- défini en 000001_create_notification_tables.up.sql.

INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html) VALUES
('NEW_MESSAGE', 'push', 'fr',
    'Nouveau message',
    '',
    'Vous avez reçu un message. Ouvrez l''application pour le lire.',
    ''
),
('NEW_MESSAGE', 'push', 'en',
    'New message',
    '',
    'You have received a new message. Open the app to read it.',
    ''
),
('NEW_MESSAGE', 'inbox', 'fr',
    'Nouveau message',
    '',
    'Vous avez reçu un message dans votre conversation.',
    ''
),
('NEW_MESSAGE', 'inbox', 'en',
    'New message',
    '',
    'You have received a message in your conversation.',
    ''
)
ON CONFLICT (event_type, channel, language_code) DO NOTHING;
