package client

import (
	"context"
	"fmt"

	moderationpb "github.com/Kpeewu/tissi-mah/services/file-service/proto/gen/moderationpb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ModerationDecision int

const (
	ModerationApproved ModerationDecision = iota
	ModerationFlagged
	ModerationBlocked
)

type ModerationResult struct {
	Decision ModerationDecision
	Reason   string
}

type ModerationClient interface {
	ModerateImage(ctx context.Context, contentID, authorID string, imageBytes []byte, mimeType string) (*ModerationResult, error)
	Close() error
}

type moderationServiceClient struct {
	conn       *grpc.ClientConn
	client     moderationpb.ModerationServiceClient
	failClosed bool
	logger     *zap.Logger
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

func (c *moderationServiceClient) ModerateImage(ctx context.Context, contentID, authorID string, imageBytes []byte, mimeType string) (*ModerationResult, error) {
	resp, err := c.client.ModerateImage(ctx, &moderationpb.ModerateImageRequest{
		ContentId:   contentID,
		ContentType: "profile_picture",
		ImageBytes:  imageBytes,
		MimeType:    mimeType,
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
