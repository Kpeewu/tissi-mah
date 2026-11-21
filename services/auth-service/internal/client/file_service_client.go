package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	filepb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/filepb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// FileClient définit les opérations inter-service vers file-service.
type FileClient interface {
	DeleteAllUserFiles(ctx context.Context, userID string) error
	Close() error
}

type fileServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient filepb.FileServiceClient
	logger     *zap.Logger
}

func NewFileServiceClient(address string, logger *zap.Logger) (FileClient, error) {
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		return nil, fmt.Errorf("file-service: failed to connect to %s: %w", address, err)
	}
	return &fileServiceClient{
		conn:       conn,
		grpcClient: filepb.NewFileServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *fileServiceClient) Close() error { return c.conn.Close() }

func (c *fileServiceClient) DeleteAllUserFiles(ctx context.Context, userID string) error {
	resp, err := c.grpcClient.DeleteAllUserFiles(ctx, &filepb.DeleteAllUserFilesRequest{UserID: userID})
	if err != nil {
		c.logger.Error("file: DeleteAllUserFiles failed", zap.Error(err))
		return fmt.Errorf("file-service: DeleteAllUserFiles: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("file-service: DeleteAllUserFiles: %s", resp.ErrorMessage)
	}
	return nil
}
