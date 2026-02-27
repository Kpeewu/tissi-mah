package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/client"
	"github.com/stretchr/testify/mock"
)

type MockAuthClient struct {
	mock.Mock
}

func (m *MockAuthClient) GetAuthInfo(ctx context.Context, authID string) (*client.AuthInfo, error) {
	args := m.Called(ctx, authID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.AuthInfo), args.Error(1)
}

func (m *MockAuthClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
