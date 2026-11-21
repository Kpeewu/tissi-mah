-- ============================================================================
-- Migration 000007: Purge des lignes keyées sur Firebase UID
-- ============================================================================
-- Avant ce correctif, le chemin client-facing (handlers → service) écrivait
-- device tokens et préférences sous le Firebase UID brut, alors que le dispatcher
-- lit/écrit sous l'UserID interne (UUID). Résultat : push jamais délivrés et inbox
-- vide. Le service résout désormais firebase_uid → UserID interne avant tout accès
-- repo, donc les lignes existantes keyées sur Firebase UID sont orphelines.
--
-- On les supprime (celles dont user_id n'est PAS un UUID). Auto-guérison :
--   - device tokens : RegisterDeviceToken fait un ON CONFLICT (fcm_token) DO UPDATE
--     SET user_id = …, donc la prochaine ré-inscription (ouverture de l'app) recrée
--     la ligne sous l'UserID interne.
--   - préférences : repassent au défaut (elles n'étaient de toute façon jamais
--     honorées par le dispatcher).
-- L'inbox (notification_inbox) est déjà keyée sur l'UserID interne — on n'y touche pas.

DELETE FROM user_device_tokens
WHERE user_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

DELETE FROM user_notification_preferences
WHERE user_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
