package implementations

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type userDocumentReadImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewUserDocumentReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.UserDocumentRepositoryRead {
	return &userDocumentReadImpl{pool: pool, logger: logger}
}

func (r *userDocumentReadImpl) GetByID(ctx context.Context, documentID string) (*domain.UserDocument, error) {
	r.logger.Debug("récupération du document utilisateur par ID", zap.String("documentID", documentID))

	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_key, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents WHERE document_id = $1`

	doc := &domain.UserDocument{}
	err := r.pool.QueryRow(ctx, query, documentID).Scan(
		&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentKey, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("document utilisateur non trouvé", zap.String("documentID", documentID))
			return nil, fileErrors.ErrorDocumentNotFound
		}
		r.logger.Error("erreur récupération document utilisateur par ID", zap.String("documentID", documentID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}

func (r *userDocumentReadImpl) GetByUserID(ctx context.Context, userID string) ([]*domain.UserDocument, error) {
	r.logger.Debug("récupération des documents utilisateur par userID", zap.String("userID", userID))

	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_key, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents WHERE user_id = $1
	          ORDER BY uploaded_at DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("erreur récupération documents utilisateur par userID", zap.String("userID", userID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var docs []*domain.UserDocument
	for rows.Next() {
		doc := &domain.UserDocument{}
		err := rows.Scan(
			&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
			&doc.DocumentKey, &doc.FileSizeBytes, &doc.MimeType,
			&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
			&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
			&doc.UploadedAt, &doc.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("erreur scan document utilisateur", zap.String("userID", userID), zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *userDocumentReadImpl) ListCurrentByStatuses(ctx context.Context, statuses []string, limit int32) ([]*domain.UserDocument, error) {
	r.logger.Debug("liste des documents utilisateur courants par statuts", zap.Strings("statuses", statuses))

	// profilePicture n'est pas un document de validation KYC.
	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_key, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents
	          WHERE is_current = true AND document_type <> 'profilePicture'`
	args := []interface{}{}
	argIdx := 1
	if len(statuses) > 0 {
		query += fmt.Sprintf(` AND status = ANY($%d)`, argIdx)
		args = append(args, statuses)
		argIdx++
	}
	query += ` ORDER BY updated_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("erreur liste documents utilisateur par statuts", zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var docs []*domain.UserDocument
	for rows.Next() {
		doc := &domain.UserDocument{}
		err := rows.Scan(
			&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
			&doc.DocumentKey, &doc.FileSizeBytes, &doc.MimeType,
			&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
			&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
			&doc.UploadedAt, &doc.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("erreur scan document utilisateur (ListCurrentByStatuses)", zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *userDocumentReadImpl) GetCurrentByUserIDAndType(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error) {
	r.logger.Debug("récupération du document courant par userID et type",
		zap.String("userID", userID),
		zap.String("documentType", documentType),
	)

	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_key, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents
	          WHERE user_id = $1 AND document_type = $2 AND is_current = true
	          ORDER BY uploaded_at DESC
	          LIMIT 1`

	doc := &domain.UserDocument{}
	err := r.pool.QueryRow(ctx, query, userID, documentType).Scan(
		&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentKey, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("document courant non trouvé", zap.String("userID", userID), zap.String("documentType", documentType))
			return nil, fileErrors.ErrorDocumentNotFound
		}
		r.logger.Error("erreur récupération document courant", zap.String("userID", userID), zap.String("documentType", documentType), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}
