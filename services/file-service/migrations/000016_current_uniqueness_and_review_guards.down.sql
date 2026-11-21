DROP INDEX IF EXISTS uq_reviews_completed_vehicle;
DROP INDEX IF EXISTS uq_reviews_completed_user;
-- Le dédoublonnage is_current et le chaînage previous_review_id sont des
-- réparations de données : pas de retour arrière.
