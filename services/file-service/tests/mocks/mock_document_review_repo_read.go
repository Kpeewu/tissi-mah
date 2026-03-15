package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockDocumentReviewRepositoryRead struct {
	mock.Mock
}

func (m *MockDocumentReviewRepositoryRead) GetByID(ctx context.Context, reviewID string) (*domain.DocumentReview, error) {
	args := m.Called(ctx, reviewID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentReview), args.Error(1)
}

func (m *MockDocumentReviewRepositoryRead) GetByUserDocumentID(ctx context.Context, userDocumentID string) ([]*domain.DocumentReview, error) {
	args := m.Called(ctx, userDocumentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DocumentReview), args.Error(1)
}

func (m *MockDocumentReviewRepositoryRead) GetByVehicleDocumentID(ctx context.Context, vehicleDocumentID string) ([]*domain.DocumentReview, error) {
	args := m.Called(ctx, vehicleDocumentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DocumentReview), args.Error(1)
}
