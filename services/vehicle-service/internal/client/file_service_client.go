package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fileServiceClientImpl est le client gRPC vers file-service.
// Utilise TLS avec skip-verify pour la communication intra-cluster.
type fileServiceClientImpl struct {
	conn       *grpc.ClientConn
	grpcClient filepb.FileServiceClient
	logger     *zap.Logger
}

// NewFileServiceClient établit la connexion gRPC vers file-service.
// address doit être au format "host:port" (ex: "file-service:50053").
func NewFileServiceClient(address string, logger *zap.Logger) (FileServiceClient, error) {
	logger.Debug("connecting to file-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to file-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("file-service: failed to connect to %s: %w", address, err)
	}

	return &fileServiceClientImpl{
		conn:       conn,
		grpcClient: filepb.NewFileServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *fileServiceClientImpl) Close() error {
	return c.conn.Close()
}

// GetVehicleDocuments récupère les documents associés à un véhicule depuis file-service.
// Les documents sont indexés par DocumentType : "insurance" et "registrationCard".
// Retourne des URLs vides si aucun document n'est trouvé (tolérance aux pannes).
func (c *fileServiceClientImpl) GetVehicleDocuments(ctx context.Context, vehicleID string) (domain.VehicleDocuments, error) {
	c.logger.Debug("client: GetVehicleDocuments called", zap.String("vehicleID", vehicleID))

	resp, err := c.grpcClient.GetVehicleDocuments(ctx, &filepb.GetVehicleDocumentsRequest{
		VehicleId: vehicleID,
	})
	if err != nil {
		c.logger.Error("client: GetVehicleDocuments failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return domain.VehicleDocuments{}, fmt.Errorf("file-service: GetVehicleDocuments failed: %w", err)
	}

	docs := domain.VehicleDocuments{}
	for _, d := range resp.Documents {
		switch d.DocumentType {
		case "insurance":
			docs.AssuranceURL = d.DocumentUrl
			docs.AssuranceStatus = mapDocStatus(d.Status)
		case "registrationCard":
			docs.VehicleRegistrationURL = d.DocumentUrl
			docs.VehicleRegistrationStatus = mapDocStatus(d.Status)
		}
	}

	c.logger.Debug("client: GetVehicleDocuments success",
		zap.String("vehicleID", vehicleID),
		zap.Int("documentCount", len(resp.Documents)),
	)

	return docs, nil
}

// GetCurrentUserDocument récupère l'URL et le statut du document courant d'un utilisateur.
// Retourne ("", "MISSING", nil) si aucun document n'est trouvé.
func (c *fileServiceClientImpl) GetCurrentUserDocument(ctx context.Context, userID string, docType string) (string, string, error) {
	c.logger.Debug("client: GetCurrentUserDocument called",
		zap.String("userID", userID),
		zap.String("docType", docType),
	)

	resp, err := c.grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
		UserId:       userID,
		DocumentType: docType,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", "MISSING", nil
		}
		c.logger.Warn("client: GetCurrentUserDocument failed",
			zap.Error(err),
			zap.String("userID", userID),
			zap.String("docType", docType),
		)
		return "", "MISSING", err
	}

	return resp.DocumentUrl, mapDocStatus(resp.Status), nil
}

// mapDocStatus convertit un statut file-service vers le statut exposé à l'app.
func mapDocStatus(s string) string {
	switch s {
	case "approved":
		return "VALIDATED"
	case "pending", "underReview":
		return "PENDING"
	case "rejected":
		return "REJECTED"
	case "expired":
		return "EXPIRED"
	default:
		return "MISSING"
	}
}
