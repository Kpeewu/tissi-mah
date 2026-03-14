package client

import (
	"context"
	"fmt"

	vehiclepb "github.com/Kpeewu/tissi-mah/services/vehicle-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// VehicleServiceClient est le client gRPC vers vehicle-service.
type VehicleServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient vehiclepb.VehicleServiceClient
	logger     *zap.Logger
}

// NewVehicleServiceClient établit la connexion gRPC vers vehicle-service.
// address doit être au format "host:port" (ex: "vehicle-service:50055").
func NewVehicleServiceClient(address string, logger *zap.Logger) (*VehicleServiceClient, error) {
	logger.Debug("connecting to vehicle-service", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *VehicleServiceClient) Close() error {
	return c.conn.Close()
}

// GetVehicleInfo retourne la marque, la plaque d'immatriculation et le nombre de places d'un véhicule.
// Retourne des chaînes vides et 0 si le véhicule n'est pas trouvé.
func (c *VehicleServiceClient) GetVehicleInfo(ctx context.Context, driverID, vehicleID string) (brand, plate string, numberOfSeats int, err error) {
	c.logger.Debug("client: GetVehicleInfo called",
		zap.String("driverID", driverID),
		zap.String("vehicleID", vehicleID),
	)

	resp, err := c.grpcClient.GetVehicleDetails(ctx, &vehiclepb.GetVehicleDetailsRequest{
		UserId:    driverID,
		VehicleId: vehicleID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", "", 0, nil
		}
		c.logger.Error("client: GetVehicleDetails failed", zap.Error(err))
		return "", "", 0, fmt.Errorf("vehicle-service: GetVehicleDetails failed: %w", err)
	}

	if resp.Vehicle == nil {
		return "", "", 0, nil
	}
	return resp.Vehicle.Brand, resp.Vehicle.LicencePlate, int(resp.Vehicle.NumberOfSeats), nil
}
