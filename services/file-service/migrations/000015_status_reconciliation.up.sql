-- Migration 000015 : réparation de la désynchronisation statut document / review.
--
-- Bug : UpdateDocumentReview ne synchronisait pas user_documents.status /
-- vehicle_documents.status. Comme chaque upload pré-crée une review 'pending'
-- que ValidateDocument met ensuite à jour (branche Update), les décisions
-- support laissaient les documents bloqués en 'pending' :
--   - utilisateurs approuvés jamais sortis de la file support,
--   - documents rejetés impossibles à resoumettre.
-- On recale ici le statut des documents encore 'pending' sur la dernière
-- décision 'completed' qui les concerne.

-- user_documents : review directe (user_document_id) ou via la 2e face
WITH latest AS (
    SELECT DISTINCT ON (doc_id) doc_id, decision
    FROM (
        SELECT user_document_id AS doc_id, decision,
               COALESCE(reviewed_at, updated_at) AS decided_at
        FROM document_reviews
        WHERE status = 'completed'
          AND decision IN ('approved', 'rejected', 'resubmission')
          AND user_document_id IS NOT NULL
        UNION ALL
        SELECT second_user_document_id, decision,
               COALESCE(reviewed_at, updated_at)
        FROM document_reviews
        WHERE status = 'completed'
          AND decision IN ('approved', 'rejected', 'resubmission')
          AND second_user_document_id IS NOT NULL
    ) t
    ORDER BY doc_id, decided_at DESC
)
UPDATE user_documents ud
SET status = CASE latest.decision WHEN 'approved' THEN 'approved' ELSE 'rejected' END
FROM latest
WHERE ud.document_id = latest.doc_id
  AND ud.status = 'pending';

-- vehicle_documents : review via vehicle_document_id
WITH latest AS (
    SELECT DISTINCT ON (vehicle_document_id) vehicle_document_id, decision
    FROM document_reviews
    WHERE status = 'completed'
      AND decision IN ('approved', 'rejected', 'resubmission')
      AND vehicle_document_id IS NOT NULL
    ORDER BY vehicle_document_id, COALESCE(reviewed_at, updated_at) DESC
)
UPDATE vehicle_documents vd
SET status = CASE latest.decision WHEN 'approved' THEN 'approved' ELSE 'rejected' END
FROM latest
WHERE vd.document_id = latest.vehicle_document_id
  AND vd.status = 'pending';
