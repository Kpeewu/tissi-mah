package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VehicleServiceClient est le client gRPC vers vehicle-service.
type VehicleServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient vehiclepb.VehicleServiceClient
	logger     *zap.Logger
}

// NewVehicleServiceClient établit la connexion gRPC vers vehicle-service.
func NewVehicleServiceClient(address string, logger *zap.Logger) (*VehicleServiceClient, error) {
	logger.Debug("connecting to vehicle-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		logger.Error("failed to connect to vehicle-service", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("vehicle-service: failed to connect to %s: %w", address, err)
	}

	return &VehicleServiceClient{
		conn:       conn,
		grpcClient: vehiclepb.NewVehicleServiceClient(conn),
		logger:     logger,
	}, nil
}

// Close libère la connexion gRPC.
func (c *VehicleServiceClient) Close() error {
	return c.conn.Close()
}

// GetVehicleInfo retourne les informations essentielles d'un véhicule via vehicle-service.
// Retourne nil, nil si le véhicule n'est pas trouvé (dégradation gracieuse pour ne pas
// bloquer le retour des documents).
func (c *VehicleServiceClient) GetVehicleInfo(ctx context.Context, vehicleID string) (*VehicleInfo, error) {
	c.logger.Debug("client: GetVehicleInfo called", zap.String("vehicleID", vehicleID))

	resp, err := c.grpcClient.GetVehicleInfo(ctx, &vehiclepb.GetVehicleInfoRequest{
		VehicleId: vehicleID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			c.logger.Debug("client: vehicle not found", zap.String("vehicleID", vehicleID))
			return nil, nil
		}
		c.logger.Warn("client: GetVehicleInfo failed — dégradation gracieuse",
			zap.String("vehicleID", vehicleID),
			zap.Error(err),
		)
		return nil, nil
	}

	v := resp.GetVehicle()
	if v == nil {
		return nil, nil
	}

	return &VehicleInfo{
		VehicleID:     v.VehicleId,
		Brand:         v.Brand,
		BrandModel:    v.BrandModel,
		Color:         v.Color,
		LicencePlate:  v.LicencePlate,
		NumberOfSeats: v.NumberOfSeats,
		IsVerified:    v.IsVerified,
	}, nil
}
