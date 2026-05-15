-- persona_session_token était VARCHAR(255), insuffisant pour les JWT Persona (400-1000+ chars)
ALTER TABLE document_reviews
    ALTER COLUMN persona_session_token TYPE TEXT;
