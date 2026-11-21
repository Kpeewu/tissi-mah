-- Enrichit les templates DOCUMENT_VALIDATED / DOCUMENT_REJECTED (push + email, fr)
-- pour inclure le type de document, le statut de la revue et — en cas de rejet —
-- le motif. Les variables {{document_type}}, {{status}} et {{reason}} sont fournies
-- dans le payload de l'événement par kyc-service (cf. publishDocumentReviewNotification).

-- Push : type + statut
UPDATE notification_templates
SET title = 'Document validé ✓',
    body  = 'Votre {{document_type}} a été validé.'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'push' AND language_code = 'fr';

UPDATE notification_templates
SET title = 'Document refusé',
    body  = 'Votre {{document_type}} a été {{status}}.'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'push' AND language_code = 'fr';

-- Email validé : type + statut
UPDATE notification_templates
SET subject   = 'Document validé - TissiMah',
    body      = 'Bonjour,\n\nVotre {{document_type}} a été validé avec succès.\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>Votre {{document_type}} a été <strong>validé</strong> avec succès.</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_VALIDATED' AND channel = 'email' AND language_code = 'fr';

-- Email refusé : type + statut + motif
UPDATE notification_templates
SET subject   = 'Document refusé - TissiMah',
    body      = 'Bonjour,\n\nVotre {{document_type}} a été {{status}}.\n\nMotif : {{reason}}\n\nL''équipe TissiMah',
    body_html = '<p>Bonjour,</p><p>Votre {{document_type}} a été <strong>{{status}}</strong>.</p><p>Motif : {{reason}}</p><p>L''équipe TissiMah</p>'
WHERE event_type = 'DOCUMENT_REJECTED' AND channel = 'email' AND language_code = 'fr';
