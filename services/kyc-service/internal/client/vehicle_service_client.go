package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// vehicleServiceClientImpl est le client gRPC vers vehicle-service.
type vehicleServiceClientImpl struct {
	conn       *grpc.ClientConn
	grpcClient vehiclepb.VehicleServiceClient
	logger     *zap.Logger
}

// NewVehicleServiceClient établit la connexion gRPC vers vehicle-service.
// address doit être au format "host:port" (ex: "vehicle-service:50055").
func NewVehicleServiceClient(address string, logger *zap.Logger) (VehicleClient, error) {
	logger.Debug("connecting to vehicle-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to vehicle-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("vehicle-service: failed to connect to %s: %w", address, err)
	}

	return &vehicleServiceClientImpl{
		conn:       conn,
		grpcClient: vehiclepb.NewVehicleServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *vehicleServiceClientImpl) Close() error {
	return c.conn.Close()
}

// SetVehicleVerification fixe le flag is_verified d'un véhicule (best-effort côté
// appelant : le recalcul à chaque décision est auto-réparateur).
func (c *vehicleServiceClientImpl) SetVehicleVerification(ctx context.Context, vehicleID string, isVerified bool) error {
	c.logger.Debug("client: SetVehicleVerification",
		zap.String("vehicleID", vehicleID),
		zap.Bool("isVerified", isVerified),
	)

	_, err := c.grpcClient.SetVehicleVerification(ctx, &vehiclepb.SetVehicleVerificationRequest{
		VehicleId:  vehicleID,
		IsVerified: isVerified,
	})
	if err != nil {
		c.logger.Error("client: SetVehicleVerification failed", zap.Error(err), zap.String("vehicleID", vehicleID))
		return fmt.Errorf("vehicle-service: SetVehicleVerification: %w", err)
	}
	return nil
}
