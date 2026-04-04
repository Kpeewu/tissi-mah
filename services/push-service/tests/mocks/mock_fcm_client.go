package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/push-service/internal/fcm"
	"github.com/stretchr/testify/mock"
)

// MockFCMClient est un mock de fcm.FCMClient pour les tests unitaires.
type MockFCMClient struct {
	mock.Mock
}

func (m *MockFCMClient) Send(ctx context.Context, title, body, token string, data map[string]string) error {
	args := m.Called(ctx, title, body, token, data)
	return args.Error(0)
}

func (m *MockFCMClient) SendMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) ([]fcm.SendResult, error) {
	args := m.Called(ctx, title, body, tokens, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]fcm.SendResult), args.Error(1)
}

func (m *MockFCMClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
