-- Retire les caractères décoratifs (✓, 🎉) des titres de notifications : l'interface
-- affiche déjà une icône vectorielle par catégorie, et ces caractères ne se lisent pas
-- correctement selon les polices et les lecteurs d'écran.
UPDATE notification_templates
SET title = 'Identité vérifiée'
WHERE title = 'Identité vérifiée ✓';

UPDATE notification_templates
SET title = 'Document validé'
WHERE title = 'Document validé ✓';

UPDATE notification_templates
SET title = 'Profil conducteur vérifié'
WHERE title = 'Profil conducteur vérifié ✓';

UPDATE notification_templates
SET title = 'Bienvenue {{first_name}} !'
WHERE title = 'Bienvenue {{first_name}} ! 🎉';

-- Accord grammatical : {{document_type}} peut être masculin (« Permis de conduire »)
-- ou féminin (« Carte grise »), si bien que « a été validé » était faux une fois sur
-- deux. La tournure est reformulée pour rester correcte dans les deux cas.
UPDATE notification_templates
SET body = '{{document_type}} : votre document a été validé.'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET body = '{{document_type}} : votre document est {{status}}.'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET body      = 'Bonjour,\n\n{{document_type}} : votre document a été validé.\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>{{document_type}} : votre document a été <strong>validé</strong>.</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'email' AND language_code = 'fr';

UPDATE notification_templates
SET body      = 'Bonjour,\n\n{{document_type}} : votre document est {{status}}.\n\nMotif : {{reason}}\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>{{document_type}} : votre document est <strong>{{status}}</strong>.</p><p>Motif : {{reason}}</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'email' AND language_code = 'fr';
