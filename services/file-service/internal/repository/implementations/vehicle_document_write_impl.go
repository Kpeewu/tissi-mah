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

type vehicleDocumentWriteImpl struct {
	pool *pgxpool.Pool
}

func NewVehicleDocumentWriteRepository(pool *pgxpool.Pool) i.VehicleDocumentRepositoryWrite {
	return &vehicleDocumentWriteImpl{pool: pool}
}

func (r *vehicleDocumentWriteImpl) Create(ctx context.Context, doc *domain.VehicleDocument) (string, error) {
	query := `INSERT INTO vehicle_documents
	          (document_id, vehicle_id, document_name, document_type,
	           document_url, file_size_bytes, mime_type,
	           document_number, issued_at, expire_at, issuing_authority,
	           status, is_current)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	          RETURNING document_id`

	var documentID string
	err := r.pool.QueryRow(ctx, query,
		doc.DocumentID, doc.VehicleID, doc.DocumentName, doc.DocumentType,
		doc.DocumentURL, doc.FileSizeBytes, doc.MimeType,
		doc.DocumentNumber, doc.IssuedAt, doc.ExpireAt, doc.IssuingAuthority,
		doc.Status, doc.IsCurrent,
	).Scan(&documentID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fileErrors.ErrorDataRetrievalFailed
		}
		return "", fileErrors.ErrorInternalServer
	}
	return documentID, nil
}

func (r *vehicleDocumentWriteImpl) Update(ctx context.Context, doc *domain.VehicleDocument) (*domain.VehicleDocument, error) {
	query := `UPDATE vehicle_documents SET
	          document_name = $1, document_url = $2, file_size_bytes = $3, mime_type = $4,
	          document_number = $5, issued_at = $6, expire_at = $7, issuing_authority = $8,
	          status = $9
	          WHERE document_id = $10
	          RETURNING document_id, vehicle_id, document_name, document_type,
	                    document_url, file_size_bytes, mime_type,
	                    document_number, issued_at, expire_at, issuing_authority,
	                    status, is_current, replaced_by,
	                    uploaded_at, updated_at`

	err := r.pool.QueryRow(ctx, query,
		doc.DocumentName, doc.DocumentURL, doc.FileSizeBytes, doc.MimeType,
		doc.DocumentNumber, doc.IssuedAt, doc.ExpireAt, doc.IssuingAuthority,
		doc.Status, doc.DocumentID,
	).Scan(
		&doc.DocumentID, &doc.VehicleID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingAuthority,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fileErrors.ErrorDocumentNotFound
		}
		return nil, fileErrors.ErrorInternalServer
	}
	return doc, nil
}

func (r *vehicleDocumentWriteImpl) Delete(ctx context.Context, documentID string) error {
	query := `DELETE FROM vehicle_documents WHERE document_id = $1`
	result, err := r.pool.Exec(ctx, query, documentID)
	if err != nil {
		return fileErrors.ErrorInternalServer
	}
	if result.RowsAffected() == 0 {
		return fileErrors.ErrorDocumentNotFound
	}
	return nil
}

func (r *vehicleDocumentWriteImpl) MarkAsReplaced(ctx context.Context, documentID string, replacedBy string) error {
	query := `UPDATE vehicle_documents SET is_current = false, replaced_by = $1 WHERE document_id = $2`
	result, err := r.pool.Exec(ctx, query, replacedBy, documentID)
	if err != nil {
		return fileErrors.ErrorInternalServer
	}
	if result.RowsAffected() == 0 {
		return fileErrors.ErrorDocumentNotFound
	}
	return nil
}
