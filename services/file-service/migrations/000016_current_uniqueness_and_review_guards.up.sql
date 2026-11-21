-- Migration 000016 : garde-fous d'unicité sur les décisions + nettoyage des doublons.
--
-- 1) Nettoyage ponctuel des doublons is_current (plusieurs documents courants pour
--    un même (propriétaire, type) étaient possibles).
-- 2) Une seule review 'completed' de première tentative par unité logique — les
--    tentatives suivantes (override, resoumission) chaînent previous_review_id.
--    Empêche la double validation concurrente d'un même document.
--
-- NB : aucun index unique sur is_current. Le remplacement d'un document crée la
-- nouvelle ligne (is_current = true) AVANT de marquer l'ancienne remplacée
-- (MarkAsReplaced a besoin du nouvel ID pour replaced_by, contrainte FK) : il
-- existe donc une fenêtre légitime à deux lignes courantes. L'unicité est tenue
-- côté applicatif (blocage à l'upload, MarkAsReplaced systématique) et les lectures
-- sont déterministes (ORDER BY uploaded_at DESC LIMIT 1).
--
-- Prérequis code (même release) : createPendingReview chaîne désormais
-- previous_review_id / attempt_number sur l'historique existant.

-- 1a) Dédoublonnage user_documents : garder le plus récent par (user_id, type)
UPDATE user_documents u
SET is_current = false
WHERE is_current
  AND EXISTS (
      SELECT 1 FROM user_documents n
      WHERE n.user_id = u.user_id
        AND n.document_type = u.document_type
        AND n.is_current
        AND (n.uploaded_at > u.uploaded_at
             OR (n.uploaded_at = u.uploaded_at AND n.document_id > u.document_id))
  );

-- 1b) Dédoublonnage vehicle_documents
UPDATE vehicle_documents v
SET is_current = false
WHERE is_current
  AND EXISTS (
      SELECT 1 FROM vehicle_documents n
      WHERE n.vehicle_id = v.vehicle_id
        AND n.document_type = v.document_type
        AND n.is_current
        AND (n.uploaded_at > v.uploaded_at
             OR (n.uploaded_at = v.uploaded_at AND n.document_id > v.document_id))
  );

-- 2a) Backfill du chaînage previous_review_id sur les reviews 'completed'
--     historiques non chaînées (ordre de création par unité logique).
WITH ordered AS (
    SELECT review_id,
           LAG(review_id) OVER (
               PARTITION BY user_id, logical_document_type, COALESCE(vehicle_document_id, '')
               ORDER BY COALESCE(created_at, updated_at), review_id
           ) AS prev_id,
           ROW_NUMBER() OVER (
               PARTITION BY user_id, logical_document_type, COALESCE(vehicle_document_id, '')
               ORDER BY COALESCE(created_at, updated_at), review_id
           ) AS rn
    FROM document_reviews
    WHERE status = 'completed'
)
UPDATE document_reviews dr
SET previous_review_id = o.prev_id
FROM ordered o
WHERE dr.review_id = o.review_id
  AND dr.previous_review_id IS NULL
  AND o.rn > 1;

-- 2b) Index uniques partiels : une seule review completed de 1re tentative
CREATE UNIQUE INDEX IF NOT EXISTS uq_reviews_completed_user
    ON document_reviews (user_id, logical_document_type)
    WHERE status = 'completed' AND previous_review_id IS NULL AND vehicle_document_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_reviews_completed_vehicle
    ON document_reviews (vehicle_document_id)
    WHERE status = 'completed' AND previous_review_id IS NULL;
