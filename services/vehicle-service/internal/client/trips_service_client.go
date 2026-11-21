package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// TripsServiceClient définit le contrat pour les appels inter-service vers trips-service.
type TripsServiceClient interface {
	// GetVehicleCompletedTripCount retourne le nombre de trajets complétés pour un véhicule.
	// Retourne 0 si trips-service est indisponible (dégradation gracieuse).
	GetVehicleCompletedTripCount(ctx context.Context, vehicleID string) (int32, error)

	// Close libère la connexion gRPC.
	Close() error
}

type tripsServiceClientImpl struct {
	conn       *grpc.ClientConn
	grpcClient trippb.TripServiceClient
	logger     *zap.Logger
}

// NewTripsServiceClient établit la connexion gRPC vers trips-service.
// address doit être au format "host:port" (ex: "trips-service:50057").
func NewTripsServiceClient(address string, logger *zap.Logger) (TripsServiceClient, error) {
	logger.Debug("connecting to trips-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to trips-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("trips-service: failed to connect to %s: %w", address, err)
	}

	return &tripsServiceClientImpl{
		conn:       conn,
		grpcClient: trippb.NewTripServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *tripsServiceClientImpl) Close() error {
	return c.conn.Close()
}

// GetVehicleCompletedTripCount retourne le nombre de trajets complétés pour un véhicule.
func (c *tripsServiceClientImpl) GetVehicleCompletedTripCount(ctx context.Context, vehicleID string) (int32, error) {
	c.logger.Debug("client: GetVehicleCompletedTripCount called", zap.String("vehicleID", vehicleID))

	resp, err := c.grpcClient.GetVehicleCompletedTripCount(ctx, &trippb.GetVehicleCompletedTripCountRequest{
		VehicleId: vehicleID,
	})
	if err != nil {
		c.logger.Warn("client: GetVehicleCompletedTripCount failed",
			zap.Error(err),
			zap.String("vehicleID", vehicleID),
		)
		return 0, err
	}

	return resp.Count, nil
}
