package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/kyc-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

type MockKYCService struct {
	mock.Mock
}

func (m *MockKYCService) CreateInquiry(ctx context.Context, input serviceInterfaces.CreateInquiryInput) (*serviceInterfaces.CreateInquiryResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.CreateInquiryResult), args.Error(1)
}

func (m *MockKYCService) GetInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.InquiryDetail, error) {
	args := m.Called(ctx, userID, personaInquiryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.InquiryDetail), args.Error(1)
}

func (m *MockKYCService) GetKYCStatus(ctx context.Context, userID string) (*serviceInterfaces.KYCStatus, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.KYCStatus), args.Error(1)
}

func (m *MockKYCService) ResumeInquiry(ctx context.Context, userID string, personaInquiryID string) (*serviceInterfaces.ResumeResult, error) {
	args := m.Called(ctx, userID, personaInquiryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.ResumeResult), args.Error(1)
}

func (m *MockKYCService) ProcessWebhook(ctx context.Context, input serviceInterfaces.WebhookInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *MockKYCService) GetAdminReviews(ctx context.Context, input serviceInterfaces.GetAdminReviewsInput) ([]*serviceInterfaces.AdminReviewItem, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*serviceInterfaces.AdminReviewItem), args.Error(1)
}

func (m *MockKYCService) GetAdminReview(ctx context.Context, userID string, reviewID string) (*serviceInterfaces.AdminReviewDetail, error) {
	args := m.Called(ctx, userID, reviewID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.AdminReviewDetail), args.Error(1)
}

func (m *MockKYCService) OverrideReview(ctx context.Context, input serviceInterfaces.OverrideReviewInput) (*serviceInterfaces.OverrideResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.OverrideResult), args.Error(1)
}

func (m *MockKYCService) ValidateDocument(ctx context.Context, input serviceInterfaces.ValidateDocumentInput) (*serviceInterfaces.ValidateDocumentResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.ValidateDocumentResult), args.Error(1)
}

func (m *MockKYCService) GetManualReviewRequests(ctx context.Context, input serviceInterfaces.GetManualReviewRequestsInput) (*serviceInterfaces.GetManualReviewRequestsResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*serviceInterfaces.GetManualReviewRequestsResult), args.Error(1)
}

func (m *MockKYCService) GetManualReviewRequestDetail(ctx context.Context, userID string) (*domain.ManualReviewRequestDetail, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ManualReviewRequestDetail), args.Error(1)
}

// Compile-time check
var _ serviceInterfaces.KYCService = (*MockKYCService)(nil)
var _ = (*MockKYCService)(nil)

// Compile-time check pour PendingReview et LatestRejection (assure que les types domain sont utilisés)
var _ *domain.PendingReview = nil
var _ *domain.LatestRejection = nil
