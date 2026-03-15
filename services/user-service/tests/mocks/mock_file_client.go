package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockFileClient est le mock de client.FileClient pour les tests.
type MockFileClient struct {
	mock.Mock
}

func (m *MockFileClient) GetDocumentExpiry(ctx context.Context, userID, documentType string) string {
	return m.Called(ctx, userID, documentType).String(0)
}

func (m *MockFileClient) Close() error {
	return m.Called().Error(0)
}
