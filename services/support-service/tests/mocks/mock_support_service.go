package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/domain"
	svcIfaces "github.com/Kpeewu/tissi-mah/services/support-service/internal/service/interfaces"
	"github.com/stretchr/testify/mock"
)

// MockSupportService implémente svcIfaces.SupportService pour les tests handler.
type MockSupportService struct {
	mock.Mock
}

func (m *MockSupportService) Login(ctx context.Context, email, pwd string) (*svcIfaces.LoginResult, error) {
	args := m.Called(ctx, email, pwd)
	r, _ := args.Get(0).(*svcIfaces.LoginResult)
	return r, args.Error(1)
}

func (m *MockSupportService) VerifyOTP(ctx context.Context, sessionID, code string) (*svcIfaces.VerifyOTPResult, error) {
	args := m.Called(ctx, sessionID, code)
	r, _ := args.Get(0).(*svcIfaces.VerifyOTPResult)
	return r, args.Error(1)
}

func (m *MockSupportService) ResendOTP(ctx context.Context, sessionID string) (*svcIfaces.LoginResult, error) {
	args := m.Called(ctx, sessionID)
	r, _ := args.Get(0).(*svcIfaces.LoginResult)
	return r, args.Error(1)
}

func (m *MockSupportService) RefreshToken(ctx context.Context, raw string) (*svcIfaces.RefreshResult, error) {
	args := m.Called(ctx, raw)
	r, _ := args.Get(0).(*svcIfaces.RefreshResult)
	return r, args.Error(1)
}

func (m *MockSupportService) Logout(ctx context.Context, raw string) error {
	return m.Called(ctx, raw).Error(0)
}

func (m *MockSupportService) Me(ctx context.Context, userID string) (*domain.SupportUser, error) {
	args := m.Called(ctx, userID)
	u, _ := args.Get(0).(*domain.SupportUser)
	return u, args.Error(1)
}

func (m *MockSupportService) ChangeMyPassword(ctx context.Context, userID, cur, newPwd string) error {
	return m.Called(ctx, userID, cur, newPwd).Error(0)
}

func (m *MockSupportService) ChangeMyEmail(ctx context.Context, userID, newEmail, cur string) error {
	return m.Called(ctx, userID, newEmail, cur).Error(0)
}

func (m *MockSupportService) CreateSupportAgent(ctx context.Context, email, fn, ln string) (string, error) {
	args := m.Called(ctx, email, fn, ln)
	return args.String(0), args.Error(1)
}

func (m *MockSupportService) ListSupportAgents(ctx context.Context, limit, offset int) ([]*domain.SupportUser, int, error) {
	args := m.Called(ctx, limit, offset)
	users, _ := args.Get(0).([]*domain.SupportUser)
	return users, args.Int(1), args.Error(2)
}

func (m *MockSupportService) DeactivateSupportAgent(ctx context.Context, userID string) error {
	return m.Called(ctx, userID).Error(0)
}
