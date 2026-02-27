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

type vehicleDocumentReadImpl struct {
	pool *pgxpool.Pool
}

func NewVehicleDocumentReadRepository(pool *pgxpool.Pool) i.VehicleDocumentRepositoryRead {
	return &vehicleDocumentReadImpl{pool: pool}
}

func (r *vehicleDocumentReadImpl) GetByID(ctx context.Context, documentID string) (*domain.VehicleDocument, error) {
	query := `SELECT document_id, vehicle_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_authority,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM vehicle_documents WHERE document_id = $1`

	doc := &domain.VehicleDocument{}
	err := r.pool.QueryRow(ctx, query, documentID).Scan(
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
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}

func (r *vehicleDocumentReadImpl) GetByVehicleID(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error) {
	query := `SELECT document_id, vehicle_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_authority,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM vehicle_documents WHERE vehicle_id = $1
	          ORDER BY uploaded_at DESC`

	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var docs []*domain.VehicleDocument
	for rows.Next() {
		doc := &domain.VehicleDocument{}
		err := rows.Scan(
			&doc.DocumentID, &doc.VehicleID, &doc.DocumentName, &doc.DocumentType,
			&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
			&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingAuthority,
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
