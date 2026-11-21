package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockSupportUserReadRepository implémente interfaces.SupportUserReadRepository via testify/mock.
type MockSupportUserReadRepository struct {
	mock.Mock
}

func (m *MockSupportUserReadRepository) GetByID(ctx context.Context, userID string) (*domain.SupportUser, error) {
	args := m.Called(ctx, userID)
	u, _ := args.Get(0).(*domain.SupportUser)
	return u, args.Error(1)
}

func (m *MockSupportUserReadRepository) GetByEmail(ctx context.Context, email string) (*domain.SupportUser, error) {
	args := m.Called(ctx, email)
	u, _ := args.Get(0).(*domain.SupportUser)
	return u, args.Error(1)
}

func (m *MockSupportUserReadRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockSupportUserReadRepository) List(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error) {
	args := m.Called(ctx, limit, offset)
	users, _ := args.Get(0).([]*domain.SupportUser)
	return users, args.Int(1), args.Error(2)
}

func (m *MockSupportUserReadRepository) ListPendingPasswordResets(ctx context.Context) ([]*domain.SupportUser, error) {
	args := m.Called(ctx)
	users, _ := args.Get(0).([]*domain.SupportUser)
	return users, args.Error(1)
}

func (m *MockSupportUserReadRepository) ListAdminEmails(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	emails, _ := args.Get(0).([]string)
	return emails, args.Error(1)
}
