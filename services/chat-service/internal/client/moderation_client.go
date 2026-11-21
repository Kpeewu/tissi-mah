package client

import (
	"context"
	"fmt"

	moderationpb "github.com/Kpeewu/tissi-mah/services/chat-service/proto/gen/moderationpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ModerationDecision représente le résultat d'une décision de modération.
type ModerationDecision int

const (
	ModerationApproved ModerationDecision = iota
	ModerationFlagged
	ModerationBlocked
)

// ModerationResult est le résultat de l'analyse d'un texte.
type ModerationResult struct {
	Decision ModerationDecision
	Reason   string
}

// ModerationClient abstrait les appels vers moderation-service.
type ModerationClient interface {
	ModerateText(ctx context.Context, contentID, text, authorID string) (*ModerationResult, error)
	Close() error
}

type moderationServiceClient struct {
	conn        *grpc.ClientConn
	client      moderationpb.ModerationServiceClient
	failClosed  bool
	logger      *zap.Logger
}

func NewModerationServiceClient(address string, failClosed bool, logger *zap.Logger) (ModerationClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("moderation-service client dial: %w", err)
	}
	return &moderationServiceClient{
		conn:       conn,
		client:     moderationpb.NewModerationServiceClient(conn),
		failClosed: failClosed,
		logger:     logger,
	}, nil
}

func (c *moderationServiceClient) ModerateText(ctx context.Context, contentID, text, authorID string) (*ModerationResult, error) {
	resp, err := c.client.ModerateText(ctx, &moderationpb.ModerateTextRequest{
		ContentId:   contentID,
		ContentType: "chat_message",
		Text:        text,
		AuthorId:    authorID,
	})
	if err != nil {
		c.logger.Warn("moderation-service unavailable", zap.Error(err))
		if c.failClosed {
			return &ModerationResult{Decision: ModerationBlocked, Reason: "moderation service unavailable"}, nil
		}
		return &ModerationResult{Decision: ModerationApproved}, nil
	}

	result := &ModerationResult{Reason: resp.Reason}
	switch resp.Decision {
	case moderationpb.ModerationDecision_BLOCKED:
		result.Decision = ModerationBlocked
	case moderationpb.ModerationDecision_FLAGGED:
		result.Decision = ModerationFlagged
	default:
		result.Decision = ModerationApproved
	}
	return result, nil
}

func (c *moderationServiceClient) Close() error {
	return c.conn.Close()
}
