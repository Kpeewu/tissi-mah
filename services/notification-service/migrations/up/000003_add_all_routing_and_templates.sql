-- ============================================================================
-- Migration 000003: Align routing with publisher constants + add all templates
-- ============================================================================

-- Partie A: Supprimer les routing obsolètes (noms désalignés avec publisher.go)
DELETE FROM notification_event_routing WHERE event_type IN (
    'BOOKING_CANCELLED',
    'PAYMENT_RECEIVED',
    'TRIP_COMPLETED',
    'NEW_RATING',
    'PAYOUT_COMPLETED',
    'PAYOUT_FAILED',
    'TRIP_REMINDER'
);

-- Partie B: Ajouter les routing manquants ou corrigés
INSERT INTO notification_event_routing (event_type, send_push, send_email, create_inbox_entry, priority) VALUES
    ('BOOKING_CANCELLED_BY_DRIVER',     true,  true,  true,  'critical'),
    ('BOOKING_CANCELLED_BY_PASSENGER',  true,  true,  true,  'critical'),
    ('PAYMENT_COMPLETED',               true,  true,  true,  'critical'),
    ('TRIP_ENDED',                      true,  false, true,  'standard'),
    ('RATING_RECEIVED',                 true,  false, true,  'low'),
    ('DRIVER_PAYMENT_LAUNCHED',         true,  true,  true,  'standard'),
    ('TRIP_DEPARTURE_REMINDER',         true,  false, true,  'standard'),
    ('WAYPOINT_CANCELED',               true,  true,  true,  'critical'),
    ('DOCUMENT_VALIDATED',              true,  true,  false, 'standard'),
    ('DOCUMENT_REJECTED',               true,  true,  false, 'standard'),
    ('REFUND_PROCESSED',                true,  true,  true,  'standard'),
    ('DRIVER_PROFILE_VERIFIED',         true,  true,  false, 'standard'),
    ('NO_SHOW_AT_DEPARTURE',            true,  false, true,  'standard')
ON CONFLICT (event_type) DO NOTHING;

-- NEW_BOOKING_REQUEST est déjà seedé en 000001 avec send_push=true, send_email=false, inbox=true
-- On le met à jour pour s'assurer de la cohérence
UPDATE notification_event_routing
SET send_push = true, send_email = false, create_inbox_entry = true, priority = 'critical'
WHERE event_type = 'NEW_BOOKING_REQUEST';

-- ============================================================================
-- Partie C: Templates push (fr)
-- ============================================================================

INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html) VALUES

-- Booking events
('NEW_BOOKING_REQUEST', 'push', 'fr',
    'Nouvelle demande de réservation',
    '',
    'Un passager souhaite réserver {{seats}} place(s) sur votre trajet.',
    ''
),
('BOOKING_CONFIRMED', 'push', 'fr',
    'Réservation confirmée !',
    '',
    'Votre réservation a été acceptée par le conducteur. Bon voyage !',
    ''
),
('BOOKING_REJECTED', 'push', 'fr',
    'Réservation refusée',
    '',
    'Votre demande de réservation n''a pas été acceptée.',
    ''
),
('BOOKING_CANCELLED_BY_DRIVER', 'push', 'fr',
    'Réservation annulée',
    '',
    'Votre réservation a été annulée par le conducteur.',
    ''
),
('BOOKING_CANCELLED_BY_PASSENGER', 'push', 'fr',
    'Réservation annulée',
    '',
    'Un passager a annulé sa réservation sur votre trajet.',
    ''
),

-- Trip lifecycle
('TRIP_STARTED', 'push', 'fr',
    'Votre trajet commence !',
    '',
    'Le conducteur a démarré le trajet. Bon voyage !',
    ''
),
('TRIP_ENDED', 'push', 'fr',
    'Trajet terminé',
    '',
    'Votre trajet est terminé. N''oubliez pas de noter votre expérience !',
    ''
),
('TRIP_CANCELLED', 'push', 'fr',
    'Trajet annulé',
    '',
    'Votre trajet a été annulé par le conducteur.',
    ''
),
('TRIP_MODIFIED', 'push', 'fr',
    'Trajet modifié',
    '',
    'Les horaires de votre trajet ont été modifiés.',
    ''
),
('WAYPOINT_CANCELED', 'push', 'fr',
    'Arrêt supprimé',
    '',
    'Un arrêt de votre trajet a été supprimé par le conducteur.',
    ''
),
('TRIP_DEPARTURE_REMINDER', 'push', 'fr',
    'Rappel de départ',
    '',
    'Votre trajet part bientôt. Soyez prêt(e) !',
    ''
),

-- Payment events
('PAYMENT_COMPLETED', 'push', 'fr',
    'Paiement confirmé',
    '',
    'Votre paiement de {{amount}} XOF a bien été reçu.',
    ''
),
('PAYMENT_FAILED', 'push', 'fr',
    'Échec du paiement',
    '',
    'Votre paiement n''a pas pu être traité. Veuillez réessayer.',
    ''
),
('REFUND_PROCESSED', 'push', 'fr',
    'Remboursement en cours',
    '',
    'Votre remboursement de {{amount}} XOF est en cours de traitement.',
    ''
),
('DRIVER_PAYMENT_LAUNCHED', 'push', 'fr',
    'Virement initié',
    '',
    'Un virement de {{amount}} XOF a été lancé vers votre compte.',
    ''
),

-- Rating
('RATING_RECEIVED', 'push', 'fr',
    'Nouvelle évaluation',
    '',
    'Vous avez reçu une note de {{stars}}/5.',
    ''
),

-- KYC / Documents
('KYC_APPROVED', 'push', 'fr',
    'Identité vérifiée ✓',
    '',
    'Votre vérification d''identité a été approuvée avec succès.',
    ''
),
('KYC_REJECTED', 'push', 'fr',
    'Vérification refusée',
    '',
    'Votre vérification d''identité a été rejetée. Contactez le support.',
    ''
),
('DOCUMENT_VALIDATED', 'push', 'fr',
    'Document validé ✓',
    '',
    'Votre document a été validé avec succès.',
    ''
),
('DOCUMENT_REJECTED', 'push', 'fr',
    'Document refusé',
    '',
    'Votre document a été refusé. Veuillez en soumettre un nouveau.',
    ''
),
('DOCUMENT_EXPIRING_SOON', 'push', 'fr',
    'Document expirant bientôt',
    '',
    'Un de vos documents expire bientôt. Pensez à le renouveler.',
    ''
),
('DRIVER_PROFILE_VERIFIED', 'push', 'fr',
    'Profil conducteur vérifié ✓',
    '',
    'Votre profil conducteur a été validé. Vous pouvez maintenant créer des trajets !',
    ''
),

-- Other
('ACCOUNT_SUSPENDED', 'push', 'fr',
    'Compte suspendu',
    '',
    'Votre compte a été suspendu. Contactez le support pour plus d''informations.',
    ''
),
('NO_SHOW_AT_DEPARTURE', 'push', 'fr',
    'Absence signalée',
    '',
    'Votre absence au départ a été signalée.',
    ''
)

ON CONFLICT (event_type, channel, language_code) DO NOTHING;

-- ============================================================================
-- Partie D: Templates email (fr) — uniquement pour send_email=true
-- ============================================================================

INSERT INTO notification_templates (event_type, channel, language_code, title, subject, body, body_html) VALUES

('BOOKING_CANCELLED_BY_DRIVER', 'email', 'fr',
    'Votre réservation a été annulée',
    'Annulation de votre réservation TissiMah',
    'Bonjour,\n\nVotre réservation a été annulée par le conducteur.\n\nSi un paiement a été effectué, un remboursement sera traité dans les prochains jours.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre réservation a été annulée par le conducteur.</p><p>Si un paiement a été effectué, un remboursement sera traité dans les prochains jours.</p><p>L''équipe TissiMah</p>'
),
('BOOKING_CANCELLED_BY_PASSENGER', 'email', 'fr',
    'Un passager a annulé sa réservation',
    'Annulation de réservation - TissiMah',
    'Bonjour,\n\nUn passager a annulé sa réservation sur votre trajet.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Un passager a annulé sa réservation sur votre trajet.</p><p>L''équipe TissiMah</p>'
),
('BOOKING_REJECTED', 'email', 'fr',
    'Votre réservation a été refusée',
    'Réservation refusée - TissiMah',
    'Bonjour,\n\nVotre demande de réservation n''a pas été acceptée par le conducteur.\n\nVous pouvez rechercher un autre trajet sur TissiMah.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre demande de réservation n''a pas été acceptée par le conducteur.</p><p>Vous pouvez rechercher un autre trajet sur TissiMah.</p><p>L''équipe TissiMah</p>'
),
('BOOKING_CONFIRMED', 'email', 'fr',
    'Votre réservation est confirmée !',
    'Confirmation de réservation - TissiMah',
    'Bonjour,\n\nVotre réservation a été confirmée par le conducteur. Bon voyage !\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre réservation a été confirmée par le conducteur. Bon voyage !</p><p>L''équipe TissiMah</p>'
),
('WAYPOINT_CANCELED', 'email', 'fr',
    'Votre arrêt a été supprimé',
    'Modification de trajet - TissiMah',
    'Bonjour,\n\nUn arrêt de votre trajet a été supprimé par le conducteur.\n\nSi cela affecte votre voyage, un remboursement sera traité.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Un arrêt de votre trajet a été supprimé par le conducteur.</p><p>Si cela affecte votre voyage, un remboursement sera traité.</p><p>L''équipe TissiMah</p>'
),
('PAYMENT_COMPLETED', 'email', 'fr',
    'Paiement confirmé',
    'Confirmation de paiement - TissiMah',
    'Bonjour,\n\nVotre paiement de {{amount}} XOF a bien été reçu et votre réservation est confirmée.\n\nMerci de voyager avec TissiMah !\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre paiement de <strong>{{amount}} XOF</strong> a bien été reçu et votre réservation est confirmée.</p><p>Merci de voyager avec TissiMah !</p><p>L''équipe TissiMah</p>'
),
('PAYMENT_FAILED', 'email', 'fr',
    'Échec du paiement',
    'Problème de paiement - TissiMah',
    'Bonjour,\n\nVotre paiement n''a pas pu être traité. Veuillez vérifier vos informations et réessayer.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre paiement n''a pas pu être traité. Veuillez vérifier vos informations et réessayer.</p><p>L''équipe TissiMah</p>'
),
('REFUND_PROCESSED', 'email', 'fr',
    'Remboursement en cours',
    'Remboursement - TissiMah',
    'Bonjour,\n\nVotre remboursement de {{amount}} XOF est en cours de traitement. Il sera crédité sur votre compte sous 3 à 5 jours ouvrés.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre remboursement de <strong>{{amount}} XOF</strong> est en cours de traitement. Il sera crédité sur votre compte sous 3 à 5 jours ouvrés.</p><p>L''équipe TissiMah</p>'
),
('DRIVER_PAYMENT_LAUNCHED', 'email', 'fr',
    'Virement initié',
    'Virement en cours - TissiMah',
    'Bonjour,\n\nUn virement de {{amount}} XOF a été initié vers votre compte. Il sera disponible sous 1 à 2 jours ouvrés.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Un virement de <strong>{{amount}} XOF</strong> a été initié vers votre compte. Il sera disponible sous 1 à 2 jours ouvrés.</p><p>L''équipe TissiMah</p>'
),
('KYC_APPROVED', 'email', 'fr',
    'Votre identité a été vérifiée',
    'Vérification d''identité approuvée - TissiMah',
    'Bonjour,\n\nVotre vérification d''identité a été approuvée avec succès. Vous pouvez maintenant utiliser toutes les fonctionnalités de TissiMah.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre vérification d''identité a été <strong>approuvée</strong> avec succès.</p><p>Vous pouvez maintenant utiliser toutes les fonctionnalités de TissiMah.</p><p>L''équipe TissiMah</p>'
),
('KYC_REJECTED', 'email', 'fr',
    'Vérification d''identité rejetée',
    'Vérification d''identité refusée - TissiMah',
    'Bonjour,\n\nVotre vérification d''identité a été rejetée. Veuillez contacter notre support pour plus d''informations.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre vérification d''identité a été <strong>rejetée</strong>.</p><p>Veuillez contacter notre support pour plus d''informations.</p><p>L''équipe TissiMah</p>'
),
('DOCUMENT_VALIDATED', 'email', 'fr',
    'Document validé',
    'Document validé - TissiMah',
    'Bonjour,\n\nVotre document a été validé avec succès.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre document a été <strong>validé</strong> avec succès.</p><p>L''équipe TissiMah</p>'
),
('DOCUMENT_REJECTED', 'email', 'fr',
    'Document refusé',
    'Document refusé - TissiMah',
    'Bonjour,\n\nVotre document a été refusé. Veuillez en soumettre un nouveau conforme aux exigences.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre document a été <strong>refusé</strong>. Veuillez en soumettre un nouveau conforme aux exigences.</p><p>L''équipe TissiMah</p>'
),
('DOCUMENT_EXPIRING_SOON', 'email', 'fr',
    'Document expirant bientôt',
    'Renouvellement de document requis - TissiMah',
    'Bonjour,\n\nUn de vos documents expire bientôt. Pensez à le renouveler pour continuer à utiliser TissiMah.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Un de vos documents expire bientôt. Pensez à le renouveler pour continuer à utiliser TissiMah.</p><p>L''équipe TissiMah</p>'
),
('DRIVER_PROFILE_VERIFIED', 'email', 'fr',
    'Profil conducteur validé',
    'Bienvenue en tant que conducteur TissiMah !',
    'Bonjour,\n\nVotre profil conducteur a été validé. Vous pouvez maintenant créer des trajets et transporter des passagers sur TissiMah.\n\nBonne route !\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre profil conducteur a été <strong>validé</strong>.</p><p>Vous pouvez maintenant créer des trajets et transporter des passagers sur TissiMah.</p><p>Bonne route !<br/>L''équipe TissiMah</p>'
),
('ACCOUNT_SUSPENDED', 'email', 'fr',
    'Votre compte a été suspendu',
    'Compte suspendu - TissiMah',
    'Bonjour,\n\nVotre compte TissiMah a été suspendu. Veuillez contacter notre support pour plus d''informations.\n\nL''équipe TissiMah',
    '<p>Bonjour,</p><p>Votre compte TissiMah a été suspendu.</p><p>Veuillez contacter notre support pour plus d''informations.</p><p>L''équipe TissiMah</p>'
)

ON CONFLICT (event_type, channel, language_code) DO NOTHING;
