package provider

import (
	"context"
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.uber.org/zap"
)

// SendGridProvider envoie des emails via l'API SendGrid.
type SendGridProvider struct {
	client *sendgrid.Client
	from   string
	logger *zap.Logger
}

// NewSendGridProvider crée un nouveau provider SendGrid.
func NewSendGridProvider(apiKey, from string, logger *zap.Logger) *SendGridProvider {
	return &SendGridProvider{
		client: sendgrid.NewSendClient(apiKey),
		from:   from,
		logger: logger,
	}
}

func (p *SendGridProvider) Send(ctx context.Context, to, subject, bodyText, bodyHTML string) (string, error) {
	from := mail.NewEmail("TissiMah", p.from)
	toEmail := mail.NewEmail("", to)
	message := mail.NewSingleEmail(from, subject, toEmail, bodyText, bodyHTML)

	p.logger.Debug("sendgrid: sending email",
		zap.String("to", to),
		zap.String("subject", subject),
	)

	response, err := p.client.SendWithContext(ctx, message)
	if err != nil {
		return "", fmt.Errorf("sendgrid: %w", err)
	}

	if response.StatusCode >= 400 {
		p.logger.Error("sendgrid: send failed",
			zap.Int("statusCode", response.StatusCode),
			zap.String("body", response.Body),
		)
		return "", fmt.Errorf("sendgrid: status %d, body: %s", response.StatusCode, response.Body)
	}

	messageID := ""
	if ids, ok := response.Headers["X-Message-Id"]; ok && len(ids) > 0 {
		messageID = ids[0]
	}

	p.logger.Info("sendgrid: email sent",
		zap.String("to", to),
		zap.String("messageId", messageID),
	)

	return messageID, nil
}

func (p *SendGridProvider) Name() string {
	return "sendgrid"
}
