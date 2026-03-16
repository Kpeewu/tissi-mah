package client

import (
	"context"
	"fmt"

	filepb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen"
	"github.com/Kpeewu/tissi-mah/services/vehicle-service/internal/domain"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fileServiceClientImpl est le client gRPC vers file-service.
// Utilise insecure.NewCredentials() pour la communication intra-cluster.
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
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
		case "registrationCard":
			docs.VehicleRegistrationURL = d.DocumentUrl
		}
	}

	c.logger.Debug("client: GetVehicleDocuments success",
		zap.String("vehicleID", vehicleID),
		zap.Int("documentCount", len(resp.Documents)),
	)

	return docs, nil
}
