package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/client"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
)

type kycServiceImpl struct {
	fileClient    client.FileServiceClient
	personaClient client.PersonaClient
	logger        *zap.Logger
}

// NewKYCService crée une nouvelle instance du service KYC
func NewKYCService(
	fileClient client.FileServiceClient,
	personaClient client.PersonaClient,
	logger *zap.Logger,
) serviceInterfaces.KYCService {
	return &kycServiceImpl{
		fileClient:    fileClient,
		personaClient: personaClient,
		logger:        logger,
	}
}

func (s *kycServiceImpl) CreateInquiry(ctx context.Context, input serviceInterfaces.CreateInquiryInput) (*serviceInterfaces.CreateInquiryResult, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) GetInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.InquiryDetail, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) GetKYCStatus(ctx context.Context, userID string) (*serviceInterfaces.KYCStatus, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) ResumeInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.ResumeResult, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) ProcessWebhook(ctx context.Context, input serviceInterfaces.WebhookInput) error {
	// TODO: implémenter
	return nil
}

func (s *kycServiceImpl) GetAdminReviews(ctx context.Context, input serviceInterfaces.GetAdminReviewsInput) ([]*serviceInterfaces.AdminReviewItem, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) GetAdminReview(ctx context.Context, userID string, reviewID string) (*serviceInterfaces.AdminReviewDetail, error) {
	// TODO: implémenter
	return nil, nil
}

func (s *kycServiceImpl) OverrideReview(ctx context.Context, input serviceInterfaces.OverrideReviewInput) (*serviceInterfaces.OverrideResult, error) {
	// TODO: implémenter
	return nil, nil
}
