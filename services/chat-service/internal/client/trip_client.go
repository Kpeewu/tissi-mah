package client

import (
	"context"
	"fmt"

	trippb "github.com/Kpeewu/tissi-mah/services/trips-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TripInfo contient les infos nécessaires côté chat.
type TripInfo struct {
	TripID   string
	DriverID string
	Status   string // "scheduled","in_progress","completed","cancelled"
}

// TripClient vérifie auprès du trips-service le statut d'un trajet.
type TripClient interface {
	GetTripByID(ctx context.Context, tripID string) (*TripInfo, error)
	Close() error
}

type tripClientImpl struct {
	grpcClient trippb.TripServiceClient
	conn       *grpc.ClientConn
	logger     *zap.Logger
}

func NewTripClient(addr string, logger *zap.Logger) (TripClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("chat: dial trips-service %s: %w", addr, err)
	}
	return &tripClientImpl{
		grpcClient: trippb.NewTripServiceClient(conn),
		conn:       conn,
		logger:     logger,
	}, nil
}

func (c *tripClientImpl) GetTripByID(ctx context.Context, tripID string) (*TripInfo, error) {
	resp, err := c.grpcClient.GetTripByID(ctx, &trippb.GetTripByIDRequest{
		TripId: tripID,
	})
	if err != nil {
		return nil, fmt.Errorf("trips-service: GetTripByID: %w", err)
	}
	return &TripInfo{
		TripID:   resp.TripId,
		DriverID: resp.DriverId,
		Status:   resp.Status,
	}, nil
}

func (c *tripClientImpl) Close() error { return c.conn.Close() }
