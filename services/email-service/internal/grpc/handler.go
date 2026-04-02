package grpc

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/email-service/internal/provider"
	emailpb "github.com/Kpeewu/tissi-mah/services/email-service/proto/gen"
	"go.uber.org/zap"
)

const serviceVersion = "1.0.0"

// EmailHandler implémente emailpb.EmailServiceServer.
type EmailHandler struct {
	emailpb.UnimplementedEmailServiceServer
	provider provider.EmailProvider
	logger   *zap.Logger
}

func NewEmailHandler(provider provider.EmailProvider, logger *zap.Logger) *EmailHandler {
	return &EmailHandler{provider: provider, logger: logger}
}

// SendEmail envoie un email via le fournisseur configuré.
func (h *EmailHandler) SendEmail(ctx context.Context, req *emailpb.SendEmailRequest) (*emailpb.SendEmailResponse, error) {
	h.logger.Debug("handler: SendEmail called",
		zap.String("to", req.To),
		zap.String("subject", req.Subject),
	)

	if req.To == "" || req.Subject == "" {
		return &emailpb.SendEmailResponse{
			Success:      false,
			ErrorMessage: "to and subject are required",
		}, nil
	}

	messageID, err := h.provider.Send(ctx, req.To, req.Subject, req.BodyText, req.BodyHtml)
	if err != nil {
		h.logger.Error("handler: SendEmail failed",
			zap.Error(err),
			zap.String("provider", h.provider.Name()),
		)
		return &emailpb.SendEmailResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("handler: SendEmail success",
		zap.String("to", req.To),
		zap.String("provider", h.provider.Name()),
		zap.String("messageId", messageID),
	)

	return &emailpb.SendEmailResponse{
		Success:           true,
		ProviderMessageId: messageID,
	}, nil
}

// Health retourne l'état de santé du service.
func (h *EmailHandler) Health(ctx context.Context, req *emailpb.HealthRequest) (*emailpb.HealthResponse, error) {
	return &emailpb.HealthResponse{
		Status:  "ok",
		Version: serviceVersion,
		Service: "email-service",
	}, nil
}
