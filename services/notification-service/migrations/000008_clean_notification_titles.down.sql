-- Restaure les titres décorés et l'ancienne formulation.
UPDATE notification_templates SET title = 'Identité vérifiée ✓' WHERE title = 'Identité vérifiée';
UPDATE notification_templates SET title = 'Document validé ✓' WHERE title = 'Document validé';
UPDATE notification_templates SET title = 'Profil conducteur vérifié ✓' WHERE title = 'Profil conducteur vérifié';
UPDATE notification_templates SET title = 'Bienvenue {{first_name}} ! 🎉' WHERE title = 'Bienvenue {{first_name}} !';

UPDATE notification_templates
SET body = 'Votre {{document_type}} a été validé.'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET body = 'Votre {{document_type}} a été {{status}}.'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'push' AND language_code = 'fr';
