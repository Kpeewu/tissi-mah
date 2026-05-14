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

type vehicleDocumentWriteImpl struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewVehicleDocumentWriteRepository(pool *pgxpool.Pool, logger *zap.Logger) i.VehicleDocumentRepositoryWrite {
	return &vehicleDocumentWriteImpl{pool: pool, logger: logger}
}

func (r *vehicleDocumentWriteImpl) Create(ctx context.Context, doc *domain.VehicleDocument) (string, error) {
	r.logger.Debug("création d'un document véhicule", zap.String("documentID", doc.DocumentID), zap.String("vehicleID", doc.VehicleID))

	query := `INSERT INTO vehicle_documents
	          (document_id, user_id, vehicle_id, document_name, document_type,
	           document_key, file_size_bytes, mime_type,
	           document_number, issued_at, expire_at, issuing_authority,
	           status, is_current)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	          RETURNING document_id`

	var documentID string
	err := r.pool.QueryRow(ctx, query,
		doc.DocumentID, doc.UserID, doc.VehicleID, doc.DocumentName, doc.DocumentType,
		doc.DocumentKey, doc.FileSizeBytes, doc.MimeType,
		doc.DocumentNumber, doc.IssuedAt, doc.ExpireAt, doc.IssuingAuthority,
		doc.Status, doc.IsCurrent,
	).Scan(&documentID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error("création document véhicule : aucune ligne retournée", zap.String("documentID", doc.DocumentID), zap.Error(err))
			return "", fileErrors.ErrorDataRetrievalFailed
		}
		r.logger.Error("erreur création document véhicule", zap.String("documentID", doc.DocumentID), zap.Error(err))
		return "", fileErrors.ErrorInternalServer
	}

	r.logger.Info("document véhicule créé avec succès", zap.String("documentID", documentID))
	return documentID, nil
}

func (r *vehicleDocumentWriteImpl) Update(ctx context.Context, doc *domain.VehicleDocument) (*domain.VehicleDocument, error) {
	r.logger.Debug("mise à jour du document véhicule", zap.String("documentID", doc.DocumentID))

	query := `UPDATE vehicle_documents SET
	          document_name = $1, document_key = $2, file_size_bytes = $3, mime_type = $4,
	          document_number = $5, issued_at = $6, expire_at = $7, issuing_authority = $8,
	          status = $9
	          WHERE document_id = $10
	          RETURNING document_id, user_id, vehicle_id, document_name, document_type,
	                    document_key, file_size_bytes, mime_type,
	                    document_number, issued_at, expire_at, issuing_authority,
	                    status, is_current, replaced_by,
	                    uploaded_at, updated_at`

	err := r.pool.QueryRow(ctx, query,
		doc.DocumentName, doc.DocumentKey, doc.FileSizeBytes, doc.MimeType,
		doc.DocumentNumber, doc.IssuedAt, doc.ExpireAt, doc.IssuingAuthority,
		doc.Status, doc.DocumentID,
	).Scan(
		&doc.DocumentID, &doc.UserID, &doc.VehicleID, &doc.DocumentName, &doc.DocumentType,
		&doc.DocumentKey, &doc.FileSizeBytes, &doc.MimeType,
		&doc.DocumentNumber, &doc.IssuedAt, &doc.ExpireAt, &doc.IssuingAuthority,
		&doc.Status, &doc.IsCurrent, &doc.ReplacedBy,
		&doc.UploadedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error("mise à jour document véhicule : document non trouvé", zap.String("documentID", doc.DocumentID), zap.Error(err))
			return nil, fileErrors.ErrorDocumentNotFound
		}
		r.logger.Error("erreur mise à jour document véhicule", zap.String("documentID", doc.DocumentID), zap.Error(err))
		return nil, fileErrors.ErrorInternalServer
	}

	r.logger.Info("document véhicule mis à jour avec succès", zap.String("documentID", doc.DocumentID))
	return doc, nil
}

func (r *vehicleDocumentWriteImpl) Delete(ctx context.Context, documentID string) error {
	r.logger.Debug("suppression du document véhicule", zap.String("documentID", documentID))

	query := `DELETE FROM vehicle_documents WHERE document_id = $1`
	result, err := r.pool.Exec(ctx, query, documentID)
	if err != nil {
		r.logger.Error("erreur suppression document véhicule", zap.String("documentID", documentID), zap.Error(err))
		return fileErrors.ErrorInternalServer
	}
	if result.RowsAffected() == 0 {
		r.logger.Error("suppression document véhicule : document non trouvé", zap.String("documentID", documentID))
		return fileErrors.ErrorDocumentNotFound
	}

	r.logger.Info("document véhicule supprimé avec succès", zap.String("documentID", documentID))
	return nil
}

func (r *vehicleDocumentWriteImpl) MarkAsReplaced(ctx context.Context, documentID string, replacedBy string) error {
	r.logger.Debug("marquage du document véhicule comme remplacé",
		zap.String("documentID", documentID),
		zap.String("replacedBy", replacedBy),
	)

	query := `UPDATE vehicle_documents SET is_current = false, replaced_by = $1 WHERE document_id = $2`
	result, err := r.pool.Exec(ctx, query, replacedBy, documentID)
	if err != nil {
		r.logger.Error("erreur marquage document véhicule comme remplacé", zap.String("documentID", documentID), zap.Error(err))
		return fileErrors.ErrorInternalServer
	}
	if result.RowsAffected() == 0 {
		r.logger.Error("marquage document véhicule : document non trouvé", zap.String("documentID", documentID))
		return fileErrors.ErrorDocumentNotFound
	}

	r.logger.Info("document véhicule marqué comme remplacé", zap.String("documentID", documentID), zap.String("replacedBy", replacedBy))
	return nil
}
