package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockPushClient struct {
	mock.Mock
}

func (m *MockPushClient) SendPush(ctx context.Context, title, body, token string, data map[string]string) (bool, string, error) {
	args := m.Called(ctx, title, body, token, data)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockPushClient) SendPushMulticast(ctx context.Context, title, body string, tokens []string, data map[string]string) (int32, int32, error) {
	args := m.Called(ctx, title, body, tokens, data)
	return int32(args.Int(0)), int32(args.Int(1)), args.Error(2)
}

func (m *MockPushClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
