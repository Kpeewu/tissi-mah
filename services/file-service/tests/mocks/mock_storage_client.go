package mocks

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) Upload(ctx context.Context, key string, data io.Reader, contentType string, size int64) (string, error) {
	args := m.Called(ctx, key, data, contentType, size)
	return args.String(0), args.Error(1)
}

func (m *MockStorageClient) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockStorageClient) GenerateURL(key string) string {
	args := m.Called(key)
	return args.String(0)
}
