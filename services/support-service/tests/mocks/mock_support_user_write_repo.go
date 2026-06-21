package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

// MockSupportUserWriteRepository implémente interfaces.SupportUserWriteRepository via testify/mock.
type MockSupportUserWriteRepository struct {
	mock.Mock
}

func (m *MockSupportUserWriteRepository) Create(ctx context.Context, u *domain.SupportUser) error {
	return m.Called(ctx, u).Error(0)
}

func (m *MockSupportUserWriteRepository) UpdatePassword(ctx context.Context, userID, hash string, mustChange bool) error {
	return m.Called(ctx, userID, hash, mustChange).Error(0)
}

func (m *MockSupportUserWriteRepository) UpdateEmail(ctx context.Context, userID, newEmail string) error {
	return m.Called(ctx, userID, newEmail).Error(0)
}

func (m *MockSupportUserWriteRepository) UpdateName(ctx context.Context, userID, firstName, lastName string) error {
	return m.Called(ctx, userID, firstName, lastName).Error(0)
}

func (m *MockSupportUserWriteRepository) UpdateRole(ctx context.Context, userID, role string) error {
	return m.Called(ctx, userID, role).Error(0)
}

func (m *MockSupportUserWriteRepository) Deactivate(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockSupportUserWriteRepository) Activate(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockSupportUserWriteRepository) SoftDelete(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockSupportUserWriteRepository) SetPasswordResetRequested(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockSupportUserWriteRepository) ClearPasswordResetRequested(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}

func (m *MockSupportUserWriteRepository) ClaimPasswordResetRequest(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
