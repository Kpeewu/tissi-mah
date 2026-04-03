package client

import (
	"context"
	"fmt"

	pushpb "github.com/Kpeewu/tissi-mah/services/push-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PushServiceClient struct {
	conn   *grpc.ClientConn
	client pushpb.PushServiceClient
	logger *zap.Logger
}

func NewPushServiceClient(address string, logger *zap.Logger) (*PushServiceClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to push-service at %s: %w", address, err)
	}

	return &PushServiceClient{
		conn:   conn,
		client: pushpb.NewPushServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *PushServiceClient) SendPush(ctx context.Context, title, body, token string, data map[string]string) (bool, string, error) {
	resp, err := c.client.SendPush(ctx, &pushpb.SendPushRequest{
		Title:    title,
		Body:     body,
		FcmToken: token,
		Data:     data,
	})
	if err != nil {
		c.logger.Error("push rpc failed", zap.Error(err))
		return false, "", err
	}

	return resp.Success, resp.ErrorCode, nil
}

func (c *PushServiceClient) SendPushMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) (int32, int32, error) {
	resp, err := c.client.SendPushMulticast(ctx, &pushpb.SendPushMulticastRequest{
		Title:     title,
		Body:      body,
		FcmTokens: tokens,
		Data:      data,
	})
	if err != nil {
		c.logger.Error("push multicast rpc failed", zap.Error(err))
		return 0, 0, err
	}

	return resp.SuccessCount, resp.FailureCount, nil
}

func (c *PushServiceClient) Close() error {
	return c.conn.Close()
}
