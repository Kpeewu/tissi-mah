-- Restaure les textes génériques des templates DOCUMENT_VALIDATED / DOCUMENT_REJECTED
-- (push + email, fr) tels que définis en 000003_add_all_routing_and_templates.up.sql.

UPDATE notification_templates
SET title = 'Document validé ✓',
    body  = 'Votre document a été validé avec succès.'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET title = 'Document refusé',
    body  = 'Votre document a été refusé. Veuillez en soumettre un nouveau.'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET subject   = 'Document validé - TissiMah',
    body      = 'Bonjour,\n\nVotre document a été validé avec succès.\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>Votre document a été <strong>validé</strong> avec succès.</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'email' AND language_code = 'fr';

UPDATE notification_templates
SET subject   = 'Document refusé - TissiMah',
    body      = 'Bonjour,\n\nVotre document a été refusé. Veuillez en soumettre un nouveau conforme aux exigences.\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>Votre document a été <strong>refusé</strong>. Veuillez en soumettre un nouveau conforme aux exigences.</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'email' AND language_code = 'fr';
