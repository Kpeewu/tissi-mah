package fcm

import (
	"context"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// Client implémente FCMClient via Firebase Cloud Messaging v1.
type Client struct {
	app       *firebase.App
	msgClient *messaging.Client
	logger    *zap.Logger
}

func NewClient(credentialsPath string, logger *zap.Logger) (*Client, error) {
	ctx := context.Background()

	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, err
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{
		app:       app,
		msgClient: msgClient,
		logger:    logger,
	}, nil
}

// Send envoie une notification push à un seul token FCM.
func (c *Client) Send(ctx context.Context, title, body, token string, data map[string]string) error {
	msg := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Token: token,
		Data:  data,
	}

	_, err := c.msgClient.Send(ctx, msg)
	return err
}

// SendMulticast envoie une notification push à plusieurs tokens (max 500).
func (c *Client) SendMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) ([]SendResult, error) {
	msg := &messaging.MulticastMessage{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Tokens: tokens,
		Data:   data,
	}

	resp, err := c.msgClient.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, err
	}

	results := make([]SendResult, len(resp.Responses))
	for i, r := range resp.Responses {
		result := SendResult{Token: tokens[i], Success: r.Success}
		if r.Error != nil {
			if messaging.IsUnregistered(r.Error) {
				result.ErrorCode = "UNREGISTERED"
			} else if messaging.IsInvalidArgument(r.Error) {
				result.ErrorCode = "INVALID_ARGUMENT"
			} else {
				result.ErrorCode = "UNKNOWN"
			}
		}
		results[i] = result
	}

	return results, nil
}

// Close libère les ressources du client Firebase.
func (c *Client) Close() error {
	return nil
}
