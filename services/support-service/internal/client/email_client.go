package client

import (
	"context"
	"fmt"

	emailpb "github.com/Kpeewu/tissi-mah/services/email-service/proto/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EmailClient encapsule l'appel gRPC vers email-service.
type EmailClient struct {
	conn   *grpc.ClientConn
	client emailpb.EmailServiceClient
	logger *zap.Logger
}

func NewEmailClient(host, port string, logger *zap.Logger) (*EmailClient, error) {
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial email-service: %w", err)
	}
	return &EmailClient{
		conn:   conn,
		client: emailpb.NewEmailServiceClient(conn),
		logger: logger,
	}, nil
}

func (c *EmailClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// SendEmail envoie un email transactionnel via email-service.
func (c *EmailClient) SendEmail(ctx context.Context, to, subject, bodyText, bodyHTML string) error {
	resp, err := c.client.SendEmail(ctx, &emailpb.SendEmailRequest{
		To:       to,
		Subject:  subject,
		BodyText: bodyText,
		BodyHtml: bodyHTML,
	})
	if err != nil {
		c.logger.Error("send email failed", zap.String("to", to), zap.Error(err))
		return err
	}
	if !resp.GetSuccess() {
		c.logger.Warn("email-service returned failure",
			zap.String("to", to),
			zap.String("err", resp.GetErrorMessage()),
		)
		return fmt.Errorf("email-service failure: %s", resp.GetErrorMessage())
	}
	return nil
}
