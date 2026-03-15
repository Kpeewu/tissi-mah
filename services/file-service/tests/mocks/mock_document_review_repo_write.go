package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockDocumentReviewRepositoryWrite struct {
	mock.Mock
}

func (m *MockDocumentReviewRepositoryWrite) Create(ctx context.Context, review *domain.DocumentReview) (string, error) {
	args := m.Called(ctx, review)
	return args.String(0), args.Error(1)
}
