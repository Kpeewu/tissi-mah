package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	emailpb "github.com/Kpeewu/tissi-mah/services/email-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type EmailServiceClient struct {
	conn   *grpc.ClientConn
	client emailpb.EmailServiceClient
	logger *zap.Logger
}

func NewEmailServiceClient(address string, logger *zap.Logger) (*EmailServiceClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to email-service at %s: %w", address, err)
	}

	return &EmailServiceClient{
		conn:   conn,
		client: emailpb.NewEmailServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *EmailServiceClient) SendEmail(ctx context.Context, to, subject, bodyText, bodyHTML string) (bool, string, error) {
	resp, err := c.client.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:       to,
		Subject:  subject,
		BodyText: bodyText,
		BodyHtml: bodyHTML,
	})
	if err != nil {
		c.logger.Error("email rpc failed", zap.Error(err))
		return false, "", err
	}

	return resp.Success, resp.ErrorMessage, nil
}

func (c *EmailServiceClient) Close() error {
	return c.conn.Close()
}
