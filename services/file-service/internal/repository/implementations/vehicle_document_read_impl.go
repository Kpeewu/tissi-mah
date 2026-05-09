package implementations

import (
	"context"
	"errors"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	i "github.com/Kpeewu/tissi-mah/services/file-service/internal/repository/interfaces"
	fileErrors "github.com/Kpeewu/tissi-mah/services/file-service/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type vehicleDocumentReadImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewVehicleDocumentReadRepository(pool *pgxpool.Pool, logger *zap.Logger) i.VehicleDocumentRepositoryRead {
	return &vehicleDocumentReadImpl{pool: pool, logger: logger}
}

func (r *vehicleDocumentReadImpl) GetByID(ctx context.Context, documentID string) (*domain.VehicleDocument, error) {
	r.logger.Debug("récupération du document véhicule par ID", zap.String("documentID", documentID))

	query := `SELECT document_id, user_id, vehicle_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_authority,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM vehicle_documents WHERE document_id = $1`

	doc := &domain.VehicleDocument{}
	err := r.pool.QueryRow(ctx, query, documentID).Scan(
		&doc.DocumentID, &doc.UserID, &doc.VehicleID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingAuthority,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Debug("document véhicule non trouvé", zap.String("documentID", documentID))
			return nil, fileErrors.ErrorDocumentNotFound
		}
		r.logger.Error("erreur récupération document véhicule par ID", zap.String("documentID", documentID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	return doc, nil
}

func (r *vehicleDocumentReadImpl) GetByVehicleID(ctx context.Context, vehicleID string) ([]*domain.VehicleDocument, error) {
	r.logger.Debug("récupération des documents véhicule par vehicleID", zap.String("vehicleID", vehicleID))

	query := `SELECT document_id, user_id, vehicle_id, document_name, document_type,
	                 document_url, file_size_bytes, mime_type,
	                 document_number, issued_at, expire_at, issuing_authority,
	                 status, is_current, replaced_by,
	                 uploaded_at, updated_at
	          FROM vehicle_documents WHERE vehicle_id = $1
	          ORDER BY uploaded_at DESC`

	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		r.logger.Error("erreur récupération documents véhicule par vehicleID", zap.String("vehicleID", vehicleID), zap.Error(err))
		return nil, fileErrors.ErrorDataRetrievalFailed
	}
	defer rows.Close()

	var docs []*domain.VehicleDocument
	for rows.Next() {
		doc := &domain.VehicleDocument{}
		err := rows.Scan(
			&doc.DocumentID, &doc.UserID, &doc.VehicleID, &doc.DocumentName, &doc.DocumentType,
			&doc.DocumentURL, &doc.FileSizeBytes, &doc.MimeType,
			&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingAuthority,
			&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
			&doc.UploadedAt, &doc.UpdatedAt,
		)
		if err != nil {
			r.logger.Error("erreur scan document véhicule", zap.String("vehicleID", vehicleID), zap.Error(err))
			return nil, fileErrors.ErrorDataRetrievalFailed
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
