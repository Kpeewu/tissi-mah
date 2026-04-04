package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockEmailProvider est un mock de provider.EmailProvider pour les tests unitaires.
type MockEmailProvider struct {
	mock.Mock
}

func (m *MockEmailProvider) Send(ctx context.Context, to, subject, bodyText, bodyHTML string) (string, error) {
	args := m.Called(ctx, to, subject, bodyText, bodyHTML)
	return args.String(0), args.Error(1)
}

func (m *MockEmailProvider) Name() string {
	args := m.Called()
	return args.String(0)
}
