package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockEmailClient struct {
	mock.Mock
}

func (m *MockEmailClient) SendEmail(ctx context.Context, to, subject, bodyText, bodyHTML string) (bool, string, error) {
	args := m.Called(ctx, to, subject, bodyText, bodyHTML)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockEmailClient) Close() error {
	args := m.Called()
	return args.Error(0)
}
