package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/file-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockUserDocumentRepositoryRead struct {
	mock.Mock
}

func (m *MockUserDocumentRepositoryRead) GetByID(ctx context.Context, documentID string) (*domain.UserDocument, error) {
	args := m.Called(ctx, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserDocument), args.Error(1)
}

func (m *MockUserDocumentRepositoryRead) GetByUserID(ctx context.Context, userID string) ([]*domain.UserDocument, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserDocument), args.Error(1)
}

func (m *MockUserDocumentRepositoryRead) GetCurrentByUserIDAndType(ctx context.Context, userID string, documentType string) (*domain.UserDocument, error) {
	args := m.Called(ctx, userID, documentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.UserDocument), args.Error(1)
}
