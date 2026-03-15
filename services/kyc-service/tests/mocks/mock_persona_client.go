package mocks

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/kyc-service/internal/domain"
	"github.com/stretchr/testify/mock"
)

type MockPersonaClient struct {
	mock.Mock
}

func (m *MockPersonaClient) CreateInquiry(ctx context.Context, templateID string, referenceID string) (*domain.PersonaInquiry, error) {
	args := m.Called(ctx, templateID, referenceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PersonaInquiry), args.Error(1)
}

func (m *MockPersonaClient) ResumeInquiry(ctx context.Context, inquiryID string) (*domain.PersonaSession, error) {
	args := m.Called(ctx, inquiryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PersonaSession), args.Error(1)
}
