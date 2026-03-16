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

func (m *MockFileClient) UploadProfilePicture(ctx context.Context, userID string, imageBytes []byte) (string, error) {
	args := m.Called(ctx, userID, imageBytes)
	return args.String(0), args.Error(1)
}

func (m *MockFileClient) Close() error {
	return m.Called().Error(0)
}
