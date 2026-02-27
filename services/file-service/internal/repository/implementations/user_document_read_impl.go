package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userDocumentReadImpl struct {
	pool *pgxpool.Pool
}

func NewUserDocumentReadRepository(pool *pgxpool.Pool) i.UserDocumentRepositoryRead {
	return &userDocumentReadImpl{pool: pool}
}

func (r *userDocumentReadImpl) GetByID(ctx context.Context, documentID string) (*domain.UserDocument, error) {
	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents WHERE document_id = $1`

	doc := &domain.UserDocument{}
	err := r.pool.QueryRow(ctx, query, documentID).Scan(
		&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fileErrors.ErrorDocumentNotFound
		}
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}

func (r *userDocumentReadImpl) GetByUserID(ctx context.Context, userID string) ([]*domain.UserDocument, error) {
	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents WHERE user_id = $1
	          ORDER BY uploaded_at DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var docs []*domain.UserDocument
	for rows.Next() {
		doc := &domain.UserDocument{}
		err := rows.Scan(
			&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
			&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
			&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
			&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
			&doc.UploadedAt, &doc.UpdatedAt,
		)
		if err != nil {
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *userDocumentReadImpl) GetCurrentByUserIDAndType(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error) {
	query := `SELECT document_id, user_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_country,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM user_documents
	          WHERE user_id = $1 AND document_type = $2 AND is_current = true`

	doc := &domain.UserDocument{}
	err := r.pool.QueryRow(ctx, query, userID, documentType).Scan(
		&doc.DocumentID, &doc.UserID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingCountry,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fileErrors.ErrorDocumentNotFound
		}
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}
