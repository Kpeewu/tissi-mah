package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	chatpb "github.com/Kpeewu/tissi-mah/services/auth-service/proto/gen/chatpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ChatClient définit les opérations inter-service vers chat-service.
type ChatClient interface {
	AnonymizeUserData(ctx context.Context, userID string) error
	Close() error
}

type chatServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient chatpb.ChatServiceClient
	logger     *zap.Logger
}

func NewChatServiceClient(address string, logger *zap.Logger) (ChatClient, error) {
	conn, err := grpcutil.NewClientConn(address, logger)
	if err != nil {
		return nil, fmt.Errorf("chat-service: failed to connect to %s: %w", address, err)
	}
	return &chatServiceClient{
		conn:       conn,
		grpcClient: chatpb.NewChatServiceClient(conn),
		logger:     logger,
	}, nil
}

func (c *chatServiceClient) Close() error { return c.conn.Close() }

func (c *chatServiceClient) AnonymizeUserData(ctx context.Context, userID string) error {
	resp, err := c.grpcClient.AnonymizeUserData(ctx, &chatpb.AnonymizeUserDataRequest{UserId: userID})
	if err != nil {
		c.logger.Error("chat: AnonymizeUserData failed", zap.Error(err))
		return fmt.Errorf("chat-service: AnonymizeUserData: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("chat-service: AnonymizeUserData: %s", resp.ErrorMessage)
	}
	return nil
}
