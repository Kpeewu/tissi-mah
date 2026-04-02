package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"go.uber.org/zap"
)

// SESProvider envoie des emails via AWS SES v2.
type SESProvider struct {
	client *sesv2.Client
	from   string
	logger *zap.Logger
}

// NewSESProvider crée un nouveau provider AWS SES.
func NewSESProvider(ctx context.Context, region, from string, logger *zap.Logger) (*SESProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	return &SESProvider{
		client: sesv2.NewFromConfig(cfg),
		from:   from,
		logger: logger,
	}, nil
}

func (p *SESProvider) Send(ctx context.Context, to, subject, bodyText, bodyHTML string) (string, error) {
	p.logger.Debug("ses: sending email",
		zap.String("to", to),
		zap.String("subject", subject),
	)

	input := &sesv2.SendEmailInput{
		FromEmailAddress: &p.from,
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: &subject},
				Body: &types.Body{
					Text: &types.Content{Data: &bodyText},
					Html: &types.Content{Data: &bodyHTML},
				},
			},
		},
	}

	result, err := p.client.SendEmail(ctx, input)
	if err != nil {
		p.logger.Error("ses: send failed", zap.Error(err))
		return "", fmt.Errorf("ses: %w", err)
	}

	messageID := ""
	if result.MessageId != nil {
		messageID = *result.MessageId
	}

	p.logger.Info("ses: email sent",
		zap.String("to", to),
		zap.String("messageId", messageID),
	)

	return messageID, nil
}

func (p *SESProvider) Name() string {
	return "aws_ses"
}
