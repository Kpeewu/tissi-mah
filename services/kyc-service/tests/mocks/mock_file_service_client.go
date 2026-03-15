package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockFileServiceClient struct {
	mock.Mock
}

func (m *MockFileServiceClient) GetCurrentUserDocument(ctx context.Context, userID string, documentType string) (*domain.DocumentRef, error) {
	args := m.Called(ctx, userID, documentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentRef), args.Error(1)
}

func (m *MockFileServiceClient) GetVehicleDocuments(ctx context.Context, vehicleID string) ([]*domain.DocumentRef, error) {
	args := m.Called(ctx, vehicleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DocumentRef), args.Error(1)
}

func (m *MockFileServiceClient) CreateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error) {
	args := m.Called(ctx, review)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) GetDocumentReview(ctx context.Context, reviewID string) (*domain.Review, error) {
	args := m.Called(ctx, reviewID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) GetDocumentReviewByPersonaInquiryID(ctx context.Context, personaInquiryID string) (*domain.Review, error) {
	args := m.Called(ctx, personaInquiryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) GetDocumentReviewsByUserID(ctx context.Context, userID string) ([]*domain.Review, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) UpdateDocumentReview(ctx context.Context, review *domain.Review) (*domain.Review, error) {
	args := m.Called(ctx, review)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) ListDocumentReviews(ctx context.Context, userID string, status string, decision string, page int32, pageSize int32) ([]*domain.Review, error) {
	args := m.Called(ctx, userID, status, decision, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Review), args.Error(1)
}

func (m *MockFileServiceClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
